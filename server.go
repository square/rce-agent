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

	"gopkg.in/yaml.v2"

	"google.golang.org/grpc"
	log "google.golang.org/grpc/grpclog"

	"github.com/go-cmd/cmd"
	pb "github.com/square/rce-agent/pb"
	"golang.org/x/net/context"
)

// Interface for the RCE Agent Server
// TODO: all these should be renamed from "Job" to "Command"
type Server interface {

	// Get all jobs after a specific start point
	GetJobs(since *pb.StartTime, stream pb.RCEAgent_GetJobsServer) error

	// Get the status of the job with the specified id
	GetJobStatus(ctx context.Context, id *pb.JobID) (*pb.JobStatus, error)

	// Start a new job. This is a non-blocking call.
	StartJob(ctx context.Context, details *pb.JobRequest) (*pb.JobStatus, error)
	// TODO: new rpc fn to stream output?

	// Given a job ID, stop that job.
	StopJob(ctx context.Context, id *pb.JobID) (*pb.JobStatus, error)

	// Start the server. This is a non-blocking call.
	Start() error

	// Stop the server.
	Stop() error

	// Blocks until jobID is complete
	WaitOnJob(ctx context.Context, id *pb.JobID) error
}

// TODO: clean up the job state machine, maybe formalize?
const (
	STATE_NOT_STARTED int64 = iota
	STATE_RUNNING
	STATE_COMPLETED
	STATE_FAILED
)

// non-exported struct
type server struct {
	jobsM            *sync.RWMutex          // Lock for allJobs
	allJobs          map[uint64]*runningJob // Map of all previously requested jobs. map[jobID] -> job //TODO
	runnableCommands Runnables              // Map of runnable commands for this agent: map[command name] -> command
	IDm              *sync.Mutex            // Mutex for nextID //TODO: uuids!
	nextID           uint64                 // the next available JobID //TODO: uuids!

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
		jobsM:            &sync.RWMutex{},
		allJobs:          map[uint64]*runningJob{},
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

// GetJobs list all jobs known by the server
func (s *server) GetJobs(since *pb.StartTime, stream pb.RCEAgent_GetJobsServer) error {
	log.Println("Getting all jobs.")
	s.jobsM.RLock()
	defer s.jobsM.RUnlock()
	return nil
	for id, job := range s.allJobs {
		if job.StartTime > since.StartTime {
			j := &pb.JobID{JobID: id}
			if err := stream.Send(j); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetJobStatus returns the job status of the given job id.
func (s *server) GetJobStatus(ctx context.Context, id *pb.JobID) (*pb.JobStatus, error) {
	log.Printf("job=%d: Status request received.", id.JobID)
	return s.getJobStatus(id.JobID)
}

// getJobStatus returns the job status of the given job id.
func (s *server) getJobStatus(id uint64) (*pb.JobStatus, error) {
	job, err := s.findJob(id)
	if err != nil {
		return nil, err
	}

	return job.getStatus(), nil
}

// StopJob stops a given job by ID
// TODO(cu): update this to use go-cmd/cmd's stop
func (s *server) StopJob(ctx context.Context, id *pb.JobID) (*pb.JobStatus, error) {
	log.Printf("job=%d: Stop request received.", id.JobID)

	//cmd.Stop()
	//s :=s.Status()
	// return s, nil

	job, err := s.findJob(id.JobID)
	if err != nil {
		return nil, err
	}

	if job.Status != STATE_RUNNING {
		return nil, fmt.Errorf("job=%d: Job not running.", id.JobID)
	}

	pid := int(job.getStatus().PID)

	// TODO: how would we test this?
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil, fmt.Errorf("job=%d: Process with pid %d not found: %v", id.JobID, pid, err)
	}

	// TODO: figure out the correct signal to send
	err = proc.Signal(syscall.SIGTERM)
	if err != nil {
		return nil, fmt.Errorf("job=%d: Error killing pid %d: %v", id.JobID, pid, err)
	}
	// TODO: need to wait until the job goroutine finishes updating status?

	log.Printf("job=%d: Successfully killed pid %d", id.JobID, pid)
	return job.getStatus(), nil
}

// StartJob starts a job in a goroutine and immediately returns a jobstatus.
// If a client wants updates on the job it must poll to determine
// completion/output.
func (s *server) StartJob(ctx context.Context, details *pb.JobRequest) (*pb.JobStatus, error) {
	cmdSpec, err := s.runnableCommands.FindByName(details.CommandName)
	if err != nil {
		return &pb.JobStatus{}, err
	}

	args := append(cmdSpec.Args(), details.Arguments...)

	cmd := cmd.NewCmd(cmdSpec.Path(), args...)

	job := &runningJob{
		Cmd:         cmd,
		JobID:       s.getNewJobID(),
		JobName:     details.JobName,
		Status:      STATE_NOT_STARTED,
		CommandName: details.CommandName,
		ExitCode:    -1,
		Args:        args,
	}
	log.Printf("job=%d: New job received (name: %s, commandName: %s, path: %s, args: %v).", job.JobID,
		job.JobName, job.CommandName, cmdSpec.Path(), job.Args)

	s.jobsM.Lock()
	s.allJobs[job.JobID] = job
	s.jobsM.Unlock()

	job.runLock.Lock()
	go job.do()

	// TODO: this should return an ID
	return job.getStatus(), nil
}

// WaitOnJob blocks until the specified job returns
func (s *server) WaitOnJob(ctx context.Context, id *pb.JobID) error {
	job, err := s.findJob(id.JobID)
	if err != nil {
		return err
	}

	job.wait()

	return nil
}

// findJob takes a job id and returns the matching job
func (s *server) findJob(id uint64) (*runningJob, error) {
	s.jobsM.Lock()
	defer s.jobsM.Unlock()

	job, ok := s.allJobs[id]
	if !ok {
		return nil, fmt.Errorf("job=%d: Job not found.", id)
	}

	return job, nil
}

// Get a new unique job id for each new job that gets requested.
// TODO: make this a uuid instead of an int
func (s *server) getNewJobID() uint64 {
	s.IDm.Lock()
	defer s.IDm.Unlock()
	myID := s.nextID
	s.nextID++
	return myID
}

// runningJob maages the underlying command execution mechanism
// TODO: rename to 'command', as 'Command' will become 'commandSpec'
// TODO(cu): rethink these names, especially 'Job', 'JobName', 'CommandName', 'Args'
type runningJob struct {
	*cmd.Cmd            // go-cmd/cmd that manages the execution
	runLock  sync.Mutex // Remains locked while command is running

	sync.Mutex
	JobID       uint64   // Unique ID of the job
	JobName     string   // Unsure of original intent. I believe this should be the 'name' from the config
	Status      int64    // Where the state machin lives, not currently used much TODO: rename to State
	CommandName string   // Unsure of original intent. I believe this is the path?
	StartTime   int64    // unixtime of the command's start
	FinishTime  int64    // unixtime of the command's finish
	ExitCode    int64    // exit code of command, -1 if failure or in-flight
	Args        []string // args passed to exec'd command
}

// getStatus blends info in runningJob with that in the Cmd to produce a pb.JobStatus
// Much of this funcitonality will eventually be a part of go-cmd/cmd
func (j *runningJob) getStatus() *pb.JobStatus {
	j.Mutex.Lock()
	defer j.Mutex.Unlock()

	cmdStatus := j.Cmd.Status()

	status := &pb.JobStatus{
		JobID:       j.JobID,
		JobName:     j.JobName,
		CommandName: cmdStatus.Cmd,
		PID:         int64(cmdStatus.PID),  // TODO: change cmd pkg type to int64?
		StartTime:   j.StartTime,           // will be added to cmd pkg
		FinishTime:  j.FinishTime,          // will be added to cmd pkg / TODO: rename to Complete
		ExitCode:    int64(cmdStatus.Exit), // TODO: change pb.JobStatus type to int?
		Args:        j.Args,                // will be added to cmd pkg
		Stdout:      cmdStatus.Stdout,
	}

	if cmdStatus.Error != nil {
		status.Error = cmdStatus.Error.Error() // this is golang errs that occur
	}

	return status
}

// do wraps the command execution logic and manages job metadata before/after
// it's running
func (j *runningJob) do() {
	j.Mutex.Lock()
	j.StartTime = time.Now().Unix()
	j.Status = STATE_RUNNING
	j.Mutex.Unlock()

	status := <-j.Cmd.Start()

	jobState := STATE_COMPLETED
	j.runLock.Unlock()

	if j.ExitCode != 0 {
		jobState = STATE_FAILED
	}

	j.Mutex.Lock()
	defer j.Mutex.Unlock()

	j.FinishTime = time.Now().Unix()
	j.ExitCode = int64(status.Exit)
	j.Status = jobState
}

// wait blocks until the job returns
func (j *runningJob) wait() {
	j.runLock.Lock()
	j.runLock.Unlock()
}
