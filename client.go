// Copyright 2017 Square, Inc.

// Package rce provides a gRPC-based Remote Code Execution client and server.
// The server (or "agent") runs on a remote host and executes a whitelist of
// shell commands specified in a config file. The client calls the server to
// execute whitelist commands. Commands from different clients run concurrently;
// there are no safeguards against conflicting or incompatible commands.
package rce

import (
	"crypto/tls"
	"io"
	"time"

	"github.com/square/rce-agent/pb"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	// ConnectTimeout describes the total timeout for establishing a client
	// connection to the rceagent server.
	ConnectTimeout = time.Duration(10) * time.Second

	// ConnectBackoffMaxDelay configures the dialer to use the
	// provided maximum delay when backing off after
	// failed connection attempts.
	ConnectBackoffMaxDelay = time.Duration(2) * time.Second
)

// A Client calls a remote agent (server) to execute commands.
type Client interface {
	// Connect to a remote agent.
	Open(host, port string) error

	// Close connection to a remote agent.
	Close() error

	// Return hostname and port of remote agent, if connected.
	AgentAddr() (string, string)

	// Start a command on the remote agent. Must be connected first by calling
	// Connect. This call is non-blocking. It returns the ID of the command or
	// an error.
	Start(cmdName string, args []string) (id string, err error)

	// Wait for a command on the remote agent. This call blocks until the command
	// completes. It returns the final statue of the command or an error.
	Wait(id string) (*pb.Status, error)

	// Get the status of a running command. This is safe to call by multiple
	// goroutines. ErrNotFound is returned if Wait or Stop has already been
	// called.
	GetStatus(id string) (*pb.Status, error)

	// Stop a running command. ErrNotFound is returne if Wait or Stop has already
	// been called.
	Stop(id string) error

	// Return a list of all running command IDs.
	Running() ([]string, error)
}

type client struct {
	host      string
	port      string
	conn      *grpc.ClientConn
	agent     pb.RCEAgentClient
	tlsConfig *tls.Config
}

// NewClient makes a new Client.
func NewClient(tlsConfig *tls.Config) Client {
	return &client{tlsConfig: tlsConfig}
}

func (c *client) Open(host, port string) error {
	var opt grpc.DialOption
	if c.tlsConfig == nil {
		opt = grpc.WithInsecure()
	} else {
		creds := credentials.NewTLS(c.tlsConfig)
		err := creds.OverrideServerName(host)
		if err != nil {
			return err
		}
		opt = grpc.WithTransportCredentials(creds)
	}
	conn, err := grpc.Dial(
		host+":"+port,
		opt, // insecure or with TLS

		// Block = actually connect. Timeout = max time to retry on failure
		// (no option to set retry count). Backoff delay = time between retries,
		// up to Timeout.
		grpc.WithBlock(),
		grpc.WithTimeout(ConnectTimeout),
		grpc.WithBackoffMaxDelay(ConnectBackoffMaxDelay),
	)
	if err != nil {
		return err
	}
	c.conn = conn
	c.agent = pb.NewRCEAgentClient(conn)
	c.host = host
	c.port = port
	return nil
}

func (c *client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *client) AgentAddr() (string, string) {
	return c.host, c.port
}

func (c *client) Start(cmdName string, args []string) (string, error) {
	cmd := &pb.Command{
		Name:      cmdName,
		Arguments: args,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	id, err := c.agent.Start(ctx, cmd)
	if err != nil {
		return "", err
	}

	return id.ID, nil
}

func (c *client) Wait(id string) (*pb.Status, error) {
	return c.agent.Wait(context.TODO(), &pb.ID{ID: id})
}

func (c *client) GetStatus(id string) (*pb.Status, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return c.agent.GetStatus(ctx, &pb.ID{ID: id})
}

func (c *client) Stop(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := c.agent.Stop(ctx, &pb.ID{ID: id})
	return err
}

func (c *client) Running() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stream, err := c.agent.Running(ctx, &pb.Empty{})
	if err != nil {
		return nil, err
	}

	ids := []string{}
	for {
		id, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		ids = append(ids, id.ID)
	}

	return ids, nil
}
