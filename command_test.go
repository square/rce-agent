// Copyright 2017 Square, Inc.

package rce

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/go-test/deep"
	yaml "gopkg.in/yaml.v2"
)

var err error

func TestFindByName(t *testing.T) {
	cmd := Command{
		Name: "found",
		Exec: []string{"/some/path", "arg1"},
	}

	rc := Runnables{
		cmd,
		Command{
			Name: "other.command",
			Exec: []string{"/other/path"},
		},
	}

	var f Command

	f, err = rc.FindByName(cmd.Name)
	if err != nil {
		t.Error("command not found")
	}

	// Finds the command
	diff := deep.Equal(f, cmd)
	if diff != nil {
		t.Error("wrong command found")
	}

	// Returns ErrCommandNotFound if not found
	f, err = rc.FindByName(cmd.Name + ".NOT")
	if err != ErrCommandNotFound {
		t.Error("error not returned when not found")
	}
}

func TestValidateNoDuplicates(t *testing.T) {
	good := Runnables{
		Command{"one", []string{}},
		Command{"two", []string{}},
	}

	err = good.ValidateNoDuplicates()
	if err != nil {
		t.Error("error returned when no duplicates existed")
	}

	bad := Runnables{
		Command{"one", []string{}},
		Command{"one", []string{}},
	}

	err = bad.ValidateNoDuplicates()
	if err == nil {
		t.Error("no error returned")
	}
}

func TestValidateAbsPath(t *testing.T) {
	good := Command{"good", []string{"/bin/ls"}}
	bad := Command{"bad", []string{"./bin/tr"}}

	if good.ValidateAbsPath() != nil {
		t.Error("expected good validation failed")
	}

	if bad.ValidateAbsPath() == nil {
		t.Error("expected bad validation passed")
	}
}

func TestLoadCommands(t *testing.T) {
	configFile := "test/runnable-cmds.yaml"

	dir, _ := filepath.Abs(configFile)
	f, _ := ioutil.ReadFile(dir)

	var conf struct {
		Commands Runnables `yaml:"commands"`
	}

	yaml.Unmarshal(f, &conf)

	cmds := conf.Commands

	t.Logf("Loaded: %+v\n", cmds)

	expected := Runnables{
		Command{
			Name: "exit.zero",
			Exec: []string{"/usr/bin/true"},
		},
		Command{
			Name: "exit.one",
			Exec: []string{"/bin/false", "some-arg"},
		},
	}

	diff := deep.Equal(cmds, expected)
	if diff != nil {
		t.Error(diff)
	}
}
