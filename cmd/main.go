package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/Moleus/os-solver/pkg/xlsx"
	"github.com/xuri/excelize/v2"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	log "github.com/Moleus/os-solver/pkg/logging"
	m "github.com/Moleus/os-solver/pkg/machine"
)

var (
	cpuCount          = flag.Int("cpus", 4, "Number of CPUs")
	inputFile         = flag.String("input", "", "Input file")
	outputFile        = flag.String("output", "result.txt", "Output file")
	procStatsFile     = flag.String("procStats", "procStats.txt", "Process stats file")
	schedAlgo         = flag.String("algo", "fcfs", "Scheduling algorithm (default: fcfs). Possible values: fcfs, rr1, rr2, spn, srt, hrrn, rr")
	roundRobinQuantum = flag.Int("quantum", 4, "Round robin quantum (default: 4)")
	arrivalInterval   = flag.Int("interval", 2, "Proc arrival interval (default: 2)")
	logLevel          = flag.String("log", "debug", "Log level (default: debug)")
	exportXlsx        = flag.String("export-xlsx", "", "Path for creating xlsx report")
)

func calcArrivalTime(procId int) int {
	return procId * *arrivalInterval
}

func ParseTask(task string) m.Task {
	task = strings.TrimSpace(task)
	var taskType m.ResourceType
	taskTypeStr := task[:3]
	switch taskTypeStr {
	case "IO1":
		taskType = m.IO1
	case "IO2":
		taskType = m.IO2
	case "CPU":
		taskType = m.CPU
	}
	taskTime, err := strconv.Atoi(task[4 : len(task)-1])
	if err != nil {
		panic(err)
	}

	return m.Task{ResouceType: taskType, TotalTime: taskTime}
}

func ParseProcess(id int, line string, logger *slog.Logger, clock log.GlobalTimer) *m.Process {
	line = strings.TrimSpace(line)
	tasks := strings.Split(line, ";")
	if tasks[len(tasks)-1] == "" {
		tasks = tasks[:len(tasks)-1]
	}
	slog.Debug(fmt.Sprintf("Tasks: %v\n", tasks))

	var parsedTasks = make([]m.Task, len(tasks))
	for i, task := range tasks {
		parsedTasks[i] = ParseTask(task)
	}
	process := m.NewProcess(id, calcArrivalTime(id), parsedTasks, logger, clock)
	return process
}

func ParseProcesses(r io.Reader, logger *slog.Logger, clock log.GlobalTimer) []*m.Process {
	scanner := bufio.NewScanner(r)
	var processes = make([]*m.Process, 0)
	var i int
	for scanner.Scan() {
		process := ParseProcess(i, scanner.Text(), logger, clock)
		i++
		processes = append(processes, process)
	}
	return processes
}

func snapshotState(w io.Writer, row string) {
	fmt.Fprintf(w, "%s\n", row)
}

func printProcsStats(w io.Writer, procs []*m.Process) {
	fmt.Fprintf(w, "Process\tArrival\tService\tWaiting\tFinish time\tTurnaround (Tr)\tTr/Ts\n")
	for _, proc := range procs {
		stats := proc.GetStats()
		normalizedTurnaround := float64(stats.TurnaroundTime) / float64(stats.ServiceTime)
		fmt.Fprintf(w, "%d\t%d\t%d\t%d\t%d\t%d\t%f\n", stats.ProcId+1, stats.EntranceTime, stats.ServiceTime, stats.ReadyOrBlockedTime, stats.ExitTime, stats.TurnaroundTime, normalizedTurnaround)
	}
}

func getScheduler(schedAlgo string, procQueue *m.ProcQueue, cpuCount int) (m.Evictor, m.SelectionFunction) {
	switch schedAlgo {
	case "fcfs":
		return m.NewNonPreemptive(), m.NewSelectionFIFO()
	case "rr1":
		return m.NewRoundRobinEvictor(1), m.NewSelectionFIFO()
	case "rr4":
		return m.NewRoundRobinEvictor(4), m.NewSelectionFIFO()
	case "rr":
		return m.NewRoundRobinEvictor(*roundRobinQuantum), m.NewSelectionFIFO()
	case "spn":
		return m.NewNonPreemptive(), m.NewSelectionSPN()
	case "srt":
		srt := m.NewSchedulerSRT(procQueue, cpuCount)
		return srt, srt
	case "hrrn":
		return m.NewNonPreemptive(), m.NewSelectionHRRN()
	default:
		panic(fmt.Sprintf("Unknown scheduling algorithm %s", schedAlgo))
	}
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		panic(fmt.Sprintf("Unknown log level %s", level))
	}
}

func main() {
	flag.Parse()
	var input io.Reader

	if *inputFile != "" {
		f, err := os.Open(*inputFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		input = f
	} else {
		input = os.Stdin
	}

	var output io.Writer

	output, err := os.Create(*outputFile)
	if err != nil {
		panic(err)
	}

	defer output.(*os.File).Close()
	snapshotFunc := func(state m.DumpState) {
		snapshotState(output, fmt.Sprintf("%3s %s %s %s", state.Tick, strings.Join(state.CpusState, " "), state.Io1State, state.Io2State))
	}
	var f *excelize.File
	if *exportXlsx != "" {
		f = xlsx.GetF(*exportXlsx, *schedAlgo)
		colors := xlsx.GenerateStyles(f)
		snapshotFunc = func(state m.DumpState) {
			snapshotState(output, fmt.Sprintf("%3s %s %s %s", state.Tick, strings.Join(state.CpusState, " "), state.Io1State, state.Io2State))
			xlsx.SnapshotStateXlsx(f, *schedAlgo, state.Tick, state.CpusState, state.Io1State, state.Io2State, colors, *cpuCount)
		}
	}

	clock := &m.Clock{CurrentTick: 0}

	logLevel := parseLogLevel(*logLevel)
	defaultHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	logger := slog.New(log.NewTickLoggerHandler(defaultHandler, clock))
	processes := ParseProcesses(input, logger, clock)

	logger.Info(fmt.Sprintf("Running with %d CPUs", *cpuCount))
	logger.Info(fmt.Sprintf("Total processes: %d", len(processes)))

	// IO is always fcfs
	fcfs := m.NewNonPreemptive()

	fifoSelection := m.NewSelectionFIFO()

	cpuProcQueue := m.NewProcQueue("CPUs", clock)
	evictor, selectionFunc := getScheduler(*schedAlgo, cpuProcQueue, *cpuCount)

	io1ProcQueue := m.NewProcQueue("IO1", clock)
	io2ProcQueue := m.NewProcQueue("IO2", clock)

	io1Scheduler := m.NewSchedulerWrapper("IO1", io2ProcQueue, fifoSelection, fcfs, m.NewResource("IO1", m.IO1), clock, logger)
	io2Scheduler := m.NewSchedulerWrapper("IO2", io1ProcQueue, fifoSelection, fcfs, m.NewResource("IO2", m.IO2), clock, logger)
	cpuScheduler := m.NewSchedulerWrapper("CPUs", cpuProcQueue, selectionFunc, evictor, m.NewCpuPool(*cpuCount), clock, logger)

	// Run scheduler
	machine := m.NewMachine(cpuScheduler, io1Scheduler, io2Scheduler, clock, logger, snapshotFunc, *cpuCount)

	machine.Run(processes)

	procStatsFile, err := os.Create(*procStatsFile)
	if err != nil {
		panic(err)
	}

	defer procStatsFile.Close()
	printProcsStats(procStatsFile, processes)
	if *exportXlsx != "" {
		xlsx.PrintProcsStats(f, *schedAlgo, processes, 1+*cpuCount+2+1)
		xlsx.SaveReport(f, *exportXlsx)
	}
}
