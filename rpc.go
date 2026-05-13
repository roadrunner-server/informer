package informer

import (
	"context"
	stderr "errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	informerV1 "github.com/roadrunner-server/api-go/v6/informer/v1"
)

// jobsTimeout caps how long a single GetJobs request waits for the underlying
// driver's JobsState call.
const jobsTimeout = time.Minute

var (
	errNoSuchPlugin       = stderr.New("no such plugin")
	errNoWorkerManagement = stderr.New("plugin does not support workers management")
)

type rpc struct {
	plugin *Plugin
}

func (r *rpc) ListPlugins(_ context.Context, _ *connect.Request[informerV1.ListPluginsRequest]) (*connect.Response[informerV1.PluginsList], error) {
	plugins := make([]string, 0, len(r.plugin.withWorkers))
	for name := range r.plugin.withWorkers {
		plugins = append(plugins, name)
	}
	return connect.NewResponse(&informerV1.PluginsList{Plugins: plugins}), nil
}

func (r *rpc) GetWorkers(_ context.Context, req *connect.Request[informerV1.GetWorkersRequest]) (*connect.Response[informerV1.WorkersList], error) {
	name := req.Msg.GetPlugin()
	svc, ok := r.plugin.withWorkers[name]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%w: %s", errNoSuchPlugin, name))
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
	return connect.NewResponse(&informerV1.WorkersList{Workers: workers}), nil
}

func (r *rpc) GetJobs(ctx context.Context, req *connect.Request[informerV1.GetJobsRequest]) (*connect.Response[informerV1.JobsList], error) {
	name := req.Msg.GetPlugin()
	svc, ok := r.plugin.withJobs[name]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%w: %s", errNoSuchPlugin, name))
	}

	jobsCtx, cancel := context.WithTimeoutCause(ctx, jobsTimeout, stderr.New("JOBS operation canceled, timeout reached (1m)"))
	defer cancel()

	states, err := svc.JobsState(jobsCtx)
	if err != nil {
		code := connect.CodeInternal
		switch {
		case stderr.Is(err, context.Canceled):
			code = connect.CodeCanceled
		case stderr.Is(err, context.DeadlineExceeded):
			code = connect.CodeDeadlineExceeded
		}
		return nil, connect.NewError(code, err)
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
	return connect.NewResponse(&informerV1.JobsList{States: jobStates}), nil
}

func (r *rpc) AddWorker(_ context.Context, req *connect.Request[informerV1.AddWorkerRequest]) (*connect.Response[informerV1.Response], error) {
	name := req.Msg.GetPlugin()
	mgr, ok := r.plugin.workersManager[name]
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%w: %s", errNoWorkerManagement, name))
	}
	if err := mgr.AddWorker(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&informerV1.Response{Ok: true}), nil
}

func (r *rpc) RemoveWorker(ctx context.Context, req *connect.Request[informerV1.RemoveWorkerRequest]) (*connect.Response[informerV1.Response], error) {
	name := req.Msg.GetPlugin()
	mgr, ok := r.plugin.workersManager[name]
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%w: %s", errNoWorkerManagement, name))
	}
	if err := mgr.RemoveWorker(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&informerV1.Response{Ok: true}), nil
}
