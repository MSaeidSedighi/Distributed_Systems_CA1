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
	mu          sync.Mutex
	mapTasks    []Task
	reduceTasks []Task
	mapDone     bool
	reduceDone  bool
	nReduce     int
	nMap        int
	tasks       map[int]*TaskTracker
}

type TaskTracker struct {
	Status    TaskState
	WorkerId  string
	StartTime time.Time
}

func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

func (c *Coordinator) monitor() {
	for {
		c.mu.Lock()
		if c.Done() {
			c.mu.Unlock()
			return
		}
		
		now := time.Now()
		for _, tracker := range c.tasks {
			if tracker.Status == InProgress && now.Sub(tracker.StartTime) > 10*time.Second {
				tracker.Status = Pending
				tracker.WorkerId = ""
				tracker.StartTime = time.Time{}
			}
		}
		c.mu.Unlock()
		time.Sleep(time.Second)
	}
}

func (c *Coordinator) Done() bool {
	return c.mapDone && c.reduceDone
}

func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{
		mapTasks:    make([]Task, len(files)),
		reduceTasks: make([]Task, nReduce),
		tasks:       make(map[int]*TaskTracker),
		nReduce:     nReduce,
		nMap:        len(files),
	}

	for i, file := range files {
		c.mapTasks[i] = Task{Map, i, file, nReduce, len(files)}
		c.tasks[i] = &TaskTracker{Pending, "", time.Time{}}
	}

	for i := 0; i < nReduce; i++ {
		c.reduceTasks[i] = Task{Reduce, i, "", nReduce, len(files)}
		c.tasks[len(files)+i] = &TaskTracker{Pending, "", time.Time{}}
	}

	go c.monitor()
	c.server()
	return &c
}

func (c *Coordinator) AssignTask(args *TaskArgs, reply *TaskReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.mapDone {
		for i, task := range c.mapTasks {
			if c.tasks[i].Status == Pending {
				c.tasks[i].Status = InProgress
				c.tasks[i].WorkerId = args.WorkerId
				c.tasks[i].StartTime = time.Now()
				reply.Task = task
				return nil
			}
		}
		c.mapDone = true
		for i := 0; i < c.nMap; i++ {
			if c.tasks[i].Status != Completed {
				c.mapDone = false
				break
			}
		}
	}

	if c.mapDone && !c.reduceDone {
		for i, task := range c.reduceTasks {
			taskId := c.nMap + i
			if c.tasks[taskId].Status == Pending {
				c.tasks[taskId].Status = InProgress
				c.tasks[taskId].WorkerId = args.WorkerId
				c.tasks[taskId].StartTime = time.Now()
				reply.Task = task
				return nil
			}
		}
		c.reduceDone = true
		for i := c.nMap; i < c.nMap+c.nReduce; i++ {
			if c.tasks[i].Status != Completed {
				c.reduceDone = false
				break
			}
		}
	}

	reply.Task = Task{Type: None}
	return nil
}

func (c *Coordinator) TaskDone(args *ReportArgs, reply *ReportReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	taskId := args.TaskNum
	if args.TaskType == Reduce {
		taskId += c.nMap
	}

	if c.tasks[taskId].WorkerId != args.WorkerId {
		reply.Success = false
		return nil
	}

	c.tasks[taskId].Status = Completed
	reply.Success = true
	return nil
}