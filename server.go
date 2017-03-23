// Copyright 2017 Square, Inc.

package rce

import (
	"errors"
	"log"
	"net"

	"github.com/square/rce-agent/cmd"
	pb "github.com/square/rce-agent/pb"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func init() {
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile | log.LUTC)
}

var (
	// ErrNotFound is returned for calls on nonexistent commands. The command
	// either never existed, or was reaped by the client calling Wait or Stop.
	ErrNotFound = errors.New("not found")
)

// A Server executes a whitelist of commands when called by clients.
type Server interface {
	// Start the gRPC server, non-blocking.
	StartServer() error

	// Stop the gRPC server gracefully.
	StopServer() error

	pb.RCEAgentServer
}

// Internal implementation of pb.RCEAgentServer interface.
type server struct {
	laddr      string       // host:port listen address
	repo       cmd.Repo     // running commands
	whitelist  cmd.Runnable // commands from config file
	grpcServer *grpc.Server // gRPC server instance of this agent
}

// NewServer makes a new Server.
func NewServer(laddr string, configFile string) (Server, error) {
	whitelist, err := cmd.LoadCommands(configFile)
	if err != nil {
		return nil, err
	}

	s := &server{
		laddr:     laddr,
		repo:      cmd.NewRepo(),
		whitelist: whitelist,
	}

	// TODO: use tls

	// Create a gRPC server and register this agent a implementing the
	// RCEAgentServer interface and protocol
	grpcServer := grpc.NewServer()
	pb.RegisterRCEAgentServer(grpcServer, s)
	s.grpcServer = grpcServer

	return s, nil
}

func (s *server) StartServer() error {
	lis, err := net.Listen("tcp", s.laddr)
	if err != nil {
		return err
	}
	go s.grpcServer.Serve(lis)
	return nil
}

func (s *server) StopServer() error {
	s.grpcServer.GracefulStop()
	return nil
}

// //////////////////////////////////////////////////////////////////////////
// pb.RCEAgentServer interface methods
// //////////////////////////////////////////////////////////////////////////

func (s *server) Start(ctx context.Context, c *pb.Command) (*pb.ID, error) {
	id := &pb.ID{}

	spec, err := s.whitelist.FindByName(c.Name)
	if err != nil {
		return id, err
	}

	// Append cmd request args to cmd spec args
	cmd := cmd.NewCmd(spec, append(spec.Args(), c.Arguments...))
	if err := s.repo.Add(cmd); err != nil {
		return id, err
	}

	log.Printf("cmd=%s: start: %s path: %s args: %v", cmd.Id, c.Name, spec.Path(), cmd.Args)
	cmd.Cmd.Start()
	id.ID = cmd.Id
	return id, nil
}

func (s *server) Wait(ctx context.Context, id *pb.ID) (*pb.Status, error) {
	log.Printf("cmd=%s: wait", id.ID)
	defer log.Printf("cmd=%s: wait return", id.ID)

	cmd := s.repo.Get(id.ID)
	if cmd == nil {
		return nil, ErrNotFound
	}

	<-cmd.Cmd.Start()
	finalStatus, err := s.GetStatus(ctx, id)

	// Reap the command
	s.repo.Remove(id.ID)

	return finalStatus, err
}

func (s *server) GetStatus(ctx context.Context, id *pb.ID) (*pb.Status, error) {
	log.Printf("cmd=%s: status", id.ID)

	cmd := s.repo.Get(id.ID)
	if cmd == nil {
		return nil, ErrNotFound
	}

	// Get go-cm/cmd.Status struct
	cmdStatus := cmd.Cmd.Status()

	// Make a pb.Status struct by adding and mapping some fields
	pbStatus := &pb.Status{
		ID:        cmd.Id,                // add
		Name:      cmd.Name,              // add
		ExitCode:  int64(cmdStatus.Exit), // map
		PID:       int64(cmdStatus.PID),  // map
		StartTime: cmdStatus.StartTs,     // map
		StopTime:  cmdStatus.StopTs,      // map
		Args:      cmd.Args,              // map
		Stdout:    cmdStatus.Stdout,      // same
		Stderr:    cmdStatus.Stderr,      // same
	}

	// Map go-cmd status to pb state
	switch {
	case cmdStatus.StartTs == 0 && cmdStatus.StopTs == 0:
		pbStatus.State = pb.STATE_PENDING
	case cmdStatus.StartTs > 0 && cmdStatus.StopTs == 0:
		pbStatus.State = pb.STATE_RUNNING
	case cmdStatus.StopTs > 0 && cmdStatus.Exit == 0:
		pbStatus.State = pb.STATE_COMPLETE
	case cmdStatus.StopTs > 0 && cmdStatus.Exit != 0:
		pbStatus.State = pb.STATE_FAIL
	default:
		pbStatus.State = pb.STATE_UNKNOWN
	}

	return pbStatus, nil
}

func (s *server) Stop(ctx context.Context, id *pb.ID) (*pb.Status, error) {
	log.Printf("cmd=%s: stop", id.ID)

	cmd := s.repo.Get(id.ID)
	if cmd == nil {
		return nil, ErrNotFound
	}

	cmd.Cmd.Stop()
	finalStatus, err := s.GetStatus(context.TODO(), id)

	// Reap the command
	s.repo.Remove(id.ID)

	return finalStatus, err
}

func (s *server) Running(empty *pb.Empty, stream pb.RCEAgent_RunningServer) error {
	log.Println("list running")
	for _, id := range s.repo.All() {
		if err := stream.Send(&pb.ID{ID: id}); err != nil {
			return err
		}
	}
	return nil
}
