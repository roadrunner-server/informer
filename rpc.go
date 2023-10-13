package informer

import (
	"context"

	"github.com/roadrunner-server/api/v4/plugins/v3/jobs"
	"github.com/roadrunner-server/sdk/v4/state/process"
)

type rpc struct {
	srv *Plugin
}

// WorkerList contains a list of workers.
type WorkerList struct {
	// Workers are list of workers.
	Workers []*process.State `json:"workers"`
}

// List all plugins with workers.
func (rpc *rpc) List(_ bool, list *[]string) error {
	*list = make([]string, 0, len(rpc.srv.withWorkers))

	// append all plugin names to the output result
	for name := range rpc.srv.withWorkers {
		*list = append(*list, name)
	}

	return nil
}

// Workers state of a given service.
func (rpc *rpc) Workers(service string, list *WorkerList) error {
	workers := rpc.srv.Workers(service)
	if workers == nil {
		return nil
	}

	// write actual processes
	list.Workers = workers

	return nil
}

// Jobs provides information about jobs for the registered plugin using jobs
func (p *Plugin) Jobs(name string) []*jobs.State {
	svc, ok := p.withJobs[name]
	if !ok {
		return nil
	}

	st, err := svc.JobsState(context.Background())
	if err != nil {
		// skip errors here
		return nil
	}

	return st
}

func (rpc *rpc) AddWorker(plugin string, _ *bool) error {
	return rpc.srv.AddWorker(plugin)
}

func (rpc *rpc) RemoveWorker(plugin string, _ *bool) error {
	return rpc.srv.RemoveWorker(plugin)
}

// sort.Sort

func (w *WorkerList) Len() int {
	return len(w.Workers)
}

func (w *WorkerList) Less(i, j int) bool {
	return w.Workers[i].Pid < w.Workers[j].Pid
}

func (w *WorkerList) Swap(i, j int) {
	w.Workers[i], w.Workers[j] = w.Workers[j], w.Workers[i]
}
