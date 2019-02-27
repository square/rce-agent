# RCE Agent

[![Build Status](https://travis-ci.org/square/rce-agent.svg?branch=master)](https://travis-ci.org/square/rce-agent) [![Go Report Card](https://goreportcard.com/badge/github.com/square/rce-agent)](https://goreportcard.com/report/github.com/square/rce-agent) [![GoDoc](https://godoc.org/github.com/square/rce-agent?status.svg)](https://godoc.org/github.com/square/rce-agent)

rce-agent is a gRPC-based Remote Command Execution (RCE) client and server.
The server (or "agent") runs on a remote host and executes a whitelist of
shell commands specified in a config file. The client calls the server to
execute whitelist commands. TLS is used to secure and authenticate the connection.

rce-agent replaces SSH or other methods of remote code execution. There are no
passwords&mdash;only TLS certificates&mdash;and commands are limited to a whitelist.
This eliminates the need for SSH keys, passwords, or forwarding.
