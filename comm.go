package cmdchat

import (
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// InitReadWriteChannels creates and initialized read / write channels for
// communication via the WebSocket connection
func InitReadWriteChannels(ws *websocket.Conn) (chan []byte, chan []byte) {

	readChan, writeChan := make(chan []byte), make(chan []byte)

	go Read(ws, readChan)
	go Write(ws, writeChan)

	return readChan, writeChan
}

// Read performs read operations on the WebSocket connection
func Read(ws *websocket.Conn, c chan []byte) {

	log := logrus.StandardLogger()

	defer func() {
		close(c)
		log.Debugf("Stopped waiting for messages to read from WebSocket ...")
	}()

	ws.SetReadLimit(DefaultMaxMessageSize)
	if err := ws.SetReadDeadline(time.Now().Add(DefaultKeepAliveDeadline)); err != nil {
		log.Errorf("Error setting read deadline on WebSocket: %s", err)
		return
	}
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(DefaultKeepAliveDeadline))
	})

	log.Debugf("Waiting for messages to read from WebSocket ...")

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Errorf("Error reading from WebSocket: %s", err)
			}
			break
		}

		c <- message
	}
}

// Write performs write operations on the WebSocket connection
func Write(ws *websocket.Conn, c chan []byte) {

	log := logrus.StandardLogger()

	ticker := time.NewTicker(DefaultKeepAliveInterval)
	defer func() {
		ticker.Stop()
		log.Debugf("Stopped waiting for messages to write to WebSocket ...")
	}()

	log.Debugf("Waiting for messages to write to WebSocket ...")

	for {
		select {
		case message, ok := <-c:

			if err := ws.SetWriteDeadline(time.Now().Add(DefaultWriteTimeout)); err != nil {
				log.Errorf("Error setting write deadline on WebSocket: %s", err)
				close(c)
				return
			}
			if !ok {

				// Channel was closed, terminate writer
				if err := ws.WriteMessage(websocket.CloseMessage, nil); err != nil {
					log.Errorf("Error writing close message to WebSocket: %s", err)
				}
				return
			}

			w, err := ws.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Errorf("Error obtaining WebSocket writer: %s", err)
				close(c)
				return
			}
			if _, err = w.Write(message); err != nil {
				log.Errorf("Error writing to WebSocket: %s", err)
				close(c)
				return
			}
			if err := w.Close(); err != nil {
				log.Errorf("Error closing WebSocket writer: %s", err)
				close(c)
				return
			}

		case <-ticker.C:
			if err := ws.SetWriteDeadline(time.Now().Add(DefaultWriteTimeout)); err != nil {
				log.Errorf("Error setting write deadline on WebSocket: %s", err)
				close(c)
				return
			}
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Errorf("Error writing keepalive message to WebSocket: %s", err)
				close(c)
				return
			}
		}
	}
}

// SanitizeMessage ensure UTF-8 compatibility and terminates output with a
// newline
func SanitizeMessage(msg string) []byte {

	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}

	return []byte(strings.ToValidUTF8(msg, ""))
}
