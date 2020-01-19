// Copyright 2017-2020 Square, Inc.

package rce

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
)

// TLSFiles represents the TLS files necessary to create a tls.Config.
type TLSFiles struct {
	CACert string
	Cert   string
	Key    string
}

// TLSConfig returns a new tls.Config.
func (f TLSFiles) TLSConfig() (*tls.Config, error) {
	// If all files empty, then no TLS config
	if f.CACert == "" && f.Cert == "" && f.Key == "" {
		return nil, nil
	}

	// If any file is given, all must be given
	switch {
	case f.CACert == "":
		return nil, fmt.Errorf("CA certificate file not specified")
	case f.Cert == "":
		return nil, fmt.Errorf("Client certificate file not specified")
	case f.Key == "":
		return nil, fmt.Errorf("Client key file not specified")
	}

	// Load CA cert
	caCert, err := ioutil.ReadFile(f.CACert)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Load client cert+key
	cert, err := tls.LoadX509KeyPair(f.Cert, f.Key)
	if err != nil {
		return nil, fmt.Errorf("tls.LoadX509KeyPair %s %s: %s", f.Cert, f.Key, err)
	}

	// Build tls.Config suitable for both client and server side
	tlsConfig := &tls.Config{
		RootCAs:      caCertPool,                     // client uses to verify server
		ClientCAs:    caCertPool,                     // server uses to verify client
		ClientAuth:   tls.RequireAndVerifyClientCert, // server must verify client cert
		Certificates: []tls.Certificate{cert},        // client/server cert (given to other side of connection)
	}
	tlsConfig.BuildNameToCertificate() // maps CommonName and SubjectAlternateName to cert

	return tlsConfig, nil
}
