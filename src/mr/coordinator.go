package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)

type Coordinator struct {
	// Your definitions here.
	mu            sync.Mutex
	mapTasks      []Task
	reduceTasks   []Task
	mapDone       bool
	reduceDone    bool
	nReduce       int
	nMap          int
	taskStatus    map[int]TaskStatus
	lastHeartbeat map[int]time.Time
}

type TaskStatus struct {
	State     TaskState
	WorkerId  string
	StartTime time.Time
}

type TaskState int

const (
	TaskStatePending TaskState = iota
	TaskStateInProgress
	TaskStateCompleted
)

// Your code here -- RPC handlers for the worker to call.

// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	// l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

// monitorTasks periodically checks for failed tasks and resets them
func (c *Coordinator) monitorTasks() {
	for {
		c.mu.Lock()
		now := time.Now()
		for i, status := range c.taskStatus {
			if status.State == TaskStateInProgress {
				// If task has been running for more than 10 seconds, consider it failed
				if now.Sub(status.StartTime) > 10*time.Second {
					c.taskStatus[i] = TaskStatus{
						State:     TaskStatePending,
						WorkerId:  "",
						StartTime: time.Time{},
					}
				}
			}
		}
		c.mu.Unlock()
		time.Sleep(time.Second)
	}
}

// Done returns true if all tasks are completed
func (c *Coordinator) Done() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mapDone && c.reduceDone
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{
		mapTasks:      make([]Task, len(files)),
		reduceTasks:   make([]Task, nReduce),
		taskStatus:    make(map[int]TaskStatus),
		lastHeartbeat: make(map[int]time.Time),
		nReduce:       nReduce,
		nMap:          len(files),
	}

	// Initialize map tasks
	for i, file := range files {
		c.mapTasks[i] = Task{
			Type:     MapTask,
			TaskNum:  i,
			Filename: file,
			NReduce:  nReduce,
			NMap:     len(files),
		}
		c.taskStatus[i] = TaskStatus{
			State:     TaskStatePending,
			WorkerId:  "",
			StartTime: time.Time{},
		}
	}

	// Initialize reduce tasks
	for i := 0; i < nReduce; i++ {
		c.reduceTasks[i] = Task{
			Type:    ReduceTask,
			TaskNum: i,
			NReduce: nReduce,
			NMap:    len(files),
		}
		c.taskStatus[len(files)+i] = TaskStatus{
			State:     TaskStatePending,
			WorkerId:  "",
			StartTime: time.Time{},
		}
	}

	// Start task monitoring
	go c.monitorTasks()

	c.server()
	return &c
}

// RequestTask handles worker requests for new tasks
func (c *Coordinator) RequestTask(args *TaskArgs, reply *TaskReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check for map tasks first
	if !c.mapDone {
		for i, task := range c.mapTasks {
			status := c.taskStatus[i]
			if status.State == TaskStatePending {
				// Assign task to worker
				c.taskStatus[i] = TaskStatus{
					State:     TaskStateInProgress,
					WorkerId:  args.WorkerId,
					StartTime: time.Now(),
				}
				reply.Task = task
				return nil
			}
		}
		// If no pending map tasks, check if all map tasks are completed
		allMapDone := true
		for i := 0; i < c.nMap; i++ {
			if c.taskStatus[i].State != TaskStateCompleted {
				allMapDone = false
				break
			}
		}
		if allMapDone {
			c.mapDone = true
		}
	}

	// If map phase is done, check for reduce tasks
	if c.mapDone && !c.reduceDone {
		for i, task := range c.reduceTasks {
			status := c.taskStatus[c.nMap+i]
			if status.State == TaskStatePending {
				// Assign task to worker
				c.taskStatus[c.nMap+i] = TaskStatus{
					State:     TaskStateInProgress,
					WorkerId:  args.WorkerId,
					StartTime: time.Now(),
				}
				reply.Task = task
				return nil
			}
		}
		// If no pending reduce tasks, check if all reduce tasks are completed
		allReduceDone := true
		for i := c.nMap; i < c.nMap+c.nReduce; i++ {
			if c.taskStatus[i].State != TaskStateCompleted {
				allReduceDone = false
				break
			}
		}
		if allReduceDone {
			c.reduceDone = true
		}
	}

	// If no tasks available, return NoTask
	reply.Task = Task{Type: NoTask}
	return nil
}

// ReportTask handles worker reports of task completion
func (c *Coordinator) ReportTask(args *ReportArgs, reply *ReportReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	taskIndex := args.TaskNum
	if args.TaskType == ReduceTask {
		taskIndex += c.nMap
	}

	// Verify the task is assigned to this worker
	status := c.taskStatus[taskIndex]
	if status.WorkerId != args.WorkerId {
		reply.Success = false
		return nil
	}

	// Mark task as completed
	c.taskStatus[taskIndex] = TaskStatus{
		State:     TaskStateCompleted,
		WorkerId:  args.WorkerId,
		StartTime: status.StartTime,
	}
	reply.Success = true
	return nil
}
