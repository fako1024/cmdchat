package cmdchat

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// Hub denotes a connection hub / WebSocket interface
type Hub struct {
	ws  *websocket.Conn
	log *logrus.Logger

	ReadChan  chan []byte
	WriteChan chan []byte
}

// New initializes a new hub
func New(uri string) (*Hub, error) {

	// Connect to server
	ws, _, err := websocket.DefaultDialer.Dial(uri, nil)
	if err != nil {
		return nil, err
	}

	// Initialize a new hub
	obj := &Hub{
		ws:        ws,
		log:       logrus.StandardLogger(),
		ReadChan:  make(chan []byte),
		WriteChan: make(chan []byte),
	}

	// Start listening / channel handling
	go obj.Read()
	go obj.Write()

	return obj, nil
}

// Close closes a hub
func (h *Hub) Close() error {
	return h.ws.Close()
}

// Read performs read operations on the WebSocket connection
func (h *Hub) Read() {

	defer func() {
		close(h.ReadChan)
		h.log.Debugf("Stopped waiting for messages to read from WebSocket ...")
	}()

	h.ws.SetReadLimit(DefaultMaxMessageSize)
	if err := h.ws.SetReadDeadline(time.Now().Add(DefaultKeepAliveDeadline)); err != nil {
		h.log.Errorf("Error setting read deadline on WebSocket: %s", err)
		return
	}
	h.ws.SetPongHandler(func(string) error {
		return h.ws.SetReadDeadline(time.Now().Add(DefaultKeepAliveDeadline))
	})

	h.log.Debugf("Waiting for messages to read from WebSocket ...")

	for {
		_, message, err := h.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.log.Errorf("Error reading from WebSocket: %s", err)
			}
			break
		}

		h.ReadChan <- message
	}
}

// Write performs write operations on the WebSocket connection
func (h *Hub) Write() {

	log := logrus.StandardLogger()

	ticker := time.NewTicker(DefaultKeepAliveInterval)
	defer func() {
		ticker.Stop()
		log.Debugf("Stopped waiting for messages to write to WebSocket ...")
	}()

	log.Debugf("Waiting for messages to write to WebSocket ...")

	for {
		select {
		case message, ok := <-h.WriteChan:

			if err := h.ws.SetWriteDeadline(time.Now().Add(DefaultWriteTimeout)); err != nil {
				log.Errorf("Error setting write deadline on WebSocket: %s", err)
				close(h.WriteChan)
				return
			}
			if !ok {

				// Channel was closed, terminate writer
				if err := h.ws.WriteMessage(websocket.CloseMessage, nil); err != nil {
					log.Errorf("Error writing close message to WebSocket: %s", err)
				}
				return
			}

			w, err := h.ws.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Errorf("Error obtaining WebSocket writer: %s", err)
				close(h.WriteChan)
				return
			}
			if _, err = w.Write(message); err != nil {
				log.Errorf("Error writing to WebSocket: %s", err)
				close(h.WriteChan)
				return
			}
			if err := w.Close(); err != nil {
				log.Errorf("Error closing WebSocket writer: %s", err)
				close(h.WriteChan)
				return
			}

		case <-ticker.C:
			if err := h.ws.SetWriteDeadline(time.Now().Add(DefaultWriteTimeout)); err != nil {
				log.Errorf("Error setting write deadline on WebSocket: %s", err)
				close(h.WriteChan)
				return
			}
			if err := h.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Errorf("Error writing keepalive message to WebSocket: %s", err)
				close(h.WriteChan)
				return
			}
		}
	}
}
