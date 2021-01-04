package main

import (
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/ssh/terminal"
)

func prepareAuthHeader(user string) (string, error) {

	// Skip asking for password if no user was provided
	if user == "" {
		return "", nil
	}

	// Prompt for username
	fmt.Printf("Enter password for %s (will not be echoed): ", user)

	password, err := terminal.ReadPassword(0)
	if err != nil {
		return "", err
	}

	return generateBasicAuth(user, string(password)), nil
}

func generateBasicAuth(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}
