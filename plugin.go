package informer

import (
	"context"

	"github.com/roadrunner-server/api/v3/plugins/v1/jobs"
	"github.com/roadrunner-server/endure/v2/dep"
	"github.com/roadrunner-server/sdk/v4/state/process"
)

const PluginName = "informer"

// Informer used to get workers from particular plugin or set of plugins
type Informer interface {
	Workers() []*process.State
	// Name return user-friendly name of the plugin
	Name() string
}

// JobsStat interface provide statistic for the jobs plugin
type JobsStat interface {
	// JobsState returns slice with the attached drivers information
	JobsState(ctx context.Context) ([]*jobs.State, error)
	// Name return user-friendly name of the plugin
	Name() string
}

type Plugin struct {
	withJobs    map[string]JobsStat
	withWorkers map[string]Informer
}

func (p *Plugin) Init() error {
	p.withWorkers = make(map[string]Informer)
	p.withJobs = make(map[string]JobsStat)

	return nil
}

// Workers provides BaseProcess slice with workers for the requested plugin
func (p *Plugin) Workers(name string) []*process.State {
	svc, ok := p.withWorkers[name]
	if !ok {
		return nil
	}

	return svc.Workers()
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

// Collects declares services to be collected.
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
	}
}

// Name of the service.
func (p *Plugin) Name() string {
	return PluginName
}

// RPC returns associated rpc service.
func (p *Plugin) RPC() any {
	return &rpc{srv: p}
}
