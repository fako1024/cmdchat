package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fako1024/cmdchat"
	uuid "github.com/satori/go.uuid"
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

	id := uuid.NewV4()
	uri := "ws://" + server + "/control/" + id.String() + "/" + host + "/ws"

	// Instantiate a new Hub
	hub, err := cmdchat.New(uri, keyPath)
	if err != nil {
		log.Fatalf("Failed to establish WebSocket connection: %s", err)
	}
	defer hub.Close()
	log.Infof("Connected controller to websocket at %s", uri)

	// Continuously read commands from STDIN
	reader := bufio.NewReader(os.Stdin)
	for {

		// Prompt for and parse user input
		text, exit, err := prompt(reader)
		if err != nil {
			log.Errorf("Failed to read command line: %s", err)
			continue
		}
		if exit {
			return
		}

		// Send the command to the client
		hub.WriteChan <- text
		log.Debugf("Sent command: %s", text)

		// Retrieve and print the response
		resp, ok := <-hub.ReadChan
		if !ok {
			log.Fatalf("Failed to read command response from channel")
		}

		fmt.Printf("%s", resp)
	}
}

func prompt(reader *bufio.Reader) (string, bool, error) {

	// Prompt for input
	fmt.Print("# ")
	text, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return "", true, nil
		}
		return "", false, err
	}

	// convert CRLF to LF
	text = strings.Replace(text, "\n", "", -1)
	if text == "exit" {
		return "", true, nil
	}

	return text, false, nil
}
