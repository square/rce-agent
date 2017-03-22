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

	// Returns the agent hostname and port.
	AgentAddr() (string, string)

	Start(cmdName string, args []string) (id string, err error)
	Wait(id string) (*pb.Status, error)
	GetStatus(id string) (*pb.Status, error)
	Stop(id string) (*pb.Status, error)
	Running() ([]string, error)
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

func (c *client) Stop(id string) (*pb.Status, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return c.agent.Stop(ctx, &pb.ID{ID: id})
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
