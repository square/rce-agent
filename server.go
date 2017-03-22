// Copyright 2017 Square, Inc.

package rce

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/go-cmd/cmd"
	pb "github.com/square/rce-agent/pb"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	log "google.golang.org/grpc/grpclog"
	"gopkg.in/yaml.v2"
)

// A Server accepts RPC calls to execute allowed commands.
type Server interface {
	// Start the server. This is a non-blocking call.
	Start() error

	// Stop the server.
	Stop() error

	// Get all cmds after a specific start point
	GetCommands(since *pb.StartTime, stream pb.RCEAgent_GetCommandsServer) error

	// Get the status of the cmd with the specified id
	GetCommandStatus(ctx context.Context, id *pb.CommandID) (*pb.CommandStatus, error)

	// Start a new cmd. This is a non-blocking call.
	StartCommand(ctx context.Context, details *pb.CommandRequest) (*pb.CommandStatus, error)

	// Given a cmd ID, stop that cmd.
	StopCommand(ctx context.Context, id *pb.CommandID) (*pb.CommandStatus, error)

	// Blocks until cmdID is complete
	WaitOnCommand(ctx context.Context, id *pb.CommandID) error
}

// TODO: clean up the cmd state machine, maybe formalize?
const (
	STATE_NOT_STARTED int64 = iota
	STATE_RUNNING
	STATE_COMPLETED
	STATE_FAILED
)

// non-exported struct
type server struct {
	cmdsM            *sync.RWMutex              // Lock for allCommands
	allCommands      map[uint64]*runningCommand // Map of all previously requested cmds. map[cmdID] -> cmd //TODO
	runnableCommands Runnables                  // Map of runnable commands for this agent: map[command name] -> command
	IDm              *sync.Mutex                // Mutex for nextID //TODO: uuids!
	nextID           uint64                     // the next available CommandID //TODO: uuids!

	// server stuff
	laddr      string       // host:port listen address
	grpcServer *grpc.Server // gRPC server instance that this agent is using
}

// NewServer creates a new gRPC server listening on the given host:port (laddr)
// and using the given configFile to load allowed commands. The server is not
// started.
func NewServer(laddr string, configFile string) (Server, error) {

	cfg, err := LoadRunnableCommands(configFile)
	if err != nil {
		return nil, err
	}
	s := &server{
		cmdsM:            &sync.RWMutex{},
		allCommands:      map[uint64]*runningCommand{},
		nextID:           0,
		runnableCommands: cfg.Commands,
		IDm:              &sync.Mutex{},
		laddr:            laddr,
	}
	grpcServer := grpc.NewServer()
	pb.RegisterRCEAgentServer(grpcServer, s)
	// TODO: use tls

	s.grpcServer = grpcServer
	return s, nil
}

// Config struct  for parsing the input config yaml file
type Config struct {
	Commands Runnables `yaml:"commands"` // Whitelist of available commands
	TLS      struct {  // SSL Configuration settings
		sslCAFile  string `yaml:"sslCAFile"`  // SSL CA filepath
		sslCrtFile string `yaml:"sslCrtFile"` // SSL CRT filepath
		sslKeyFile string `yaml:"sslKeyFile"` // SSL Key Filepath
	} `yaml:"tlsconfig"` // TODO:  ssl isnt even tested yet
}

// LoadRunableCommands loads the config file into memory
func LoadRunnableCommands(configFile string) (*Config, error) {
	f, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	err = yaml.Unmarshal(f, cfg)
	if err != nil {
		return nil, err
	}

	err = cfg.Commands.Validate()
	if err != nil {
		return cfg, err // should we return a zero-value Config here?
	}

	return cfg, nil
}

// Start starts the gRPC server. This function is non blocking and will return
// an error if there is one starting the listener.  It is up to the user to
// call s.Stop() to properly stop the server.
func (s *server) Start() error {
	lis, err := net.Listen("tcp", s.laddr)
	if err != nil {
		return err
	}
	go s.grpcServer.Serve(lis)
	return nil
}

// Stop stops the gRPC server.
func (s *server) Stop() error {
	s.grpcServer.GracefulStop()
	return nil
}

// GetCommands list all cmds known by the server
func (s *server) GetCommands(since *pb.StartTime, stream pb.RCEAgent_GetCommandsServer) error {
	log.Println("Getting all cmds.")
	s.cmdsM.RLock()
	defer s.cmdsM.RUnlock()
	return nil
	for id, cmd := range s.allCommands {
		if cmd.StartTime > since.StartTime {
			j := &pb.CommandID{CommandID: id}
			if err := stream.Send(j); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetCommandStatus returns the cmd status of the given cmd id.
func (s *server) GetCommandStatus(ctx context.Context, id *pb.CommandID) (*pb.CommandStatus, error) {
	log.Printf("cmd=%d: Status request received.", id.CommandID)
	return s.getCommandStatus(id.CommandID)
}

// getCommandStatus returns the cmd status of the given cmd id.
func (s *server) getCommandStatus(id uint64) (*pb.CommandStatus, error) {
	cmd, err := s.findCommand(id)
	if err != nil {
		return nil, err
	}

	return cmd.getStatus(), nil
}

// StopCommand stops a given cmd by ID
// TODO(cu): update this to use go-cmd/cmd's stop
func (s *server) StopCommand(ctx context.Context, id *pb.CommandID) (*pb.CommandStatus, error) {
	log.Printf("cmd=%d: Stop request received.", id.CommandID)

	//cmd.Stop()
	//s :=s.Status()
	// return s, nil

	cmd, err := s.findCommand(id.CommandID)
	if err != nil {
		return nil, err
	}

	if cmd.Status != STATE_RUNNING {
		return nil, fmt.Errorf("cmd=%d: Command not running.", id.CommandID)
	}

	pid := int(cmd.getStatus().PID)

	// TODO: how would we test this?
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil, fmt.Errorf("cmd=%d: Process with pid %d not found: %v", id.CommandID, pid, err)
	}

	// TODO: figure out the correct signal to send
	err = proc.Signal(syscall.SIGTERM)
	if err != nil {
		return nil, fmt.Errorf("cmd=%d: Error killing pid %d: %v", id.CommandID, pid, err)
	}
	// TODO: need to wait until the cmd goroutine finishes updating status?

	log.Printf("cmd=%d: Successfully killed pid %d", id.CommandID, pid)
	return cmd.getStatus(), nil
}

// StartCommand starts a cmd in a goroutine and immediately returns a cmdstatus.
// If a client wants updates on the cmd it must poll to determine
// completion/output.
func (s *server) StartCommand(ctx context.Context, details *pb.CommandRequest) (*pb.CommandStatus, error) {
	cmdSpec, err := s.runnableCommands.FindByName(details.CommandName)
	if err != nil {
		return &pb.CommandStatus{}, err
	}

	args := append(cmdSpec.Args(), details.Arguments...)

	cmd := cmd.NewCmd(cmdSpec.Path(), args...)

	r := &runningCommand{
		Cmd:         cmd,
		CommandID:   s.getNewCommandID(),
		Status:      STATE_NOT_STARTED,
		CommandName: details.CommandName,
		ExitCode:    -1,
		Args:        args,
	}
	log.Printf("cmd=%d: New cmd received (name: %s, path: %s, args: %v).",
		r.CommandID, r.CommandName, cmdSpec.Path(), cmd.Args)

	s.cmdsM.Lock()
	s.allCommands[r.CommandID] = r
	s.cmdsM.Unlock()

	r.runLock.Lock()
	go r.do()

	// TODO: this should return an ID
	return r.getStatus(), nil
}

// WaitOnCommand blocks until the specified cmd returns
func (s *server) WaitOnCommand(ctx context.Context, id *pb.CommandID) error {
	cmd, err := s.findCommand(id.CommandID)
	if err != nil {
		return err
	}

	cmd.wait()

	return nil
}

// findCommand takes a cmd id and returns the matching cmd
func (s *server) findCommand(id uint64) (*runningCommand, error) {
	s.cmdsM.Lock()
	defer s.cmdsM.Unlock()

	cmd, ok := s.allCommands[id]
	if !ok {
		return nil, fmt.Errorf("cmd=%d: Command not found.", id)
	}

	return cmd, nil
}

// Get a new unique cmd id for each new cmd that gets requested.
// TODO: make this a uuid instead of an int
func (s *server) getNewCommandID() uint64 {
	s.IDm.Lock()
	defer s.IDm.Unlock()
	myID := s.nextID
	s.nextID++
	return myID
}

// runningCommand maages the underlying command execution mechanism
type runningCommand struct {
	*cmd.Cmd            // go-cmd/cmd that manages the execution
	runLock  sync.Mutex // Remains locked while command is running

	sync.Mutex
	CommandID   uint64   // Unique ID of the cmd
	CommandName string   // Unsure of original intent. I believe this should be the 'name' from the config
	Status      int64    // Where the state machin lives, not currently used much TODO: rename to State
	StartTime   int64    // unixtime of the command's start
	FinishTime  int64    // unixtime of the command's finish
	ExitCode    int64    // exit code of command, -1 if failure or in-flight
	Args        []string // args passed to exec'd command
}

// getStatus blends info in runningCommand with that in the Cmd to produce a pb.CommandStatus
// Much of this funcitonality will eventually be a part of go-cmd/cmd
func (j *runningCommand) getStatus() *pb.CommandStatus {
	j.Mutex.Lock()
	defer j.Mutex.Unlock()

	cmdStatus := j.Cmd.Status()

	status := &pb.CommandStatus{
		CommandID:   j.CommandID,
		CommandName: cmdStatus.Cmd,
		PID:         int64(cmdStatus.PID),  // TODO: change cmd pkg type to int64?
		StartTime:   j.StartTime,           // will be added to cmd pkg
		FinishTime:  j.FinishTime,          // will be added to cmd pkg / TODO: rename to Complete
		ExitCode:    int64(cmdStatus.Exit), // TODO: change pb.CommandStatus type to int?
		Args:        j.Args,                // will be added to cmd pkg
		Stdout:      cmdStatus.Stdout,
	}

	if cmdStatus.Error != nil {
		status.Error = cmdStatus.Error.Error() // this is golang errs that occur
	}

	return status
}

// do wraps the command execution logic and manages cmd metadata before/after
// it's running
func (j *runningCommand) do() {
	j.Mutex.Lock()
	j.StartTime = time.Now().Unix()
	j.Status = STATE_RUNNING
	j.Mutex.Unlock()

	status := <-j.Cmd.Start()

	cmdState := STATE_COMPLETED
	j.runLock.Unlock()

	if j.ExitCode != 0 {
		cmdState = STATE_FAILED
	}

	j.Mutex.Lock()
	defer j.Mutex.Unlock()

	j.FinishTime = time.Now().Unix()
	j.ExitCode = int64(status.Exit)
	j.Status = cmdState
}

// wait blocks until the cmd returns
func (j *runningCommand) wait() {
	j.runLock.Lock()
	j.runLock.Unlock()
}
