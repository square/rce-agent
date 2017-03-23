// Copyright 2017 Square, Inc.

package cmd

import (
	"log"
	"sync"
)

// Repo is a thread-safe collection of Cmd used by an rce.Server.
type Repo interface {
	// Add new command to repo, identified by Cmd.Id.
	Add(*Cmd) error

	// Remove command from repo.
	Remove(id string) error

	// Return command from repo, or nil if doesn't exist.
	Get(id string) *Cmd

	// Return list of all command IDs in repos. The list is a copy and can change
	// between calls.
	All() []string
}

type repo struct {
	*sync.Mutex
	all map[string]*Cmd // keyed on id
}

// NewRepo makes a new empty Repo.
func NewRepo() Repo {
	return &repo{
		Mutex: &sync.Mutex{},
		all:   map[string]*Cmd{},
	}
}

func (r *repo) Add(cmd *Cmd) error {
	r.Lock()
	defer r.Unlock()
	if c := r.all[cmd.Id]; c != nil {
		return ErrDuplicateCommand
	}
	r.all[cmd.Id] = cmd
	return nil
}

func (r *repo) Remove(id string) error {
	r.Lock()
	defer r.Unlock()
	log.Printf("cmd=%s remove", id)
	delete(r.all, id)
	return nil
}

func (r *repo) Get(id string) *Cmd {
	r.Lock()
	defer r.Unlock()
	return r.all[id]
}

func (r *repo) All() []string {
	r.Lock()
	defer r.Unlock()
	all := make([]string, len(r.all))
	i := 0
	for id := range r.all {
		all[i] = id
	}
	return all
}
