// Copyright 2017 Square, Inc.

package rce

import (
	"crypto/tls"
	"fmt"
	"io"
	"time"

	"github.com/square/rce-agent/pb"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Interface for sending commands to the remote agent
type Client interface {

	// Get All jobs that have been submitted after the input time
	GetJobs(since time.Time) ([]uint64, error)

	// Get the status of the given job id.
	// An error is returned if the jobid does not exist
	GetJobStatus(jobID uint64) (*JobStatus, error)

	// Starts a job. This is a non-blocking operation.
	// A job status will be returned immediately. It is up to the
	// client user to continuously poll for status.
	StartJob(jobName, jobCommand string, args []string) (*JobStatus, error)

	// Stops a job given the job id.
	// If the job id is not found, or the job is not currently running,
	// a non-nil error will be returned.
	// This will issue a SIGTERM signal to the running job. The job status
	// of that job will be returned. Because that job is killed with a SIGTERM
	// the exit code will not be available.
	StopJob(jobID uint64) (*JobStatus, error)

	// Get the hostname of the agent that the client is connected to.
	GetAgentHostname() string

	// Get the port of the agent that the client is connected to.
	GetAgentPort() string

	// Open the connection to the agent
	Open(host, port string) error

	// Closes the connection to the agent.
	Close() error
}

type JobStatus struct {
	Hostname    string   // hostname that the job ran on
	Port        string   // port through which the agent is listening
	JobID       uint64   // id of this job (this is not necessarily unique across all hosts
	JobName     string   // user provided name for this job (for easy identification)
	Pid         int64    // PID of the job that ran/is running
	Status      int64    // State of the job: not yet started, running, completed
	CommandName string   // The name for the command requested
	StartTime   int64    // Start time of job in unix time
	FinishTime  int64    // finish time of job in unix time. Will be 0 if not yet finished
	ExitCode    int64    // Exit code of the command. -1 if the command has not yet finished or is signaled
	Args        []string // Arguments to command
	Stdout      []string // stdout from the command.
	Stderr      []string // stderr from the command.
	Error       string   // Error (From running the command)
}

// TODO: use this if it turns out to be cleaner than passing in individual args
type JobRequest struct {
	JobName     string
	CommandName string
	Arguments   []string
}

// non-exported intentionally
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

// Queries the agent for all JobIDs that have been submitted before the input time.
// Agents (currently) do not persist job data between restarts, so any jobs
// that have occured prior to the most recent start will not be returned.
func (c *client) GetJobs(since time.Time) ([]uint64, error) {
	startTime := &pb.StartTime{
		StartTime: since.Unix(),
	}

	jobs := []uint64{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stream, err := c.agent.GetJobs(ctx, startTime)
	if err != nil {
		return nil, err
	}

	for {
		jobStatus, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		jobs = append(jobs, jobStatus.JobID)
	}

	return jobs, nil
}

// Given the id of a job, return the status of that job.
// A nil JobStatus and a non-nil error will be returned if the
// job cannot be found.
func (c *client) GetJobStatus(jobID uint64) (*JobStatus, error) {
	req := &pb.JobID{
		JobID: jobID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	status, err := c.agent.GetJobStatus(ctx, req)
	if err != nil {
		return nil, err
	}

	jobStatus := c.getJobStatus(status)
	return jobStatus, nil
}

// Given the id of a job, stop that job. If the job is not found, or the job
// is not currently running. A non-nil error will be returned, and JobStatus will
// be nil.
func (c *client) StopJob(jobID uint64) (*JobStatus, error) {
	req := &pb.JobID{
		JobID: jobID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	status, err := c.agent.StopJob(ctx, req)
	if err != nil {
		return nil, err
	}

	jobStatus := c.getJobStatus(status)
	return jobStatus, nil
}

// Start a given job.
// TODO: consider taking a JobRequest struct as input instead
func (c *client) StartJob(jobName string, jobCommand string, args []string) (*JobStatus, error) {
	request := &pb.JobRequest{
		JobName:     jobName,
		CommandName: jobCommand,
		Arguments:   args,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	status, err := c.agent.StartJob(ctx, request)
	if err != nil {
		return nil, err
	}

	jobStatus := c.getJobStatus(status)
	return jobStatus, nil
}

// Given a pb.JobStatus, convert that into the JobStatus for the user
func (c *client) getJobStatus(s *pb.JobStatus) *JobStatus {
	jobStatus := &JobStatus{
		Hostname:    c.host,
		Port:        c.port,
		JobID:       s.JobID,
		JobName:     s.JobName,
		Pid:         s.PID,
		Status:      s.Status,
		CommandName: s.CommandName,
		StartTime:   s.StartTime,
		FinishTime:  s.FinishTime,
		ExitCode:    s.ExitCode,
		Args:        s.Args,
		Stdout:      s.Stdout,
		Stderr:      s.Stderr,
		Error:       s.Error,
	}
	return jobStatus
}

// Prints job status to stdout. A useful debugging tool.
func (js *JobStatus) Print() {
	fmt.Printf("Hostname    %v \n", js.Hostname)
	fmt.Printf("Port        %v \n", js.Port)
	fmt.Printf("JobName     %v \n", js.JobName)
	fmt.Printf("JobID       %v \n", js.JobID)
	fmt.Printf("PID         %v \n", js.Pid)
	fmt.Printf("Status      %v \n", js.Status)
	fmt.Printf("CommandName %v \n", js.CommandName)
	fmt.Printf("StartTime   %v \n", js.StartTime)
	fmt.Printf("FinishTime  %v \n", js.FinishTime)
	fmt.Printf("ExitCode    %v \n", js.ExitCode)
	fmt.Printf("Args        %v \n", js.Args)
	fmt.Printf("Stdout      %v \n", js.Stdout)
	fmt.Printf("Stderr      %v \n", js.Stderr)
	fmt.Printf("Error       %v \n", js.Error)
}
