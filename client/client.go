package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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
		server string
		host   string

		secretFile string
		certFile   string
		keyFile    string
		caFile     string

		debug bool
	)
	flag.StringVar(&server, "server", "ws://127.0.0.1:5000", "Server to connect to")
	flag.StringVar(&host, "host", "", "Host to send commands to")

	flag.StringVar(&secretFile, "secret", "", "Path to key file used for E2E AEAD encryption / authentication")
	flag.StringVar(&certFile, "cert", "", "Path to certificate file used for client-server authentication")
	flag.StringVar(&keyFile, "key", "", "Path to key file used for client-server authentication")
	flag.StringVar(&caFile, "ca", "", "Path to CA certificate file used for client-server authentication")

	flag.BoolVar(&debug, "debug", false, "Debug mode (more verbose logging)")
	flag.Parse()

	if debug {
		log.Level = logrus.DebugLevel
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
		resp, err := runShellCmd(msg)
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

func runShellCmd(command string) (string, error) {

	if command == "" {
		return "", nil
	}

	var (
		outStringBuf bytes.Buffer
		outBuf       io.Writer = &outStringBuf
	)

	// Check if the command requests a redirect of STDOUT / STDERR to file
	command, outFilePath, err := splitByRedirect(command)
	if err != nil {
		return "", err
	}
	if outFilePath != "" {
		outFile, err := os.OpenFile(outFilePath, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return "", err
		}
		defer outFile.Close()
		outBuf = bufio.NewWriter(outFile)
	}

	// Parse command line into command + arguments
	fields, err := shlex.Split(command)
	if err != nil || len(fields) == 0 {
		return "", fmt.Errorf("failed to parse command (%s): %s", command, err)
	}

	// Execute command
	err = generateCommand(fields, outBuf).Run()

	return outStringBuf.String(), err
}

func splitByRedirect(command string) (string, string, error) {

	split := strings.Split(command, ">")

	if len(split) == 1 {
		return command, "", nil
	}

	if len(split) == 2 {

		outFields, err := shlex.Split(split[1])
		if err != nil {
			return "", "", fmt.Errorf("failed to parse output file path: %s", err)
		}
		if len(outFields) != 1 {
			return "", "", fmt.Errorf("invalid syntax: %s", command)
		}

		return split[0], outFields[0], nil
	}

	return "", "", fmt.Errorf("invalid syntax: %s", command)
}

func generateCommand(fields []string, outBuf io.Writer) (cmd *exec.Cmd) {

	// Check if any arguments were provided
	/* #nosec G204 */
	if len(fields) == 1 {
		cmd = exec.Command(fields[0])
	} else {
		cmd = exec.Command(fields[0], fields[1:]...)
	}

	// Attach STDOUT + STDERR to output buffer
	cmd.Stdout = outBuf
	cmd.Stderr = outBuf

	return
}
