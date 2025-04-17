package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
	"strconv"
)

//
// example to show how to declare the arguments
// and reply for an RPC.
//

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// TaskType represents the type of task (Map or Reduce)
type TaskType int

const (
	MapTask TaskType = iota
	ReduceTask
	NoTask
)

// Task represents a unit of work to be done
type Task struct {
	Type     TaskType
	TaskNum  int
	Filename string
	NReduce  int
	NMap     int
}

// TaskArgs represents the arguments for requesting a task
type TaskArgs struct {
	WorkerId string
}

// TaskReply represents the reply for a task request
type TaskReply struct {
	Task Task
}

// ReportArgs represents the arguments for reporting task completion
type ReportArgs struct {
	WorkerId string
	TaskNum  int
	TaskType TaskType
}

// ReportReply represents the reply for task completion report
type ReportReply struct {
	Success bool
}

// Add your RPC definitions here.

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/5840-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
