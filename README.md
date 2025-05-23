# 🗂️ MapReduce in Go

## 📖 Overview

This project is a simplified implementation of the **MapReduce** framework in **Go (Golang)**. Inspired by Google’s MapReduce model, this system demonstrates how to distribute computational tasks across multiple workers using Remote Procedure Calls (RPCs). The project simulates a master-worker architecture where a **Coordinator** delegates `Map` and `Reduce` tasks to available **Workers**.

The system includes:

- Task coordination and fault tolerance
- RPC communication between coordinator and workers
- Intermediate file shuffling and reduction
- Custom hash partitioning for key distribution

---

## 📦 Components

### 📁 1. `rpc.go` — Shared Types and RPC Structures

Defines common enums, structs, and the Unix socket used for RPC between the coordinator and workers.

#### Enums
- `TaskType`: Indicates the type of task — `Map`, `Reduce`, or `None` (idle).
- `TaskState`: Represents the state of a task — `Pending`, `InProgress`, `Completed`.

#### Structs
- `Task`: Represents a single unit of work, with metadata like type, number, filename, and task counts.
- `TaskArgs`: Sent by a worker to request a task.
- `TaskReply`: Returned to the worker with task details.
- `ReportArgs`: Sent by a worker to report task completion.
- `ReportReply`: Acknowledgment response from the coordinator.

#### Functions
- `coordinatorSock()`: Returns a unique Unix socket path for this user to facilitate RPC communication.

---

### 🧠 2. `coordinator.go` — Coordinator Logic

Acts as the master of the MapReduce cluster. Its responsibilities include:

- Managing the global task queue
- Assigning tasks to idle workers
- Tracking task state and rescheduling failed tasks
- Initiating the reduce phase once mapping is done
- Finalizing the job after reduction

#### Key Features:
- Goroutines for concurrent task monitoring
- A main RPC server loop that listens to task requests and reports
- Timeout mechanism to reassign stuck tasks

---

### ⚙️ 3. `worker.go` — Worker Logic

The worker acts as the computational node in the system.

#### Key Responsibilities:
- Requests tasks from the coordinator via RPC
- Executes `Map` or `Reduce` functions depending on task type
- Partitions intermediate data using a hash function (`ihash`) on the key
- Stores intermediate results for reduce phase
- Reports task completion back to the coordinator

#### Intermediate File Handling:
- For `Map` tasks, it creates intermediate files (e.g., `mr-X-Y`) where:
  - `X` is the map task number
  - `Y` is the reduce bucket (determined by `ihash(key) % NReduce`)
- For `Reduce` tasks, it reads all relevant intermediate files and merges values by key

---

## 🛠️ Error Handling

The system includes basic error handling:

- RPC connection failures are logged and retried
- Task timeouts handled by the coordinator to ensure fault tolerance
- Files are written using temporary files to avoid partial writes
- Panics and file operation errors are logged and safely returned

---

## 🔁 How It Works (Flow Summary)

1. **Coordinator** initializes map tasks and starts listening on an RPC socket.
2. **Workers** repeatedly:
   - Request tasks via `GetTask`
   - Execute the assigned task (Map or Reduce)
   - Report completion via `ReportTask`
3. **Coordinator** monitors task progress, reschedules if workers fail, and transitions from Map to Reduce phase.
4. Once all reduce tasks are completed, the coordinator shuts down.

---