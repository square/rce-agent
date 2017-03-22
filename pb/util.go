package pb

import "fmt"

// Prints cmd status to stdout. A useful debugging tool.
func (s *CommandStatus) Print() {
	fmt.Printf("CommandName %v \n", s.CommandName)
	fmt.Printf("CommandID   %v \n", s.CommandID)
	fmt.Printf("PID         %v \n", s.PID)
	fmt.Printf("Status      %v \n", s.Status)
	fmt.Printf("CommandName %v \n", s.CommandName)
	fmt.Printf("StartTime   %v \n", s.StartTime)
	fmt.Printf("FinishTime  %v \n", s.FinishTime)
	fmt.Printf("ExitCode    %v \n", s.ExitCode)
	fmt.Printf("Args        %v \n", s.Args)
	fmt.Printf("Stdout      %v \n", s.Stdout)
	fmt.Printf("Stderr      %v \n", s.Stderr)
	fmt.Printf("Error       %v \n", s.Error)
}
