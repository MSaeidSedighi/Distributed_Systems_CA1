package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"sort"
	"time"
)

// Map functions return a slice of KeyValue.
type KeyValue struct {
	Key   string
	Value string
}

// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// main/mrworker.go calls this function.
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Generate a unique worker ID
	workerId := fmt.Sprintf("worker-%d", os.Getpid())

	for {
		// Request a task from the coordinator
		args := TaskArgs{WorkerId: workerId}
		reply := TaskReply{}

		if !call("Coordinator.RequestTask", &args, &reply) {
			log.Fatal("Failed to request task")
		}

		task := reply.Task
		if task.Type == NoTask {
			// No tasks available, wait and try again
			time.Sleep(time.Second)
			continue
		}

		// Execute the task
		if task.Type == MapTask {
			executeMapTask(task, mapf)
		} else if task.Type == ReduceTask {
			executeReduceTask(task, reducef)
		}

		// Report task completion
		reportArgs := ReportArgs{
			WorkerId: workerId,
			TaskNum:  task.TaskNum,
			TaskType: task.Type,
		}
		reportReply := ReportReply{}

		if !call("Coordinator.ReportTask", &reportArgs, &reportReply) {
			log.Fatal("Failed to report task completion")
		}
	}
}

func executeMapTask(task Task, mapf func(string, string) []KeyValue) {
	// Read input file
	file, err := os.Open(task.Filename)
	if err != nil {
		log.Fatalf("cannot open %v", task.Filename)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", task.Filename)
	}
	file.Close()

	// Execute map function
	kva := mapf(task.Filename, string(content))

	// Create intermediate files for each reduce task
	intermediate := make([][]KeyValue, task.NReduce)
	for _, kv := range kva {
		reduceTaskNum := ihash(kv.Key) % task.NReduce
		intermediate[reduceTaskNum] = append(intermediate[reduceTaskNum], kv)
	}

	// Write intermediate files
	for i := 0; i < task.NReduce; i++ {
		// Create temporary file
		tempFile, err := ioutil.TempFile("", "mr-tmp-*")
		if err != nil {
			log.Fatal("Failed to create temp file")
		}

		// Encode intermediate results
		enc := json.NewEncoder(tempFile)
		for _, kv := range intermediate[i] {
			err := enc.Encode(&kv)
			if err != nil {
				log.Fatal("Failed to encode KeyValue")
			}
		}

		// Rename temp file to final intermediate file
		intermediateFile := fmt.Sprintf("mr-%d-%d", task.TaskNum, i)
		tempFile.Close()
		os.Rename(tempFile.Name(), intermediateFile)
	}
}

func executeReduceTask(task Task, reducef func(string, []string) string) {
	// Read all intermediate files for this reduce task
	intermediate := []KeyValue{}
	for i := 0; i < task.NMap; i++ {
		filename := fmt.Sprintf("mr-%d-%d", i, task.TaskNum)
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("cannot open %v", filename)
		}

		dec := json.NewDecoder(file)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			intermediate = append(intermediate, kv)
		}
		file.Close()
	}

	// Sort intermediate key-value pairs by key
	sort.Slice(intermediate, func(i, j int) bool {
		return intermediate[i].Key < intermediate[j].Key
	})

	// Create output file
	tempFile, err := ioutil.TempFile("", "mr-tmp-*")
	if err != nil {
		log.Fatal("Failed to create temp file")
	}

	// Group values by key and apply reduce function
	i := 0
	for i < len(intermediate) {
		j := i + 1
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Value)
		}
		output := reducef(intermediate[i].Key, values)
		fmt.Fprintf(tempFile, "%v %v\n", intermediate[i].Key, output)
		i = j
	}

	// Rename temp file to final output file
	outputFile := fmt.Sprintf("mr-out-%d", task.TaskNum)
	tempFile.Close()
	os.Rename(tempFile.Name(), outputFile)
}

// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		fmt.Printf("call failed!\n")
	}
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
