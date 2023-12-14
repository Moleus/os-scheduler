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

import (
	"fmt"
	"log/slog"
)

type GlobalTimer interface {
  GetCurrentTick() int
}

type Machine struct {
  cpuScheduler Scheduler
  io1Scheduler Scheduler
  io2Scheduler Scheduler

  unscheduledProcs []*Process
  runningProcs []*Process
  clock *Clock
}

type Clock struct {
  currentTick int
}

func NewMachine(cpuScheduler Scheduler, io1Scheduler Scheduler, io2Scheduler Scheduler, clock *Clock) Machine {
  return Machine{cpuScheduler, io1Scheduler, io2Scheduler, []*Process{}, []*Process{}, clock}
}

func (c *Clock) GetCurrentTick() int {
  return c.currentTick
}

func (m *Machine) GetCurrentTick() int {
  return m.clock.currentTick
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

func (m *Machine) allDone() bool {
  return len(m.runningProcs) == 0 && len(m.unscheduledProcs) == 0
}

func (m *Machine) loop() {
  for {
    if m.allDone() {
      break
    }
    m.tick()
  }
}

func (m *Machine) tick() {
  m.checkForNewProcs()

  m.cpuScheduler.CheckRunningProcs()
  m.io1Scheduler.CheckRunningProcs()
  m.io2Scheduler.CheckRunningProcs()

  m.handleAllEvictedProcs()

  m.cpuScheduler.ProcessQueue()
  m.io1Scheduler.ProcessQueue()
  m.io2Scheduler.ProcessQueue()

  m.clock.currentTick++

  for _, p := range m.runningProcs {
    p.Tick()
  }
}

func (m *Machine) checkForNewProcs() {
  for i, p := range m.unscheduledProcs {
    if m.clock.currentTick < p.arrivalTime {
      // skip this proc. It's not time yet
      continue
    }
    slog.Info(fmt.Sprintf("Process %d arrived at tick %d\n", p.id, m.GetCurrentTick()))
    m.cpuScheduler.PushToQueue(p)
    // remove this proc from array
    m.unscheduledProcs = append(m.unscheduledProcs[:i], m.unscheduledProcs[i+1:]...)
    m.runningProcs = append(m.runningProcs, p)
  }
}

func (m *Machine) handleAllEvictedProcs() {
  ep := m.cpuScheduler.GetEvictedProcs()
  ep = append(ep, m.io1Scheduler.GetEvictedProcs()...)
  ep = append(ep, m.io2Scheduler.GetEvictedProcs()...)
  for _, p := range ep {
    m.handleEvictedProc(p)
  }
  m.cpuScheduler.ClearEvictedProcs()
  m.io1Scheduler.ClearEvictedProcs()
  m.io2Scheduler.ClearEvictedProcs()
}

func (m *Machine) handleEvictedProc(p *Process) {
  switch p.state {
  case TERMINATED:
    slog.Info(fmt.Sprintf("Process %d is done at tick %d\n", p.id, m.GetCurrentTick()))
    // remove from running procs
    m.runningProcs = append(m.runningProcs[:p.id], m.runningProcs[p.id+1:]...)
  case RUNNING, READY:
    // not finished or came from IO
    m.cpuScheduler.PushToQueue(p)
  case BLOCKED:
    m.pushToIO(p)
  case READS_IO:
    // TODO: proc not changing from IO to READY
    panic(fmt.Sprintf("Process %d evicted in READS_IO state but IO scheduler is nonpreemptive\n", p.id))
  }
}

func (m *Machine) pushToIO(p *Process) {
  switch p.CurTask().ResouceType {
  case IO1:
    slog.Debug(fmt.Sprintf("Process %d is blocked on IO1\n", p.id))
    m.io1Scheduler.PushToQueue(p)
  case IO2:
    slog.Debug(fmt.Sprintf("Process %d is blocked on IO2\n", p.id))
    m.io2Scheduler.PushToQueue(p)
  case CPU:
    panic(fmt.Sprintf("Proc %d is blocked by current task is cpu", p.id))
  }
}

func (m *Machine) Run(processes []*Process) {
  m.unscheduledProcs = processes

  m.loop()
}
