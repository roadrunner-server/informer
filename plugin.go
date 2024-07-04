package informer

import (
	"context"
	"time"

	"github.com/roadrunner-server/api/v4/plugins/v4/jobs"
	"github.com/roadrunner-server/endure/v2/dep"
	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/pool/state/process"
)

const PluginName = "informer"

type Named interface {
	// Name returns the user-friendly name of the plugin
	Name() string
}

type WorkerManager interface {
	Named
	// RemoveWorker removes worker from the pool.
	RemoveWorker(ctx context.Context) error
	// AddWorker adds worker to the pool.
	AddWorker() error
}

// Informer used to get workers from a particular plugin or set of plugins
type Informer interface {
	Named
	Workers() []*process.State
}

// JobsStat interface provides statistic for the job plugin
type JobsStat interface {
	Named
	// JobsState returns a slice with the attached drivers information
	JobsState(ctx context.Context) ([]*jobs.State, error)
}

type Plugin struct {
	workersManager map[string]WorkerManager
	withJobs       map[string]JobsStat
	withWorkers    map[string]Informer
}

func (p *Plugin) Init() error {
	p.withWorkers = make(map[string]Informer)
	p.workersManager = make(map[string]WorkerManager)
	p.withJobs = make(map[string]JobsStat)

	return nil
}

// Workers provide BaseProcess slice with workers for the requested plugin
func (p *Plugin) Workers(name string) []*process.State {
	svc, ok := p.withWorkers[name]
	if !ok {
		return nil
	}

	return svc.Workers()
}

func (p *Plugin) AddWorker(plugin string) error {
	if _, ok := p.workersManager[plugin]; !ok {
		return errors.Errorf("plugin %s does not support workers management", plugin)
	}

	return p.workersManager[plugin].AddWorker()
}

func (p *Plugin) RemoveWorker(plugin string) error {
	if _, ok := p.workersManager[plugin]; !ok {
		return errors.Errorf("plugin %s does not support workers management", plugin)
	}

	return p.workersManager[plugin].RemoveWorker(context.Background())
}

// Jobs provides information about jobs for the registered plugin using jobs
func (p *Plugin) Jobs(name string) []*jobs.State {
	svc, ok := p.withJobs[name]
	if !ok {
		return nil
	}

	ctx, cancel := context.WithTimeoutCause(context.Background(), time.Minute, errors.Str("JOBS operation canceled, timeout reached (1m)"))
	st, err := svc.JobsState(ctx)
	if err != nil {
		cancel()
		// skip errors here
		return nil
	}

	cancel()
	return st
}

// Collects declare services to be collected.
func (p *Plugin) Collects() []*dep.In {
	return []*dep.In{
		dep.Fits(func(pl any) {
			j := pl.(JobsStat)
			p.withJobs[j.Name()] = j
		}, (*JobsStat)(nil)),
		dep.Fits(func(pl any) {
			r := pl.(Informer)
			p.withWorkers[r.Name()] = r
		}, (*Informer)(nil)),
		dep.Fits(func(pl any) {
			r := pl.(WorkerManager)
			p.workersManager[r.Name()] = r
		}, (*WorkerManager)(nil)),
	}
}

// Name of the service.
func (p *Plugin) Name() string {
	return PluginName
}

// RPC returns associated rpc service.
func (p *Plugin) RPC() any {
	return &rpc{
		plugin: p,
	}
}
