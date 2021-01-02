package main

import (
	"bytes"
	"flag"
	"fmt"
	"os/exec"
	"time"

	"github.com/fako1024/cmdchat"
	"github.com/google/shlex"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

func main() {

	// Create logger
	log := logrus.StandardLogger()

	// Fetch flags
	var (
		server string
		host   string
		debug  bool
	)
	flag.StringVar(&server, "server", "127.0.0.1:5000", "Server to connect to")
	flag.StringVar(&host, "host", "", "Host to send commands to")
	flag.BoolVar(&debug, "debug", false, "Debug mode (more verbose logging)")
	flag.Parse()

	if debug {
		log.Level = logrus.DebugLevel
	}

	// Continuously attempt to (re-)connect
	nConns := 0
	for {
		if err := connectAndListen(server, host, log, nConns); err != nil {
			log.Error(err)
			time.Sleep(time.Second)
		}
		nConns++
	}
}

func connectAndListen(server, host string, log *logrus.Logger, nConns int) error {

	uri := "ws://" + server + "/client/" + host + "/ws"

	// Connect to server
	ws, _, err := websocket.DefaultDialer.Dial(uri, nil)
	if err != nil {
		return fmt.Errorf("failed to establish WebSocket connection: %s", err)
	}
	defer ws.Close()
	log.Infof("Connected client to websocket at %s", uri)

	// Initialize channels for reading / writing
	readChan, writeChan := cmdchat.InitReadWriteChannels(ws)

	// In case the connection was re-restablished, notify potential controllers
	if nConns > 0 {
		writeChan <- cmdchat.SanitizeMessage("Connection reset")
	}

	// Continuously receive commands
	for {
		msg, ok := <-readChan
		if !ok {
			close(writeChan)
			return fmt.Errorf("cannot read command from cannel")
		}

		// Parse fields from command and run
		resp, err := runShellCmd(string(msg))
		if err != nil {
			log.Errorf("error executing shell command (%s): %s", err, resp)
			writeChan <- cmdchat.SanitizeMessage(err.Error() + " " + resp)
			continue
		}

		log.Debugf("Executed: %s - response: %s", msg, resp)
		writeChan <- cmdchat.SanitizeMessage(resp)
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
