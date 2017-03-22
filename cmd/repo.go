// Copyright 2017 Square, Inc.

package cmd

import (
	"log"
	"sync"
)

type Repo interface {
	Add(*Cmd) error
	Remove(id string) error
	Get(id string) *Cmd
	All() []string
}

type repo struct {
	*sync.Mutex
	all map[string]*Cmd // keyed on id
}

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
