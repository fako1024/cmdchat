package cmdchat

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"

	"golang.org/x/crypto/ssh/terminal"
)

// PrepareClientCertificateAuth reads the provided client certificate / key and CA certificate
// files (and potentially inteactively unlocks an encrypted key files)
func PrepareClientCertificateAuth(certFile, keyFile, caFile string) (*tls.Config, error) {

	// Read the client certificate file
	clientCert, err := ioutil.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to read client certificate file: %s", err)
	}

	// Read the client key file
	clientKey, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to read client key file: %s", err)
	}

	// Attempt to decode the key block
	pemBlock, _ := pem.Decode(clientKey)
	if pemBlock == nil {
		return nil, fmt.Errorf("Failed to decode key file: %s", err)
	}

	// If the key is encrypted, ask for password and decrypt it
	if x509.IsEncryptedPEMBlock(pemBlock) {

		// Prompt for client certificate password
		password, err := requestPassword("Enter password for encrypted client key (will not be echoed): ")
		if err != nil {
			return nil, fmt.Errorf("Failed to acquire client key password: %s", err)
		}

		// Decrypt / parse / marshal / encode the key
		clientKey, err = extractPrivateKey(pemBlock, password)
		if err != nil {
			return nil, fmt.Errorf("Failed to extract private key: %s", err)
		}
	}

	// Load the key pair
	clientKeyCert, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		return nil, fmt.Errorf("Failed to load client key / certificate: %s", err)
	}

	// Read CA certificate from file and instantiate CA certificate pool
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to read CA certificate: %s", err)
	}
	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("Failed to obtain system CA pool: %s", err)
	}
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("Failed to add CA certificate to pool: %s", err)
	}

	tlsConfig := &tls.Config{
		CurvePreferences: []tls.CurveID{tls.CurveP521},
		Certificates:     []tls.Certificate{clientKeyCert},
		RootCAs:          caCertPool,
	}
	tlsConfig.BuildNameToCertificate()

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
	fmt.Printf(prompt)

	password, err := terminal.ReadPassword(0)
	fmt.Println()

	return password, err
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
