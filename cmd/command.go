// Copyright 2017 Square, Inc.

package cmd

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"

	gocmd "github.com/go-cmd/cmd"
	"github.com/nu7hatch/gouuid"
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
	*sync.Mutex
	Id   string
	Name string
	Cmd  *gocmd.Cmd
	Args []string // args passed to exec'd command
}

func NewCmd(s Spec, args []string) *Cmd {
	cmd := gocmd.NewCmd(s.Path(), args...)
	return &Cmd{
		Mutex: &sync.Mutex{},
		Id:    id(),
		Name:  s.Name,
		Cmd:   cmd,
		Args:  args,
	}
}

func id() string {
	uuid, _ := uuid.NewV4()
	return strings.Replace(uuid.String(), "-", "", -1)
}

// //////////////////////////////////////////////////////////////////////////
// Spec
// //////////////////////////////////////////////////////////////////////////

// Spec represents a whitelist command that the RCE agent can run.
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

// A specFile represents the YAML structure of whitelist commands. Example:
// ---
// commands:
//   - name: exit.zero
//     exec: [/usr/bin/true]
//	 - name: exit.one
//	   exec:
//       - /bin/false
//       - some-arg
type specFile struct {
	Commands Runnable `yaml:"commands"`
}

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

// Validate runs validations on all Specs in the collection
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

// ValidateNoDuplicates returns an ErrDuplicateName if the list of runnables
// contains two Specs with the same name
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

// FindByName returns a Spec matching the given name
func (r Runnable) FindByName(name string) (Spec, error) {
	for _, c := range r {
		if name == c.Name {
			return c, nil
		}
	}

	return Spec{}, ErrCommandNotFound
}
