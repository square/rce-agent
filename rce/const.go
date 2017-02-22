// Copyright 2017 Square, Inc.

package rce

type JobState int64

const (
	NotYetStarted JobState = iota
	Running       JobState = iota
	Completed     JobState = iota
)
