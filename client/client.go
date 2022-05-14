package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log/syslog"
	"time"

	"github.com/fako1024/cmdchat"
	"github.com/fako1024/gotools/shell"
	"github.com/sirupsen/logrus"

	lSyslog "github.com/sirupsen/logrus/hooks/syslog"
)

func main() {

	// Create logger
	log := logrus.StandardLogger()

	// Fetch flags
	var (
		server string
		host   string

		secretFile string
		certFile   string
		keyFile    string
		caFile     string

		debug     bool
		useSyslog bool
	)
	flag.StringVar(&server, "server", "ws://127.0.0.1:5000", "Server to connect to")
	flag.StringVar(&host, "host", "", "Host to send commands to")

	flag.StringVar(&secretFile, "secret", "", "Path to key file used for E2E AEAD encryption / authentication")
	flag.StringVar(&certFile, "cert", "", "Path to certificate file used for client-server authentication")
	flag.StringVar(&keyFile, "key", "", "Path to key file used for client-server authentication")
	flag.StringVar(&caFile, "ca", "", "Path to CA certificate file used for client-server authentication")

	flag.BoolVar(&debug, "debug", false, "Debug mode (more verbose logging)")
	flag.BoolVar(&useSyslog, "syslog", false, "Emit logs to syslog")
	flag.Parse()

	syslogLevel := syslog.LOG_INFO | syslog.LOG_DAEMON
	if debug {
		log.Level = logrus.DebugLevel
		syslogLevel = syslog.LOG_DEBUG | syslog.LOG_DAEMON
	}
	if useSyslog {
		hook, err := lSyslog.NewSyslogHook("", "", syslogLevel, "cmdchat-client")

		if err == nil {
			log.Hooks.Add(hook)
		}
	}

	tlsConfig, err := cmdchat.PrepareClientCertificateAuth(certFile, keyFile, caFile)
	if err != nil {
		log.Fatal(err)
	}

	// Continuously attempt to (re-)connect
	if err := connectAndListen(server, host, secretFile, tlsConfig, log, 0); err != nil {
		log.Fatal(err)
	}

	// Continuously attempt to (re-)connect
	nConns := 1
	for {
		time.Sleep(time.Second)
		if err := connectAndListen(server, host, secretFile, tlsConfig, log, nConns); err != nil {
			log.Error(err)
		}
		nConns++
	}
}

func connectAndListen(server, host, keyPath string, tlsConfig *tls.Config, log *logrus.Logger, nConns int) error {

	uri := server + "/client/" + host + "/ws"

	// Instantiate a new Hub
	hub, err := cmdchat.New(uri, keyPath, tlsConfig, true)
	if err != nil {
		return fmt.Errorf("failed to establish WebSocket connection: %s", err)
	}
	defer func() {
		if err := hub.Close(); err != nil {
			log.Errorf("Failed to close hub: %s", err)
		}
	}()
	log.Infof("Connected client to websocket at %s", uri)

	// In case the connection was re-restablished, notify potential controllers
	if nConns > 0 {
		hub.WriteChan <- "Connection reset"
	}

	// Continuously receive commands
	for {
		msg, ok := <-hub.ReadChan
		if !ok {
			close(hub.WriteChan)
			return fmt.Errorf("cannot read command from cannel")
		}

		// Parse fields from command and run
		resp, err := shell.Run(msg)
		if err != nil {
			log.Errorf("Error executing shell command (%s): %s", err, resp)
			hub.WriteChan <- err.Error() + " " + resp
			continue
		}

		log.Debugf("Executed: %s - response: %s", msg, resp)
		hub.WriteChan <- resp
		log.Debugf("Sent response: %s", resp)
	}
}
