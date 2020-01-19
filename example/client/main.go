// Copyright 2020 Square, Inc.

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/square/rce-agent"
	"github.com/square/rce-agent/pb"
)

var (
	flagTLSCert    string
	flagTLSKey     string
	flagTLSCA      string
	flagServerAddr string
	flagTimeout    uint
)

func init() {
	flag.StringVar(&flagTLSCert, "tls-cert", "", "TLS certificate file")
	flag.StringVar(&flagTLSKey, "tls-key", "", "TLS key file")
	flag.StringVar(&flagTLSCA, "tls-ca", "", "TLS certificate authority")
	flag.StringVar(&flagServerAddr, "server-addr", "127.0.0.1:5501", "Server address:port")
	flag.UintVar(&flagTimeout, "timeout", 3000, "Dial timeout (milliseconds)")
}

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: client [options] command [args...]")
		fmt.Println("\"command\" is a command name from the server whitelist")
		os.Exit(1)
	}
	cmd := args[0]

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

	host, port, err := net.SplitHostPort(flagServerAddr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Connecting to %s:%s...", host, port)

	client := rce.NewClient(tlsConfig)
	if err := client.Open(host, port); err != nil {
		log.Fatalf("client.Open: %s", err)
	}
	defer client.Close()

	id, err := client.Start(cmd, args[1:])
	if err != nil {
		log.Fatal("client.Start: %s", err)
	}

	doneChan := make(chan struct{})
	var finalStatus *pb.Status
	var finalErr error
	go func() {
		finalStatus, finalErr = client.Wait(id)
		close(doneChan)
	}()

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		stdoutLine := 0
		stderrLine := 0
		for {
			select {
			case <-doneChan:
				return
			case <-ticker.C:
				status, err := client.GetStatus(id)
				if err != nil {
					log.Fatal("client.Wait: %s", err)
				}
				printOutput(status.Stdout, stdoutLine)
				stdoutLine = len(status.Stdout)

				printOutput(status.Stderr, stderrLine)
				stderrLine = len(status.Stderr)
			}
		}
	}()

	<-doneChan

	if finalErr != nil {
		log.Fatal("client.Wait: %s", err)
	}

	lnfmt := "%9s: %v\n"
	fmt.Printf(lnfmt, "ID", finalStatus.ID)
	fmt.Printf(lnfmt, "Name", finalStatus.Name)
	fmt.Printf(lnfmt, "State", finalStatus.State)
	fmt.Printf(lnfmt, "PID", finalStatus.PID)
	fmt.Printf(lnfmt, "StartTime", finalStatus.StartTime)
	fmt.Printf(lnfmt, "StopTime", finalStatus.StopTime)
	fmt.Printf(lnfmt, "ExitCode", finalStatus.ExitCode)
	fmt.Printf(lnfmt, "Error", finalStatus.Error)
	fmt.Printf(lnfmt, "Stdout", "")
	for _, line := range finalStatus.Stdout {
		fmt.Printf(lnfmt, "", line)
	}
	fmt.Printf(lnfmt, "Stderr", "")
	for _, line := range finalStatus.Stderr {
		fmt.Printf(lnfmt, "", line)
	}
}

func printOutput(lines []string, fromLine int) {
	if len(lines) < fromLine {
		return
	}
	for i := fromLine; i < len(lines); i++ {
		fmt.Println(lines[i])
	}
}
