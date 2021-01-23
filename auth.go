package cmdchat

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"golang.org/x/crypto/ssh/terminal"
)

// PrepareClientCertificateAuth reads the provided client certificate / key and CA certificate
// files (and potentially inteactively unlocks an encrypted key files)
func PrepareClientCertificateAuth(certFile, keyFile, caFile string) (*tls.Config, error) {

	// Read the client certificate / key file
	clientCert, clientKey, err := readclientKeyCertificate(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read / decode client key / certificate file: %s", err)
	}

	// Load the key pair
	clientKeyCert, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load client key / certificate: %s", err)
	}

	// Read CA certificate from file and instantiate CA certificate pool
	caCert, err := ioutil.ReadFile(filepath.Clean(caFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %s", err)
	}
	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain system CA pool: %s", err)
	}
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add CA certificate to pool: %s", err)
	}

	tlsConfig := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384},
		PreferServerCipherSuites: true,
		Certificates:             []tls.Certificate{clientKeyCert},
		RootCAs:                  caCertPool,
	}

	return tlsConfig, nil
}

// PrepareBasicAuthHeader asks for a user password and creates a ready-to use Basic Auth
// Authorization header content / value
func PrepareBasicAuthHeader(user string) (string, error) {

	// Skip asking for password if no user was provided
	if user == "" {
		return "", nil
	}

	// Prompt for password
	password, err := requestPassword(fmt.Sprintf("Enter password for %s (will not be echoed): ", user))
	if err != nil {
		return "", err
	}

	return generateBasicAuth(user, string(password)), nil
}

func generateBasicAuth(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

func requestPassword(prompt string) ([]byte, error) {

	// Prompt for password
	fmt.Print(prompt)

	password, err := terminal.ReadPassword(0)
	fmt.Println()

	return password, err
}

func readclientKeyCertificate(certFile, keyFile string) ([]byte, []byte, error) {

	// Read the client certificate file
	clientCert, err := ioutil.ReadFile(filepath.Clean(certFile))
	if err != nil {
		return nil, nil, err
	}

	// Read the client key file
	clientKey, err := ioutil.ReadFile(filepath.Clean(keyFile))
	if err != nil {
		return nil, nil, err
	}

	// Attempt to decode the key block
	pemBlock, _ := pem.Decode(clientKey)
	if pemBlock == nil {
		return nil, nil, err
	}

	// If the key is encrypted, ask for password and decrypt it
	if x509.IsEncryptedPEMBlock(pemBlock) {

		// Prompt for client certificate password
		password, err := requestPassword("Enter password for encrypted client key (will not be echoed): ")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to acquire client key password: %s", err)
		}

		// Decrypt / parse / marshal / encode the key
		clientKey, err = extractPrivateKey(pemBlock, password)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to extract private key: %s", err)
		}
	}

	return clientCert, clientKey, nil
}

func extractPrivateKey(block *pem.Block, password []byte) ([]byte, error) {

	// Decrypt / parse / marshal / encode the key
	plainPEMBlock, err := x509.DecryptPEMBlock(block, password)
	if err != nil {
		return nil, err
	}
	pkey, err := x509.ParseECPrivateKey(plainPEMBlock)
	if err != nil {
		return nil, err
	}
	marshaledKey, err := x509.MarshalECPrivateKey(pkey)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(
		&pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: marshaledKey,
		},
	), nil
}
