package rce

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
)

// TLSFiles represents the TLS files necessary to create a tls.Config.
type TLSFiles struct {
	RootCert   string
	ClientCert string
	ClientKey  string
}

// TLSConfig returns a new tls.Config.
func (f TLSFiles) TLSConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(f.ClientCert, f.ClientKey)
	if err != nil {
		return nil, fmt.Errorf("tls.LoadX509KeyPair: %s", err)
	}

	caCert, err := ioutil.ReadFile(f.RootCert)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}
