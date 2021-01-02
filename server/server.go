package main

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/sirupsen/logrus"
	"gopkg.in/olahol/melody.v1"
)

// defaultMaxMessageSize denotes the default maximum size allowed for transmission (32 MiB)
const defaultMaxMessageSize = 30 << 20

func main() {

	// Create logger
	log := logrus.StandardLogger()

	// Define echo + melody frameworks
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	m := melody.New()

	// Ensure a sufficient message size even for large command output
	m.Config.MaxMessageSize = defaultMaxMessageSize

	// Prepare session store
	sessions := make(map[string]string)

	// Define handler for clients
	e.GET("/client/:client/ws", func(c echo.Context) error {
		return m.HandleRequest(c.Response().Writer, c.Request())
	})

	// Define handler for controllers
	e.GET("/control/:controller/:client/ws", func(c echo.Context) error {

		// Store the current session / link between controller and client
		clientPath := "/client/" + c.Param("client") + "/ws"
		sessions[c.Request().URL.Path] = clientPath
		sessions[clientPath] = c.Request().URL.Path

		// Handle WebSocket connection
		err := m.HandleRequest(c.Response().Writer, c.Request())

		// Remove session information
		delete(sessions, c.Request().URL.Path)
		delete(sessions, clientPath)

		return err
	})

	// Define WebSockets handler
	m.HandleMessage(func(s *melody.Session, msg []byte) {

		// Filter / restrict messages from / to controller <-> client pairs
		if err := m.BroadcastBinaryFilter(msg, func(q *melody.Session) bool {

			// If the current session matches the stores pair, emit message
			if sessions[q.Request.URL.Path] == s.Request.URL.Path {
				log.Infof("Sending message with length %d from %s to %s", len(msg), s.Request.URL.Path, q.Request.URL.Path)
				log.Debugf("Sending `%s` from %s to %s", msg, s.Request.URL.Path, q.Request.URL.Path)

				return true
			}

			return false
		}); err != nil {
			log.Warnf("Error performing WebSocket broadcast filtering: %s", err)
		}
	})

	// Start server
	log.Infof("Starting server ...")
	e.Logger.Fatal(e.Start(":5000"))
}
