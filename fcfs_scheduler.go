/*
FCFS scheduler for Single CPU
Each process has a sequence of CPU time and IO time switching
Each process has time it is added at start

We have N IO devices. Each have FCFS queue

Example input for proc1 and proc2 (CPU(x) means x time units of CPU time, IO(y) means y time units of IO time):
CPU(5) IO(2) CPU(1) IO(20) CPU(8)
CPU(4) IO(10) CPU(2)


Task:
1. measure time to complete all processes

- Процесс может в трех состояниях: CPU, IO, ready
- У нас 2 очереди: на CPU и на IO
- Мы не знаем, что будет дальше
- Процесс сам считает кол-во выполненных шагов
- Каждый квант времеи Планировщик смотрит только на наличие свободного места на CPU и IO и на очередь

*/
package main

import(
    "fmt"
    "bufio"
    "os"
    "strings"
    "strconv"
)


type Scheduler interface {
  Tick()
  AssignToResource(r Resourcer, p *Process)
  ReleaseResource(r Resourcer)
}

type RR struct {
  timeQuantum int
}

type Machine struct {
  s Scheduler

  Cpus *MultiCoreCpu
  IO1 *Resource
  IO2 *Resource

  CpuQueue []Process
  IO1Queue []Process
  IO2Queue []Process

  allProcesses []Process

  currentTick int
}

func (m *Machine) CheckQueueAndAssign(queue []Process, rs Resourcer) {
  for _, proc := range queue {
    if proc.state != READY {
      panic("Process in Resource queue is not ready")
    }
    resource, err := rs.GetFree()
    if err != nil {
      break
    }
    m.s.AssignToResource(resource, &proc)
    queue = queue[1:]
  }
}

func (m *Machine) debugPringState() {
	for _, cpu := range m.Cpus.cpus {
		s := 0
		if cpu.state == BUSY {
			s = 1
		}
		fmt.Printf("%d ", s)
	}
	fmt.Print("| ")

  fmt.Printf("%d ", m.IO1.state == BUSY)
  fmt.Printf("%d", m.IO2.state == BUSY)

	fmt.Println()
}

func (m *Machine) checkAndAssignStep() {
  // Assign waiting processes to resources
  m.CheckQueueAndAssign(m.CpuQueue, m.Cpus)
  m.CheckQueueAndAssign(m.IO1Queue, m.IO1)
  m.CheckQueueAndAssign(m.IO2Queue, m.IO2)
}

func (m *Machine) AfterTick() {
  // Check if any process is done
  for _, res := range m.Cpus.cpus {
    if res.state == BUSY && res.currentProc.CurTask().IsFinished() {
      res.currentProc.state = TERMINATED
      res.currentProc = nil
    }
  }
}

/*
0. Check completed and free + add them to waiting queue
1. Assign processes to CPU and IO
2. Increment counters
3. Set completed states
3. Debug output of current state
*/
// TODO: implmement completness checks and Queue management
// TODO: remove Tick, replace with IncrementCounters for all and UpdateState for running process
// TODO: add ready queue (cpu queue) and I/O queue (blocked state)
// TODO: add Preemt mechanism to stop process and move it to queue

func (m *Machine) Tick() {
  m.checkAndAssignStep()
  m.currentTick++
  for _, proc := range m.allProcesses {
    proc.Tick()
  }

  // debug print info for each cpu and io
  m.debugPringState()

  for _, proc := range m.allProcesses {
    proc.AfterTick()
  }

  m.AfterTick()
}

func allDone(processes []Process) bool {
  for _, proc := range processes {
    if proc.state != DONE {
      return false
    }
  }
  return true
}

func scheduler(processes []Process) {
  machine := Machine{make([]Process, 0), make([]Process, 0)}
  // infinite loop until all processes are done
  // for every tick, check if any process is ready to run
  cpuQ := machine.CpuQueue
  ioQ := machine.IoQueue

  for {
    // Check if all processes are done
    if allDone(processes) {
      break
    }
    // If have waiting process in queue, assign it to CPU
    if len(cpuQ) > 0 {
      // Get free CPU
      cpu, err := machine.GetFreeCpu()
      if err != nil {
        fmt.Println(err)
        continue
      }
      // Assign process to CPU
      machine.AssignToResource(&cpu, &cpuQ[0])
      // Remove process from queue
      cpuQ = cpuQ[1:]
    }

  }
}

func main() {
  fmt.Println("FCFS Scheduler")

  // Get input
  reader := bufio.NewReader(os.Stdin)
  fmt.Print("Enter number of processes: ")
  numProcStr, _ := reader.ReadString('\n')
  numProc, _ := strconv.Atoi(strings.TrimSpace(numProcStr))

  // Create processes
  processes := make([]Process, numProc)
  for i := 0; i < numProc; i++ {
    processes[i].id = i
    fmt.Printf("Enter arrival time for process %d: ", i)
    arrivalTimeStr, _ := reader.ReadString('\n')
    processes[i].arrivalTime, _ = strconv.Atoi(strings.TrimSpace(arrivalTimeStr))
  }

  // Run scheduler
  fmt.Println("Running scheduler")
  scheduler(processes)
}
