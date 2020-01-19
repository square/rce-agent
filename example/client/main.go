// Copyright 2020 Square, Inc.

/*
	This is an example RCE client. It uses an rce.Client (which uses a gRPC client)
	to run whitelist commands on an RCE agent (server). Your client will call
	the same rce.Client methods but otherwise be different than this example.
	The differences depend on what your client is used for. For example,
	at Square we have back end systems that use an rce.Client to run commands
	on internal hosts. So the client is not a human-driven CLI like this example
	but a "headless" client used inside a larger back end system codebase.

	This example code demonstrates two things. First, the basic creation and
	usage of an rce.Client (with TLS). Second, how to stream output from the
	remote command.
*/

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
	// ----------------------------------------------------------------------
	// Parse command line flags (options)
	// ----------------------------------------------------------------------
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: client [options] command [args...]")
		fmt.Println("\"command\" is a command name from the server whitelist")
		os.Exit(1)
	}
	cmd := args[0] // remote whitelist command

	// ----------------------------------------------------------------------
	// Load TLS if given
	// ----------------------------------------------------------------------
	// You should use rce.TLSFiles like used here because it creates a
	// tls.Config that requires mutual authentication: client verifies agent
	// TLS cert _and_ agent verifies client TLS cert. You can create your
	// own tls.Config if you don't need mutual auth.
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
		log.Println("TLS loaded")
	}

	// ----------------------------------------------------------------------
	// Create and connect rce.Client
	// ----------------------------------------------------------------------
	client := rce.NewClient(tlsConfig)

	// Split "server:port". Don't use strings pkg, use net.SplitHostPort
	host, port, err := net.SplitHostPort(flagServerAddr)
	if err != nil {
		log.Fatal(err)
	}

	// Connect to agent (server)
	log.Printf("Connecting to %s:%s...", host, port)
	if err := client.Open(host, port); err != nil {
		log.Fatalf("client.Open: %s", err)
	}
	defer client.Close() // *** Remember to close the client connection! ***
	log.Printf("Connected")

	// ----------------------------------------------------------------------
	// Start remote command
	// ----------------------------------------------------------------------
	// Commands are asynchronous, so as the godocs say, "This call is non-blocking."
	// The command ID is a UUID that identifies this run of the command,
	// similar to a PID but globally unique across all clients and agents.
	// If you store/audit remote commands, be sure to store the ID. The server log
	// prints this ID, too, so command execution can be easily traced on both sides.
	id, err := client.Start(cmd, args[1:])
	if err != nil {
		log.Fatalf("client.Start: %s", err)
	}

	// ----------------------------------------------------------------------
	// Wait for remote command
	// ----------------------------------------------------------------------
	// In the simplest case, we could call client.Wait(id) (below) and block
	// until the command finishes. But for this example we do something more
	// realistic: we presume the command might take a little while, so we call
	// client.GetStatus(id) every 2 seconds. If the command takes <2s, then
	// this loop does nothing. But if the command takes >2s, then the loop
	// streams the STDOUT and STDERR of the command.

	// First, wait for command to finish in a goroutine so we we don't block.
	// When the command finishes, close doneChan to signal the other goroutine
	// (below) to stop, too, and unblock this main func below.
	doneChan := make(chan struct{})
	var finalStatus *pb.Status
	var finalErr error
	go func() {
		finalStatus, finalErr = client.Wait(id) // block waiting for command to finish
		close(doneChan)                         // stop goroutine below and unblock main
	}()

	// Second, every 2s get the command status which includes its STDOUT and
	// STDERR. The outputs are cumulative, so we have to track the line number
	// for each and print the tail. So if the final output would be,
	//   1
	//   2
	//   3
	// Then Stdout will be like []{"1"}, []{"1", "2"}, []{"1", "2", "3"}, if
	// we check it three times.
	//
	// This goroutine stops once the Wait goroutine above stops. They're
	// synchronized on doneChan.
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
					log.Printf("client.GetStatus: %s", err)
				}
				printOutput(status.Stdout, stdoutLine)
				stdoutLine = len(status.Stdout)
				printOutput(status.Stderr, stderrLine)
				stderrLine = len(status.Stderr)
			}
		}
	}()

	// Wait for the first goroutine above to signal that the command has finished
	<-doneChan

	// ----------------------------------------------------------------------
	// Command done
	// ----------------------------------------------------------------------
	if finalErr != nil {
		log.Fatalf("client.Wait: %s", err)
	}

	// See https://godoc.org/github.com/square/rce-agent/pb#Status
	// For this example, we just pretty-print the whole struct.
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
