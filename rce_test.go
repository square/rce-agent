// Copyright 2017 Square, Inc

package rce_test

import (
	"context"
	"testing"

	"github.com/go-test/deep"
	"github.com/square/rce-agent"
	"github.com/square/rce-agent/cmd"
	"github.com/square/rce-agent/pb"
)

const (
	HOST               = "127.0.0.1"
	PORT               = "5501"
	SERVER_TEST_CONFIG = "test/server-test-commands.yaml"
)

var whitelist, _ = cmd.LoadCommands(SERVER_TEST_CONFIG)
var LADDR = HOST + ":" + PORT

func TestExitZero(t *testing.T) {
	s := rce.NewServer(LADDR, nil, whitelist)

	c := &pb.Command{
		Name:      "exit.zero",
		Arguments: []string{},
	}

	id, err := s.Start(context.TODO(), c)
	if err != nil {
		t.Error(err)
	}

	// Wait for it to finish
	gotStatus, err := s.Wait(context.TODO(), id)
	if err != nil {
		t.Fatal(err)
	}
	if gotStatus == nil {
		t.Fatal("got nil pb.Status")
	}

	if gotStatus.StopTime == 0 {
		t.Errorf("got StopTime = %d, expected > 0", gotStatus.StopTime)
	}

	if gotStatus.ExitCode != 0 {
		t.Errorf("got ExitCode = %d, expected 0", gotStatus.ExitCode)
	}
}

func TestArgs(t *testing.T) {
	s := rce.NewServer(LADDR, nil, whitelist)

	message := "some.message"

	c := &pb.Command{
		Name:      "echo",
		Arguments: []string{message},
	}

	id, err := s.Start(context.TODO(), c)
	if err != nil {
		t.Error(err)
	}
	if id == nil {
		t.Fatal("got nil pb.ID")
	}
	if id.ID == "" {
		t.Fatal("got empty pd.ID.ID, expected a UUID")
	}

	// Wait for it to finish
	gotStatus, err := s.Wait(context.TODO(), id)
	if err != nil {
		t.Error(err)
	}
	if gotStatus == nil {
		t.Fatal("got nil pb.Status")
	}

	if gotStatus.StartTime <= 0 {
		t.Errorf("StartTime <= 0, expected > 0: %d", gotStatus.StartTime)
	}
	if gotStatus.StopTime <= 0 {
		t.Errorf("StopTime <= 0, expected > 0: %d", gotStatus.StopTime)
	}
	if gotStatus.StopTime <= gotStatus.StartTime {
		t.Errorf("StopTime %d <= StartTime %d, expected it to be greater",
			gotStatus.StopTime, gotStatus.StartTime)
	}
	gotStatus.StartTime = 0
	gotStatus.StopTime = 0

	if gotStatus.PID <= 0 {
		t.Errorf("PID <= 0, expected > 0: %d", gotStatus.PID)
	}
	gotStatus.PID = 0

	expectStatus := &pb.Status{
		ID:     id.ID,
		Name:   "echo",
		State:  pb.STATE_COMPLETE,
		Args:   []string{message},
		Stdout: []string{message},
		Stderr: []string{},
	}
	if diff := deep.Equal(gotStatus, expectStatus); diff != nil {
		t.Logf("%+v", gotStatus)
		t.Error(diff)
	}
}

func TestTLSServer(t *testing.T) {
	tlsFiles := rce.TLSFiles{
		RootCert:   "./test/tls/test_root_ca.crt",
		ClientCert: "./test/tls/test_server.crt",
		ClientKey:  "./test/tls/test_server.key",
	}
	tlsConfig, err := tlsFiles.TLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	s := rce.NewServer(LADDR, tlsConfig, whitelist)

	err = s.StartServer()
	if err != nil {
		t.Fatal(err)
	}
	defer s.StopServer()

	tlsFiles = rce.TLSFiles{
		RootCert:   "./test/tls/test_root_ca.crt",
		ClientCert: "./test/tls/test_client.crt",
		ClientKey:  "./test/tls/test_client.key",
	}
	tlsConfig, err = tlsFiles.TLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	c := rce.NewClient(tlsConfig)

	err = c.Open(HOST, PORT)
	if err != nil {
		t.Error(err)
	}

	id, err := c.Start("nonexistent-cmd", []string{})
	if err == nil {
		t.Error("got nil error, expected an error")
	}
	if id != "" {
		t.Errorf("got id '%s', expected empty string", id)
	}

	err = c.Close()
	if err != nil {
		t.Error(err)
	}
}
