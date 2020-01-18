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
		return nil, fmt.Errorf("tls.LoadX509KeyPair %s %s: %s", f.ClientCert, f.ClientKey, err)
	}

	caCert, err := ioutil.ReadFile(f.RootCert)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		RootCAs:      caCertPool,
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{cert},
	}
	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}
