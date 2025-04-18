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

type KeyValue struct {
	Key   string
	Value string
}

func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

func Worker(mapf func(string, string) []KeyValue, reducef func(string, []string) string) {
	id := fmt.Sprintf("worker-%d", os.Getpid())

	for {
		task := getTask(id)
		if task.Type == None {
			time.Sleep(time.Second)
			continue
		}

		if task.Type == Map {
			doMap(task, mapf)
		} else {
			doReduce(task, reducef)
		}

		reportTask(id, task)
	}
}

func getTask(workerId string) Task {
	args := TaskArgs{workerId}
	reply := TaskReply{}

	if !call("Coordinator.AssignTask", &args, &reply) {
		os.Exit(0)
	}

	return reply.Task
}

func reportTask(workerId string, task Task) {
	args := ReportArgs{
		WorkerId: workerId,
		TaskNum:  task.TaskNum,
		TaskType: task.Type,
	}
	reply := ReportReply{}

	if !call("Coordinator.TaskDone", &args, &reply) {
		os.Exit(0)
	}
}

func doMap(task Task, mapf func(string, string) []KeyValue) {
	file, err := os.Open(task.Filename)
	if err != nil {
		log.Fatalf("cannot open %v", task.Filename)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", task.Filename)
	}
	file.Close()

	kva := mapf(task.Filename, string(content))
	intermediate := make([][]KeyValue, task.NReduce)

	for _, kv := range kva {
		reduceId := ihash(kv.Key) % task.NReduce
		intermediate[reduceId] = append(intermediate[reduceId], kv)
	}

	for reduceId, kvs := range intermediate {
		writeIntermediate(task.TaskNum, reduceId, kvs)
	}
}

func writeIntermediate(mapId, reduceId int, kvs []KeyValue) {
	tempFile, err := ioutil.TempFile("", "mr-tmp-")
	if err != nil {
		log.Fatal("Failed to create temp file")
	}

	enc := json.NewEncoder(tempFile)
	for _, kv := range kvs {
		if err := enc.Encode(&kv); err != nil {
			log.Fatal("Failed to encode KeyValue")
		}
	}

	finalName := fmt.Sprintf("mr-%d-%d", mapId, reduceId)
	tempFile.Close()
	os.Rename(tempFile.Name(), finalName)
}

func doReduce(task Task, reducef func(string, []string) string) {
	kva := readIntermediate(task)
	sort.Slice(kva, func(i, j int) bool {
		return kva[i].Key < kva[j].Key
	})

	tempFile, err := ioutil.TempFile("", "mr-tmp-")
	if err != nil {
		log.Fatal("Failed to create temp file")
	}

	i := 0
	for i < len(kva) {
		j := i + 1
		for j < len(kva) && kva[j].Key == kva[i].Key {
			j++
		}
		values := make([]string, j-i)
		for k := i; k < j; k++ {
			values[k-i] = kva[k].Value
		}
		output := reducef(kva[i].Key, values)
		fmt.Fprintf(tempFile, "%v %v\n", kva[i].Key, output)
		i = j
	}

	finalName := fmt.Sprintf("mr-out-%d", task.TaskNum)
	tempFile.Close()
	os.Rename(tempFile.Name(), finalName)
}

func readIntermediate(task Task) []KeyValue {
	var kva []KeyValue
	for i := 0; i < task.NMap; i++ {
		fileName := fmt.Sprintf("mr-%d-%d", i, task.TaskNum)
		file, err := os.Open(fileName)
		if err != nil {
			continue
		}

		dec := json.NewDecoder(file)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			kva = append(kva, kv)
		}
		file.Close()
	}
	return kva
}

func call(rpcname string, args interface{}, reply interface{}) bool {
	c, err := rpc.DialHTTP("unix", coordinatorSock())
	if err != nil {
		return false
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	return err == nil
}