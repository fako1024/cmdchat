package cmdchat

import (
	"os"
	"strings"
	"time"

	"github.com/google/tink/go/aead"
	"github.com/google/tink/go/insecurecleartextkeyset"
	"github.com/google/tink/go/keyset"
	"github.com/google/tink/go/tink"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/valyala/gozstd"
)

// Hub denotes a connection hub / WebSocket interface
type Hub struct {
	ws  *websocket.Conn
	log *logrus.Logger

	aead tink.AEAD

	ReadChan  chan string
	WriteChan chan string
}

// New initializes a new hub
func New(uri, keyPath string, generateIfNotExists bool) (*Hub, error) {

	// Connect to server
	ws, _, err := websocket.DefaultDialer.Dial(uri, nil)
	if err != nil {
		return nil, err
	}

	// Initialize a new hub
	obj := &Hub{
		ws:        ws,
		log:       logrus.StandardLogger(),
		ReadChan:  make(chan string),
		WriteChan: make(chan string),
	}

	if err := obj.instantiateAEAD(keyPath, generateIfNotExists); err != nil {
		return nil, err
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
		_, encodedMessage, err := h.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.log.Errorf("Error reading from WebSocket: %s", err)
			}
			break
		}

		message, err := h.decodeMessage(encodedMessage)
		if err != nil {
			h.log.Errorf("Error decoding message: %s", err)
			continue
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

			encodedMessage, err := h.encodeMessage(message)
			if err != nil {
				log.Errorf("Error encoding message: %s", err)
				close(h.WriteChan)
				return
			}

			w, err := h.ws.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Errorf("Error obtaining WebSocket writer: %s", err)
				close(h.WriteChan)
				return
			}
			if _, err = w.Write(encodedMessage); err != nil {
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

// EncodeMessage encodes a message
func (h *Hub) encodeMessage(msg string) ([]byte, error) {

	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}

	var buf []byte
	buf = gozstd.Compress(buf[:0], []byte(strings.ToValidUTF8(msg, "")))

	ct, err := h.aead.Encrypt(buf, nil)
	if err != nil {
		return []byte{}, err
	}

	return ct, nil
}

// DecodeMessage decodes a message
func (h *Hub) decodeMessage(data []byte) (string, error) {

	pt, err := h.aead.Decrypt(data, nil)
	if err != nil {
		return "", err
	}

	var (
		buf []byte
	)
	buf, err = gozstd.Decompress(buf[:0], pt)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}

func (h *Hub) instantiateAEAD(keyPath string, generateIfNotExists bool) error {

	var kh *keyset.Handle

	// Attempt to open key file
	keyfile, err := os.OpenFile(keyPath, os.O_RDONLY, 0600)
	if err == nil {
		defer keyfile.Close()

		// Read AEAD key from file and instantiate handler
		kh, err = insecurecleartextkeyset.Read(keyset.NewBinaryReader(keyfile))
		if err != nil {
			return err
		}
	} else {

		// If it doesn't exist and generation was requested, create a new key file
		if os.IsNotExist(err) && generateIfNotExists {
			h.log.Infof("Key file %s does not exist, generating as requested ...", keyPath)

			if kh, err = h.generateKey(keyPath); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Instantiate new AEAD instance
	h.aead, err = aead.New(kh)
	if err != nil {
		return err
	}

	return nil
}

func (h *Hub) generateKey(keyPath string) (*keyset.Handle, error) {

	keyfile, err := os.OpenFile(keyPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer keyfile.Close()

	kh, err := keyset.NewHandle(DefaultAEADChipherTemplate())
	if err != nil {
		return nil, err
	}

	return kh, insecurecleartextkeyset.Write(kh, keyset.NewBinaryWriter(keyfile))
}
