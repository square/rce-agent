// Copyright 2017-2023 Block, Inc.

package cmd_test

import (
	"testing"

	"github.com/go-test/deep"
	"github.com/square/rce-agent/cmd"
)

var err error

func TestFindByName(t *testing.T) {
	spec := cmd.Spec{
		Name: "found",
		Exec: []string{"/some/path", "arg1"},
	}

	rc := cmd.Runnable{
		spec,
		cmd.Spec{
			Name: "other.command",
			Exec: []string{"/other/path"},
		},
	}

	f, err := rc.FindByName(spec.Name)
	if err != nil {
		t.Error("command not found")
	}

	// Finds the command
	diff := deep.Equal(f, spec)
	if diff != nil {
		t.Error("wrong command found")
	}

	// Returns ErrCommandNotFound if not found
	f, err = rc.FindByName(spec.Name + ".NOT")
	if err != cmd.ErrCommandNotFound {
		t.Error("error not returned when not found")
	}
}

func TestValidateNoDuplicates(t *testing.T) {
	good := cmd.Runnable{
		cmd.Spec{"one", []string{}},
		cmd.Spec{"two", []string{}},
	}

	err = good.ValidateNoDuplicates()
	if err != nil {
		t.Error("error returned when no duplicates existed")
	}

	bad := cmd.Runnable{
		cmd.Spec{"one", []string{}},
		cmd.Spec{"one", []string{}},
	}

	err = bad.ValidateNoDuplicates()
	if err == nil {
		t.Error("no error returned")
	}
}

func TestValidateAbsPath(t *testing.T) {
	good := cmd.Spec{"good", []string{"/bin/ls"}}
	bad := cmd.Spec{"bad", []string{"./bin/tr"}}

	if good.ValidateAbsPath() != nil {
		t.Error("expected good validation failed")
	}

	if bad.ValidateAbsPath() == nil {
		t.Error("expected bad validation passed")
	}
}

func TestLoadCommands(t *testing.T) {
	got, err := cmd.LoadCommands("../test/runnable-cmds.yaml")
	if err != nil {
		t.Fatal(err)
	}
	expect := cmd.Runnable{
		cmd.Spec{
			Name: "exit.zero",
			Exec: []string{"/usr/bin/true"},
		},
		cmd.Spec{
			Name: "exit.one",
			Exec: []string{"/bin/false", "some-arg"},
		},
	}
	diff := deep.Equal(got, expect)
	if diff != nil {
		t.Error(diff)
	}
}
