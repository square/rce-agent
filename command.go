// Copyright 2017 Square, Inc.

package rce

import (
	"errors"
	"path/filepath"
)

// Command represents a command that can be run by RCE Agent
type Command struct {
	// short name of the command
	Name string `yaml:"name"`

	// arguments to be passed to exec
	Exec []string `yaml:"exec"`
}

type Runnables []Command

var (
	ErrCommandNotFound = errors.New("command not found")
	ErrNoSuchFile      = errors.New("file not found")

	// Errors from failing validation
	ErrDuplicateName = errors.New("duplicate command name found")
	ErrRelativePath  = errors.New("command uses relative path")
)

// FindByName returns a Command matching the given name
func (r Runnables) FindByName(name string) (Command, error) {
	for _, c := range r {
		if name == c.Name {
			return c, nil
		}
	}

	return Command{}, ErrCommandNotFound
}

// Validate runs validations on all Commands in the collection
func (r Runnables) Validate() error {
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
// contains two Commands with the same name
func (r Runnables) ValidateNoDuplicates() error {
	names := make(map[string]bool)

	for _, c := range r {
		if names[c.Name] {
			return ErrDuplicateName
		}

		names[c.Name] = true
	}

	return nil
}

// ValidateAbsPath returns ErrRelativePath if the Command's path is not an
// absolute path
func (c Command) ValidateAbsPath() error {
	ok := filepath.IsAbs(c.Path())

	if !ok {
		return ErrRelativePath
	}

	return nil
}

// Path returns the path part of a Command
func (c Command) Path() string {
	return c.Exec[0]
}

// Args returns the args part of a Command
func (c Command) Args() []string {
	return c.Exec[1:]
}

// TODO: do a NoSuchFile check before a command gets run
