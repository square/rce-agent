// Copyright 2017 Square, Inc.

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

// A Client is used to send commands to a remote agent.
type Client interface {
	// Open the connection to the agent
	Open(host, port string) error

	// Closes the connection to the agent.
	Close() error

	// Get All commandss that have been submitted after the input time
	GetCommands(since time.Time) ([]uint64, error)

	// Get the status of the given cmd id.
	// An error is returned if the cmdid does not exist
	GetCommandStatus(cmdID uint64) (*pb.CommandStatus, error)

	// Starts a cmd. This is a non-blocking operation.
	// A cmd status will be returned immediately. It is up to the
	// client user to continuously poll for status.
	StartCommand(cmdName string, args []string) (*pb.CommandStatus, error)

	// Stops a cmd given the cmd id.
	// If the cmd id is not found, or the cmd is not currently running,
	// a non-nil error will be returned.
	// This will issue a SIGTERM signal to the running cmd. The cmd status
	// of that cmd will be returned. Because that cmd is killed with a SIGTERM
	// the exit code will not be available.
	StopCommand(cmdID uint64) (*pb.CommandStatus, error)

	// Get the hostname of the agent that the client is connected to.
	GetAgentHostname() string

	// Get the port of the agent that the client is connected to.
	GetAgentPort() string
}

type client struct {
	host      string
	port      string
	conn      *grpc.ClientConn
	agent     pb.RCEAgentClient
	tlsConfig *tls.Config
}

// Create a new gRPC client
func NewClient(tlsConfig *tls.Config) Client {
	return &client{tlsConfig: tlsConfig}
}

// Open the connection to the RCE Agent
func (c *client) Open(host, port string) error {
	var opt grpc.DialOption
	if c.tlsConfig == nil {
		opt = grpc.WithInsecure()
	} else {
		creds := credentials.NewTLS(c.tlsConfig)
		opt = grpc.WithTransportCredentials(creds)
	}
	conn, err := grpc.Dial(host+":"+port, opt)
	if err != nil {
		return err
	}
	c.conn = conn
	c.agent = pb.NewRCEAgentClient(conn)
	c.host = host
	c.port = port
	return nil
}

// Close the connection to the RCE Agent
func (c *client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Get the hostname of the agent that this client connects to.
func (c *client) GetAgentHostname() string {
	return c.host
}

// Get the port of the agent that this client connects to.
func (c *client) GetAgentPort() string {
	return c.port
}

// Queries the agent for all CommandIDs that have been submitted before the input time.
// Agents (currently) do not persist cmd data between restarts, so any cmds
// that have occured prior to the most recent start will not be returned.
func (c *client) GetCommands(since time.Time) ([]uint64, error) {
	startTime := &pb.StartTime{
		StartTime: since.Unix(),
	}

	cmds := []uint64{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stream, err := c.agent.GetCommands(ctx, startTime)
	if err != nil {
		return nil, err
	}

	for {
		cmdStatus, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		cmds = append(cmds, cmdStatus.CommandID)
	}

	return cmds, nil
}

// Given the id of a cmd, return the status of that cmd.
// A nil CommandStatus and a non-nil error will be returned if the
// cmd cannot be found.
func (c *client) GetCommandStatus(cmdID uint64) (*pb.CommandStatus, error) {
	req := &pb.CommandID{
		CommandID: cmdID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	return c.agent.GetCommandStatus(ctx, req)
}

// Given the id of a cmd, stop that cmd. If the cmd is not found, or the cmd
// is not currently running. A non-nil error will be returned, and CommandStatus will
// be nil.
func (c *client) StopCommand(cmdID uint64) (*pb.CommandStatus, error) {
	req := &pb.CommandID{
		CommandID: cmdID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	return c.agent.StopCommand(ctx, req)
}

// Start a given cmd.
// TODO: consider taking a CommandRequest struct as input instead
func (c *client) StartCommand(cmdName string, args []string) (*pb.CommandStatus, error) {
	request := &pb.CommandRequest{
		CommandName: cmdName,
		Arguments:   args,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	status, err := c.agent.StartCommand(ctx, request)
	if err != nil {
		return nil, err
	}

	return c.GetCommandStatus(status.CommandID)
}
