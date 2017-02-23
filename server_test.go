// Copyright 2017 Square, Inc

package rce_test

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/square/rce-agent"
	"github.com/square/rce-agent/pb"
	"golang.org/x/net/context"
)

const PORT = "5501"

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
	s, err := rce.NewServer(PORT, "test/runnable-cmds.yaml")
	if err != nil {
		t.Fatal(err)
	}

	jr := &pb.JobRequest{
		JobName:     "foo",
		CommandName: "exitZero",
		Arguments:   []string{},
	}

	status, err := s.StartJob(context.TODO(), jr)
	if err != nil {
		t.Error(err)
	}

	if status.JobName != "foo" {
		t.Errorf("got JobName = %s, expected exitZero", status.JobName)
	}

	// @todo @fixme This causes a segfault
	/*

		if status.FinishTime != 0 {
			t.Errorf("got FinishTime = %d, expected zero", status.FinishTime)
		}

		// Let job finish
		time.Sleep(100 * time.Millisecond)

		if status.FinishTime == 0 {
			t.Errorf("got FinishTime = %d, expected > 0", status.FinishTime)
		}
	*/
}

// TODO: write more tests
