package informer

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	informerV1 "github.com/roadrunner-server/api-go/v6/informer/v1"
)

// jobsTimeout caps how long a single GetJobs request waits for the underlying
// driver's JobsState call.
const jobsTimeout = time.Minute

var (
	errNoSuchPlugin       = errors.New("no such plugin")
	errNoWorkerManagement = errors.New("plugin does not support workers management")
)

type rpc struct {
	plugin *Plugin
}

func (r *rpc) ListPlugins(_ *informerV1.ListPluginsRequest, out *informerV1.PluginsList) error {
	out.Plugins = slices.Collect(maps.Keys(r.plugin.withWorkers))
	return nil
}

func noSuchPlugin(name string) error {
	return fmt.Errorf("%w: %s", errNoSuchPlugin, name)
}

func noWorkerManagement(name string) error {
	return fmt.Errorf("%w: %s", errNoWorkerManagement, name)
}

func (r *rpc) GetWorkers(in *informerV1.GetWorkersRequest, out *informerV1.WorkersList) error {
	name := in.GetPlugin()
	svc, ok := r.plugin.withWorkers[name]
	if !ok {
		return noSuchPlugin(name)
	}

	states := svc.Workers()
	workers := make([]*informerV1.ProcessState, 0, len(states))
	for _, s := range states {
		workers = append(workers, &informerV1.ProcessState{
			Pid:         int32(s.Pid), //nolint:gosec
			Status:      s.Status,
			NumExecs:    s.NumExecs,
			Created:     s.Created,
			MemoryUsage: s.MemoryUsage,
			CpuPercent:  float32(s.CPUPercent),
			Command:     s.Command,
			StatusStr:   s.StatusStr,
		})
	}
	out.Workers = workers
	return nil
}

func (r *rpc) GetJobs(in *informerV1.GetJobsRequest, out *informerV1.JobsList) error {
	name := in.GetPlugin()
	svc, ok := r.plugin.withJobs[name]
	if !ok {
		return noSuchPlugin(name)
	}

	jobsCtx, cancel := context.WithTimeoutCause(context.Background(), jobsTimeout, fmt.Errorf("JOBS operation canceled, timeout reached (%s)", jobsTimeout))
	defer cancel()

	states, err := svc.JobsState(jobsCtx)
	if err != nil {
		return err
	}

	jobStates := make([]*informerV1.JobState, 0, len(states))
	for _, s := range states {
		jobStates = append(jobStates, &informerV1.JobState{
			Pipeline:     s.Pipeline,
			Driver:       s.Driver,
			Queue:        s.Queue,
			Active:       s.Active,
			Delayed:      s.Delayed,
			Reserved:     s.Reserved,
			Ready:        s.Ready,
			Priority:     s.Priority,
			ErrorMessage: s.ErrorMessage,
		})
	}
	out.States = jobStates
	return nil
}

func (r *rpc) AddWorker(in *informerV1.AddWorkerRequest, out *informerV1.Response) error {
	name := in.GetPlugin()
	mgr, ok := r.plugin.workersManager[name]
	if !ok {
		return noWorkerManagement(name)
	}
	if err := mgr.AddWorker(); err != nil {
		return err
	}
	out.Ok = true
	return nil
}

func (r *rpc) RemoveWorker(in *informerV1.RemoveWorkerRequest, out *informerV1.Response) error {
	name := in.GetPlugin()
	mgr, ok := r.plugin.workersManager[name]
	if !ok {
		return noWorkerManagement(name)
	}
	if err := mgr.RemoveWorker(context.Background()); err != nil {
		return err
	}
	out.Ok = true
	return nil
}
