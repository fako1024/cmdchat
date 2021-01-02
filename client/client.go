package main

import (
	"bytes"
	"flag"
	"fmt"
	"os/exec"
	"time"

	"github.com/fako1024/cmdchat"
	"github.com/google/shlex"
	"github.com/sirupsen/logrus"
)

func main() {

	// Create logger
	log := logrus.StandardLogger()

	// Fetch flags
	var (
		server  string
		host    string
		keyPath string
		debug   bool
	)
	flag.StringVar(&server, "server", "127.0.0.1:5000", "Server to connect to")
	flag.StringVar(&host, "host", "", "Host to send commands to")
	flag.StringVar(&keyPath, "key", "", "Path to key file used for AEAD encryption / authentication")
	flag.BoolVar(&debug, "debug", false, "Debug mode (more verbose logging)")
	flag.Parse()

	if debug {
		log.Level = logrus.DebugLevel
	}

	// Continuously attempt to (re-)connect
	if err := connectAndListen(server, host, keyPath, log, 0); err != nil {
		log.Fatal(err)
	}

	// Continuously attempt to (re-)connect
	nConns := 1
	for {
		time.Sleep(time.Second)
		if err := connectAndListen(server, host, keyPath, log, nConns); err != nil {
			log.Error(err)
		}
		nConns++
	}
}

func connectAndListen(server, host, keyPath string, log *logrus.Logger, nConns int) error {

	uri := "ws://" + server + "/client/" + host + "/ws"

	// Instantiate a new Hub
	hub, err := cmdchat.New(uri, keyPath, true)
	if err != nil {
		return fmt.Errorf("failed to establish WebSocket connection: %s", err)
	}
	defer hub.Close()
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
		resp, err := runShellCmd(msg)
		if err != nil {
			log.Errorf("error executing shell command (%s): %s", err, resp)
			hub.WriteChan <- err.Error() + " " + resp
			continue
		}

		log.Debugf("Executed: %s - response: %s", msg, resp)
		hub.WriteChan <- resp
		log.Debugf("Sent response: %s", resp)
	}
}

func runShellCmd(command string) (string, error) {

	if command == "" {
		return "", nil
	}

	// Parse command line into command + arguments
	fields, err := shlex.Split(command)
	if err != nil || len(fields) == 0 {
		return "", fmt.Errorf("failed to parse command (%s): %s", command, err)
	}

	// TODO: Need some magic here to interpret ">" in order to pipe output to a file on the client
	// See e.g. https://stackoverflow.com/questions/40090351/echo-command-in-golang
	var cmd *exec.Cmd
	if len(fields) == 1 {
		cmd = exec.Command(fields[0])
	} else {
		cmd = exec.Command(fields[0], fields[1:]...)
	}

	// Attacg STDOUT + STDERR to output buffer
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	// Execute command
	err = cmd.Run()

	return outBuf.String(), err
}
