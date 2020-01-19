// Copyright 2020 Square, Inc.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/square/rce-agent"
	"github.com/square/rce-agent/cmd"
)

var (
	flagTLSCert      string
	flagTLSKey       string
	flagTLSCA        string
	flagAddr         string
	flagCommandsFile string
)

func init() {
	flag.StringVar(&flagTLSCert, "tls-cert", "", "TLS certificate file")
	flag.StringVar(&flagTLSKey, "tls-key", "", "TLS key file")
	flag.StringVar(&flagTLSCA, "tls-ca", "", "TLS certificate authority")
	flag.StringVar(&flagAddr, "addr", "127.0.0.1:5501", "Address and port to listen on")
	flag.StringVar(&flagCommandsFile, "commands", "commands.yaml", "Commands whilelist file")
}

func main() {
	flag.Parse()

	// Load TLS if given
	tlsFiles := rce.TLSFiles{
		CACert: flagTLSCA,
		Cert:   flagTLSCert,
		Key:    flagTLSKey,
	}
	tlsConfig, err := tlsFiles.TLSConfig()
	if err != nil {
		log.Fatal(err)
	}
	if tlsConfig != nil {
		fmt.Println("TLS loaded")
	}

	commandsFile, err := filepath.Abs(flagCommandsFile)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	commands, err := cmd.LoadCommands(commandsFile)
	if err != nil {
		fmt.Printf("Error loading %s: %s\n", commandsFile, err)
		os.Exit(1)
	}

	srv := rce.NewServer(flagAddr, tlsConfig, commands)
	if err := srv.StartServer(); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
		os.Exit(1)
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	fmt.Println("CTRL-C to shut down")
	<-c
	fmt.Println("Shutting down...")
	if err := srv.StopServer(); err != nil {
		log.Printf("Error stopping server: %s\n", err)
	}
	os.Exit(0)
}
