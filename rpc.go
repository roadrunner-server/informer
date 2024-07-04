package informer

import (
	"github.com/roadrunner-server/api/v4/plugins/v4/jobs"
	"github.com/roadrunner-server/pool/state/process"
)

type rpc struct {
	plugin *Plugin
}

// WorkerList contains a list of workers.
type WorkerList struct {
	// Workers are a list of workers.
	Workers []*process.State `json:"workers"`
}

// List all plugins with workers.
func (rpc *rpc) List(_ bool, list *[]string) error {
	*list = make([]string, 0, len(rpc.plugin.withWorkers))

	// append all plugin names to the output result
	for name := range rpc.plugin.withWorkers {
		*list = append(*list, name)
	}

	return nil
}

// Workers state of a given service.
func (rpc *rpc) Workers(service string, list *WorkerList) error {
	workers := rpc.plugin.Workers(service)
	if workers == nil {
		return nil
	}

	// write actual processes
	list.Workers = workers

	return nil
}

func (rpc *rpc) Jobs(plugin string, out *[]*jobs.State) error {
	*out = rpc.plugin.Jobs(plugin)

	return nil
}

func (rpc *rpc) AddWorker(plugin string, _ *bool) error {
	return rpc.plugin.AddWorker(plugin)
}

func (rpc *rpc) RemoveWorker(plugin string, _ *bool) error {
	return rpc.plugin.RemoveWorker(plugin)
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
