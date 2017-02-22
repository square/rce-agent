// Copyright 2017 Square, Inc

package rceserver

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"

	pb "github.com/square/spincycle/agent/rce"
)

// TODO
func TestGetJobs(t *testing.T) {
}

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Printf("result")
	// some code here to check arguments perhaps?
	os.Exit(0)
}

// TODO
var execCommand = exec.Command

// TODO: lol these tests don't work
func TestRunJob(t *testing.T) {
	s := &rceAgentServer{
		allJobs:          map[uint64]*runningJob{},
		jobsM:            &sync.RWMutex{},
		runnableCommands: map[string]string{},
		nextID:           1,
		IDm:              &sync.Mutex{},
	}
	s.runnableCommands["exit 0"] = "exit"
	job := &runningJob{
		stdoutM: &sync.Mutex{},
		stderrM: &sync.Mutex{},
		statusM: &sync.Mutex{},
	}
	status := &pb.JobStatus{
		JobID:       0,
		JobName:     "test-job-1",
		Status:      int64(pb.NotYetStarted),
		CommandName: "exit 0",
		StartTime:   0,
		ExitCode:    -1,
		Args:        []string{"0"},
	}
	job.Status = status
	s.allJobs[0] = job

	defer func() { execCommand = exec.Command }()
	// This will block until the job is done running
	s.runJob(job)

	if status.ExitCode != 0 {
		t.Fatalf("Expected exit code 0, but got %d", status.ExitCode)
	}
}

// TODO: write more tests
