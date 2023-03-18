// Copyright 2017-2023 Block, Inc.

package pb

import "fmt"

// Prints cmd status to stdout. A useful debugging tool.
func (s *Status) Print() {
	fmt.Printf("ID          %v \n", s.ID)
	fmt.Printf("Name        %v \n", s.Name)
	fmt.Printf("PID         %v \n", s.PID)
	fmt.Printf("State       %v \n", s.State)
	fmt.Printf("StartTime   %v \n", s.StartTime)
	fmt.Printf("FinishTime  %v \n", s.StopTime)
	fmt.Printf("ExitCode    %v \n", s.ExitCode)
	fmt.Printf("Args        %v \n", s.Args)
	fmt.Printf("Stdout      %v \n", s.Stdout)
	fmt.Printf("Stderr      %v \n", s.Stderr)
	fmt.Printf("Error       %v \n", s.Error)
}
