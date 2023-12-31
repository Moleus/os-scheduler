package machine

import "fmt"

type ResourceState int

const (
	FREE ResourceState = iota
	BUSY
)

type ResourceType int

const (
	CPU ResourceType = iota
	IO1
	IO2
)

type Resourcer interface {
	GetFree() (*Resource, error)
	MustEvict(p *Process)
	GetProcs() []*Process
}

type Resource struct {
	name            string
	state           ResourceState
	resourceType    ResourceType
	currentProc     *Process
	ProcRunningTime int
}

func NewResource(name string, rType ResourceType) *Resource {
	return &Resource{name, FREE, rType, nil, 0}
}

type CpuPool struct {
	cpus []*Resource
}

func NewCpuPool(n int) *CpuPool {
	cpus := make([]*Resource, n, n)
	for i := 0; i < n; i++ {
		cpus[i] = NewResource(fmt.Sprintf("CPU%d", i+1), CPU)
	}
	return &CpuPool{cpus}
}

func (r *Resource) GetFree() (*Resource, error) {
	if r.state == BUSY {
		return nil, fmt.Errorf("Resource is busy")
	}
	return r, nil
}

func (r *Resource) AssignToFree(p *Process) error {
	if r.state == BUSY {
		return fmt.Errorf("Resource is busy")
	}
	r.state = BUSY
	r.currentProc = p
	switch r.resourceType {
	case CPU:
		p.AssignToCpu()
	case IO1, IO2:
		p.AssignToIo()
	}
	return nil
}

func (r *Resource) Tick() {
	if r.state == BUSY {
		r.ProcRunningTime++
	}
}

func (r *Resource) GetProcs() []*Process {
	if r.state == BUSY {
		return []*Process{r.currentProc}
	}
	return []*Process{}
}

func (r *Resource) MustEvict(p *Process) {
	if r.state == FREE {
		panic(fmt.Sprintf("Can't evict process. Resource is free"))
	}
	if r.currentProc.id != p.id {
		panic(fmt.Sprintf("Process %d is not running on resource", p.id))
	}
	r.state = FREE
	r.currentProc = nil
	p.onEvict()
}

func (cpu *CpuPool) GetFree() (*Resource, error) {
	for _, res := range cpu.cpus {
		if res.state == FREE {
			return res, nil
		}
	}
	return nil, fmt.Errorf("No available cpus")
}

func (cpu *CpuPool) Tick() {
	for _, res := range cpu.cpus {
		res.Tick()
	}
}

func (cpu *CpuPool) GetProcs() []*Process {
	procs := []*Process{}
	for _, res := range cpu.cpus {
		procs = append(procs, res.GetProcs()...)
	}
	return procs
}

func (cpu *CpuPool) MustEvict(p *Process) {
	for _, res := range cpu.cpus {
		if res.state == BUSY && res.currentProc.id == p.id {
			res.MustEvict(p)
			return
		}
	}
	panic(fmt.Sprintf("Process %d is not running on cpu", p.id))
}
