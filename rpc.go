package informer

import (
	"github.com/roadrunner-server/api/v4/plugins/v1/jobs"
	"github.com/roadrunner-server/sdk/v4/state/process"
)

type rpc struct {
	srv *Plugin
}

// WorkerList contains list of workers.
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

func (rpc *rpc) Jobs(plugin string, out *[]*jobs.State) error {
	*out = rpc.srv.Jobs(plugin)

	return nil
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
