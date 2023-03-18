// Copyright 2017-2023 Block, Inc.

package rce

import (
	"crypto/tls"
	"errors"
	"log"
	"net"

	"github.com/square/rce-agent/cmd"
	pb "github.com/square/rce-agent/pb"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
)

var (
	// ErrInvalidServerConfigAllowAnyCommand is returned by Server.StartServer() when
	// ServerConfig.AllowAnyCommand is true but ServerConfig.AllowedCommands is non-nil.
	ErrInvalidServerConfigAllowAnyCommand = errors.New("invalid ServerConfig: AllowAnyCommand is true but AllowedCommands is non-nil")

	// ErrInvalidServerConfigDisableSecurity is returned by Server.StartServer()
	// when ServerConfig.AllowAnyCommand is true and ServerConfig.TLS is nil but
	// ServerConfig.DisableSecurity is false.
	ErrInvalidServerConfigDisableSecurity = errors.New("invalid ServerConfig: AllowAnyCommand enabled but TLS is nil")

	// ErrCommandNotAllowed is safeguard error returned by the internal gRPC server when
	// ServerConfig.AllowedCommands is nil and ServerConfig.AllowAnyCommand is false.
	// This should not happen because these values are validated in Server.StartServer()
	// before starting the internal gRPC server. If this error occurs, there is a bug
	// in ServerConfig validation code.
	ErrCommandNotAllowed = errors.New("command not allowed")
)

// A Server executes a whitelist of commands when called by clients.
type Server interface {
	// Start the gRPC server, non-blocking.
	StartServer() error

	// Stop the gRPC server gracefully.
	StopServer() error

	pb.RCEAgentServer
}

// ServerConfig configures a Server.
type ServerConfig struct {
	// ----------------------------------------------------------------------
	// Required values

	// Addr is the required host:post listen address.
	Addr string

	// AllowedCommands is the list of commands the server is allowed to run.
	// By default, no commands are allowed; commands must be explicitly allowed.
	AllowedCommands cmd.Runnable

	// ----------------------------------------------------------------------
	// Optional values

	// AllowAnyCommand allows any commands if AllowedCommands is nil.
	// This is not recommended. If true, TLS must be specified (non-nil);
	// or, to enable AllowAnyCommand without TLS, DisableSecurity must be true.
	AllowAnyCommand bool

	// DisableSecurity allows AllowAnyCommand without TLS: an insecure server that
	// can execute any command from any client.
	//
	// This option should not be used.
	DisableSecurity bool

	// TLS specifies the TLS configuration for secure and verified communication.
	// Use TLSFiles.TLSConfig() to load TLS files and configure for server and
	// client verification.
	TLS *tls.Config
}

func NewServerWithConfig(cfg ServerConfig) Server {
	// Set log flags here so other pkgs can't override in their init().
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile | log.LUTC)

	s := &server{
		cfg: cfg,
		// --
		repo: cmd.NewRepo(),
	}

	// Create a gRPC server and register this agent a implementing the
	// RCEAgentServer interface and protocol
	var grpcServer *grpc.Server
	if cfg.TLS != nil {
		opt := grpc.Creds(credentials.NewTLS(cfg.TLS))
		grpcServer = grpc.NewServer(opt)
	} else {
		grpcServer = grpc.NewServer()
	}
	s.grpcServer = grpcServer

	return s
}

// Internal implementation of pb.RCEAgentServer interface.
type server struct {
	cfg ServerConfig
	// --
	repo       cmd.Repo     // running commands
	grpcServer *grpc.Server // gRPC server instance of this agent
}

// NewServer makes a new Server that listens on laddr and runs the whitelist
// of commands. If tlsConfig is nil, the sever is insecure.
func NewServer(laddr string, tlsConfig *tls.Config, whitelist cmd.Runnable) Server {
	return NewServerWithConfig(ServerConfig{
		Addr:            laddr,
		AllowedCommands: whitelist,
		TLS:             tlsConfig,
	})
}

func (s *server) StartServer() error {
	// Validate the combination of server configs that disable security
	if s.cfg.AllowAnyCommand {
		// To allow any command, there can't be any allow list
		if s.cfg.AllowedCommands != nil {
			return ErrInvalidServerConfigAllowAnyCommand
		} else {
			log.Printf("WARNING: all commands are allowed!\n")
		}

		// And to to allow any command without TLS, the user must explicitly
		// disble all security
		if s.cfg.TLS == nil && !s.cfg.DisableSecurity {
			return ErrInvalidServerConfigDisableSecurity
		} else {
			log.Printf("WARNING: all security is disabled!\n")
		}
	}

	// Register the RCEAgent service with the gRPC server.
	pb.RegisterRCEAgentServer(s.grpcServer, s)

	lis, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	go s.grpcServer.Serve(lis)
	if s.cfg.TLS != nil {
		log.Printf("secure server listening on %s", s.cfg.Addr)
	} else {
		log.Printf("insecure server listening on %s", s.cfg.Addr)
	}
	return nil
}

func (s *server) StopServer() error {
	s.grpcServer.GracefulStop()
	log.Printf("server stopped on %s", s.cfg.Addr)
	return nil
}

// //////////////////////////////////////////////////////////////////////////
// pb.RCEAgentServer interface methods
// //////////////////////////////////////////////////////////////////////////

func (s *server) Start(ctx context.Context, c *pb.Command) (*pb.ID, error) {
	id := &pb.ID{} // @todo we return this on error, but should be "return nil, <err>"

	var rceCmd *cmd.Cmd // from AllowedCommands or an arbitrary if AllowAnyCommand
	var path string     // for logging below
	if s.cfg.AllowedCommands != nil {
		spec, err := s.cfg.AllowedCommands.FindByName(c.Name)
		if err != nil {
			log.Printf("unknown command: %s", c.Name)
			return id, grpc.Errorf(codes.InvalidArgument, "unknown command: %s", c.Name)
		}
		// Append cmd request args to cmd spec args
		rceCmd = cmd.NewCmd(spec, append(spec.Args(), c.Arguments...))

		path = spec.Path()
	} else if s.cfg.AllowAnyCommand {
		// Make a spec for this arbitrary command
		spec := cmd.Spec{
			Name: c.Name, // any command, like "/usr/local/bin/gofmt"
			Exec: append([]string{c.Name}, c.Arguments...),
		}
		rceCmd = cmd.NewCmd(spec, c.Arguments)

		path = c.Name
	} else {
		return id, ErrCommandNotAllowed
	}

	if err := s.repo.Add(rceCmd); err != nil {
		// This should never happen
		log.Printf("duplicate command: %+v", rceCmd)
		return id, grpc.Errorf(codes.AlreadyExists, "duplicate command: %s", rceCmd.Id)
	}

	log.Printf("cmd=%s: start: %s path: %s args: %v", rceCmd.Id, c.Name, path, rceCmd.Args)
	rceCmd.Cmd.Start()
	id.ID = rceCmd.Id
	return id, nil
}

func (s *server) Wait(ctx context.Context, id *pb.ID) (*pb.Status, error) {
	log.Printf("cmd=%s: wait", id.ID)
	defer log.Printf("cmd=%s: wait return", id.ID)

	cmd := s.repo.Get(id.ID)
	if cmd == nil {
		return nil, notFound(id)
	}
	// Reap the command
	defer s.repo.Remove(id.ID)

	// Wait for command or ctx to finish
	select {
	case <-cmd.Cmd.Done():
	case <-ctx.Done():
	}

	// Get final status of command and convert to pb.Status. If ctx was canceled
	// and command still running, its status will indicate this and ctx.Err()
	// below will return an error, else it will return nil.
	return mapStatus(cmd), ctx.Err()
}

func (s *server) GetStatus(ctx context.Context, id *pb.ID) (*pb.Status, error) {
	log.Printf("cmd=%s: status", id.ID)
	cmd := s.repo.Get(id.ID)
	if cmd == nil {
		return nil, notFound(id)
	}
	return mapStatus(cmd), nil
}

func (s *server) Stop(ctx context.Context, id *pb.ID) (*pb.Empty, error) {
	log.Printf("cmd=%s: stop", id.ID)

	cmd := s.repo.Get(id.ID)
	if cmd == nil {
		return nil, notFound(id)
	}

	cmd.Cmd.Stop()

	return &pb.Empty{}, nil
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

func notFound(id *pb.ID) error {
	return grpc.Errorf(codes.NotFound, "command ID %s not found", id.ID)
}

func mapStatus(cmd *cmd.Cmd) *pb.Status {
	cmdStatus := cmd.Cmd.Status()

	var errMsg string
	if cmdStatus.Error != nil {
		errMsg = cmdStatus.Error.Error()
	}

	// Make a pb.Status struct by adding and mapping some fields
	pbStatus := &pb.Status{
		ID:        cmd.Id,                // add
		Name:      cmd.Name,              // add
		ExitCode:  int64(cmdStatus.Exit), // map
		Error:     errMsg,                // map
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

	return pbStatus
}
