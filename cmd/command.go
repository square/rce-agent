// Copyright 2017-2020 Square, Inc.

// Package cmd provides command file specs and structures used by an rce.Server.
package cmd

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"strings"

	gocmd "github.com/go-cmd/cmd"
	"github.com/gofrs/uuid"
	"gopkg.in/yaml.v2"
)

var (
	ErrCommandNotFound  = errors.New("command not found")
	ErrDuplicateName    = errors.New("duplicate command name found")
	ErrDuplicateCommand = errors.New("duplicate command in repo")
	ErrRelativePath     = errors.New("command uses relative path")
	ErrNoCommands       = errors.New("no commands parsed")
)

// Cmd represents a running command.
type Cmd struct {
	Id   string
	Name string
	Cmd  *gocmd.Cmd
	Args []string
}

// NewCmd makes a new Cmd with the given Spec and args, and assigns it an ID.
func NewCmd(s Spec, args []string) *Cmd {
	cmd := gocmd.NewCmd(s.Path(), args...)
	return &Cmd{
		Id:   id(),
		Name: s.Name,
		Cmd:  cmd,
		Args: args,
	}
}

func id() string {
	uuid, _ := uuid.NewV4()
	return strings.Replace(uuid.String(), "-", "", -1)
}

// //////////////////////////////////////////////////////////////////////////
// Spec
// //////////////////////////////////////////////////////////////////////////

// Spec represents one command in a YAML config file. See LoadCommands for the
// file structure.
type Spec struct {
	// Short, unique name of the command. Example: "lxc-ls". This is only an alias.
	Name string `yaml:"name"`

	// Exec args, first being the absolute cmd path. Example: ["/usr/bin/lxc-ls", "--active"].
	Exec []string `yaml:"exec"`
}

// ValidateAbsPath returns ErrRelativePath if the Spec's path is not an absolute path.
func (c Spec) ValidateAbsPath() error {
	if ok := filepath.IsAbs(c.Path()); !ok {
		return ErrRelativePath
	}
	return nil
}

// Path returns the path part of a Spec.
func (c Spec) Path() string {
	return c.Exec[0]
}

// Args returns the args part of a Spec.
func (c Spec) Args() []string {
	return c.Exec[1:]
}

// //////////////////////////////////////////////////////////////////////////
// Runnable
// //////////////////////////////////////////////////////////////////////////

type Runnable []Spec

type specFile struct {
	Commands Runnable `yaml:"commands"`
}

// LoadCommands loads all command Spec from a YAML config file. The file structure is:
//
//   ---
//   commands:
//     - name: exit.zero
//       exec: [/usr/bin/true]
//	   - name: exit.one
//	     exec:
//         - /bin/false
//         - some-arg
//
// Name must be unique. The first exec value must be an absolute command path.
// Additional exec values are optional and always included in the order listed.
func LoadCommands(file string) (Runnable, error) {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return Runnable{}, err
	}

	var s specFile
	if err := yaml.Unmarshal(bytes, &s); err != nil {
		return Runnable{}, err
	}

	if len(s.Commands) == 0 {
		return Runnable{}, ErrNoCommands
	}

	if err := s.Commands.Validate(); err != nil {
		return Runnable{}, err
	}

	return s.Commands, nil
}

// Validate validates a list of Spec and returns an error if any invalid.
func (r Runnable) Validate() error {
	var err error

	err = r.ValidateNoDuplicates()
	if err != nil {
		return err
	}

	for _, c := range r {
		err = c.ValidateAbsPath()
		if err != nil {
			return err
		}
	}

	return nil
}

// ValidateNoDuplicates returns an ErrDuplicateName if the list of Spec contains
// duplicate names.
func (r Runnable) ValidateNoDuplicates() error {
	names := make(map[string]bool)

	for _, c := range r {
		if names[c.Name] {
			return ErrDuplicateName
		}

		names[c.Name] = true
	}

	return nil
}

// FindByName returns a Spec matching the given name.
func (r Runnable) FindByName(name string) (Spec, error) {
	for _, c := range r {
		if name == c.Name {
			return c, nil
		}
	}

	return Spec{}, ErrCommandNotFound
}
