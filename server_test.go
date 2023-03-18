// Copyright 2017-2023 Block, Inc.

package rce_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/go-test/deep"
	"github.com/square/rce-agent"
	"github.com/square/rce-agent/cmd"
	"github.com/square/rce-agent/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	HOST               = "127.0.0.1"
	PORT               = "5501"
	SERVER_TEST_CONFIG = "test/server-test-commands.yaml"
)

var whitelist, _ = cmd.LoadCommands(SERVER_TEST_CONFIG)
var LADDR = HOST + ":" + PORT

func TestServerExitZero(t *testing.T) {
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

	// There's no status after the command has exited; it doesn't exist
	gotStatus2, err := s.GetStatus(context.TODO(), id)
	se, ok := status.FromError(err)
	if ok {
		if se.Code() != codes.NotFound {
			t.Errorf("gRPC status error not codes.NotFound: %v", se.Code())
		}
	} else {
		t.Errorf("error from Status after command exited not a gRPC status error: %#v", err)
	}
	if gotStatus2 != nil {
		t.Errorf("got status after command exited, expected nil: %+v", gotStatus2)
	}

	// And Stop should be idempotent, also returning a "not found" gRPC error
	_, err = s.Stop(context.TODO(), id)
	se, ok = status.FromError(err)
	if ok {
		if se.Code() != codes.NotFound {
			t.Errorf("gRPC status error not codes.NotFound: %v", se.Code())
		}
	} else {
		t.Errorf("error from Stop after command exited not a gRPC status error: %#v", err)
	}
}

func TestServerArgs(t *testing.T) {
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

func TestServerTLS(t *testing.T) {
	tlsFiles := rce.TLSFiles{
		CACert: "./test/tls/test_root_ca.crt",
		Cert:   "./test/tls/test_server.crt",
		Key:    "./test/tls/test_server.key",
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
		CACert: "./test/tls/test_root_ca.crt",
		Cert:   "./test/tls/test_client.crt",
		Key:    "./test/tls/test_client.key",
	}
	tlsConfig, err = tlsFiles.TLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	c := rce.NewClient(tlsConfig)

	err = c.Open(HOST, PORT)
	if err != nil {
		t.Fatal(err)
	}

	id, gotErr := c.Start("nonexistent-cmd", []string{})
	expectErr := grpc.Errorf(codes.InvalidArgument, "unknown command: nonexistent-cmd")
	if diff := deep.Equal(gotErr, expectErr); diff != nil {
		t.Error(diff)
	}
	if id != "" {
		t.Errorf("got id '%s', expected empty string", id)
	}

	err = c.Close()
	if err != nil {
		t.Error(err)
	}
}

func TestServerWithConfig(t *testing.T) {
	tlsFiles := rce.TLSFiles{
		CACert: "./test/tls/test_root_ca.crt",
		Cert:   "./test/tls/test_client.crt",
		Key:    "./test/tls/test_client.key",
	}
	tlsConfig, err := tlsFiles.TLSConfig()
	if err != nil {
		t.Fatal(err)
	}

	if err != nil {
		t.Fatal(err)
	}
	cfg := rce.ServerConfig{
		Addr:            LADDR,
		AllowedCommands: whitelist,
		TLS:             tlsConfig,
	}
	s := rce.NewServerWithConfig(cfg)

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

func TestServerConfigValidation(t *testing.T) {
	// AllowAnyCommand requires that AllowedCommands is nil
	cfg := rce.ServerConfig{
		Addr:            LADDR,
		AllowedCommands: whitelist,
		AllowAnyCommand: true,
	}
	s := rce.NewServerWithConfig(cfg)
	err := s.StartServer()
	if err != rce.ErrInvalidServerConfigAllowAnyCommand {
		t.Errorf("Start returned error '%v', expected '%v' (ErrInvalidServerConfigAllowAnyCommand)", err, rce.ErrInvalidServerConfigAllowAnyCommand)
	}

	// AllowAnyCommand without TLS requires DisableSecurity
	cfg = rce.ServerConfig{
		Addr:            LADDR,
		AllowAnyCommand: true,
	}
	s = rce.NewServerWithConfig(cfg)
	err = s.StartServer()
	if err != rce.ErrInvalidServerConfigDisableSecurity {
		t.Errorf("Start returned error '%v', expected '%v' (ErrInvalidServerConfigDisableSecurity)", err, rce.ErrInvalidServerConfigDisableSecurity)
	}
}

func TestServerAnyCommand(t *testing.T) {
	tlsFiles := rce.TLSFiles{
		CACert: "./test/tls/test_root_ca.crt",
		Cert:   "./test/tls/test_client.crt",
		Key:    "./test/tls/test_client.key",
	}
	tlsConfig, err := tlsFiles.TLSConfig()
	if err != nil {
		t.Fatal(err)
	}

	if err != nil {
		t.Fatal(err)
	}
	cfg := rce.ServerConfig{
		Addr:            LADDR,
		AllowAnyCommand: true,
		TLS:             tlsConfig, // required else we need DisableSecurity = true
	}
	s := rce.NewServerWithConfig(cfg)

	// Run "go version" to check that any command is allowed and it works with
	// arguments. We'll run it for real to get the output, then run it via the
	// RCE server and compare outputs to make sure it's correct.
	out, err := exec.Command("which", "go").Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) == "" {
		t.Fatal("'which go' did not return go binary path")
	}
	gobin := strings.TrimSpace(string(out))
	t.Logf("go bin: %s", gobin)

	out, err = exec.Command(string(gobin), "version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) == "" {
		t.Fatalf("'%s version' did not return anything", string(gobin))
	}
	gover := strings.TrimSpace(string(out))
	t.Logf("go ver: %s", gover)

	c := &pb.Command{
		Name:      string(gobin),
		Arguments: []string{"version"},
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

	if len(gotStatus.Stdout) != 1 {
		t.Errorf("got %d lines of stdout, expected 1: %v", len(gotStatus.Stdout), gotStatus.Stdout)
	} else if gotStatus.Stdout[0] != gover {
		t.Errorf("stdout = '%s', expected '%s'", gotStatus.Stdout[0], string(gover))
	}
}
