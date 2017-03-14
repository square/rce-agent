// Copyright 2017 Square, Inc

package rce_test

import (
	"context"
	"testing"

	"github.com/go-test/deep"
	"github.com/square/rce-agent"
	"github.com/square/rce-agent/pb"
)

const PORT = "5501"

const SERVER_TEST_CONFIG = "test/server-test-commands.yaml"

func TestExitZero(t *testing.T) {
	s, err := rce.NewServer(PORT, SERVER_TEST_CONFIG)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("server: %+v\n", s)

	jr := &pb.JobRequest{
		CommandName: "exit.zero",
		Arguments:   []string{},
	}
	t.Logf("request: %+v\n", jr)

	status, err := s.StartJob(context.TODO(), jr)
	if err != nil {
		t.Error(err)
	}
	t.Logf("initial status: %+v\n", status)

	jobID := &pb.JobID{JobID: status.JobID}

	// Wait for it to finish
	s.WaitOnJob(context.TODO(), jobID)

	status, err = s.GetJobStatus(context.TODO(), jobID)
	t.Logf("status: %+v\nerr: %+v", status, err)

	if status.FinishTime == 0 {
		t.Errorf("got FinishTime = %d, expected > 0", status.FinishTime)
	}

	if status.ExitCode != 0 {
		t.Errorf("got ExitCode = %d, expected 0", status.ExitCode)
	}
}

func TestArgs(t *testing.T) {
	s, err := rce.NewServer(PORT, SERVER_TEST_CONFIG)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("server: %+v\n", s)

	message := "some.message"

	jr := &pb.JobRequest{
		CommandName: "echo",
		Arguments:   []string{message},
	}
	t.Logf("request: %+v\n", jr)

	status, err := s.StartJob(context.TODO(), jr)
	if err != nil {
		t.Error(err)
	}
	t.Logf("initial status: %+v\n", status)

	jobID := &pb.JobID{JobID: status.JobID}

	// Wait for it to finish
	s.WaitOnJob(context.TODO(), jobID)

	status, err = s.GetJobStatus(context.TODO(), jobID)
	t.Logf("status: %+v\nerr: %+v", status, err)

	diff := deep.Equal(jr.Arguments, status.Stdout)
	if diff != nil {
		t.Error(diff)
	}
}
