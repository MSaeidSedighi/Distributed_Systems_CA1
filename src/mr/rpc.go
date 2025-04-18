package mr

import "os"
import "strconv"

type TaskType int

const (
	Map TaskType = iota
	Reduce
	None
)

type TaskState int

const (
	Pending TaskState = iota
	InProgress
	Completed
)

type Task struct {
	Type     TaskType
	TaskNum  int
	Filename string
	NReduce  int
	NMap     int
}

type TaskArgs struct {
	WorkerId string
}

type TaskReply struct {
	Task Task
}

type ReportArgs struct {
	WorkerId string
	TaskNum  int
	TaskType TaskType
}

type ReportReply struct {
	Success bool
}

func coordinatorSock() string {
	return "/var/tmp/5840-mr-" + strconv.Itoa(os.Getuid())
}