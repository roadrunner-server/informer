package informer

import (
	"context"

	endure "github.com/roadrunner-server/endure/pkg/container"
	"github.com/roadrunner-server/sdk/v3/plugins/jobs"
	"github.com/roadrunner-server/sdk/v3/state/process"
)

const PluginName = "informer"

// Informer used to get workers from particular plugin or set of plugins
type Informer interface {
	Workers() []*process.State
}

// JobsStat interface provide statistic for the jobs plugin
type JobsStat interface {
	// JobsState returns slice with the attached drivers information
	JobsState(ctx context.Context) ([]*jobs.State, error)
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
func (p *Plugin) Collects() []any {
	return []any{
		p.CollectWorkers,
		p.CollectJobs,
	}
}

// CollectWorkers obtains plugins with workers inside.
func (p *Plugin) CollectWorkers(name endure.Named, r Informer) {
	p.withWorkers[name.Name()] = r
}

func (p *Plugin) CollectJobs(name endure.Named, j JobsStat) {
	p.withJobs[name.Name()] = j
}

// Name of the service.
func (p *Plugin) Name() string {
	return PluginName
}

// RPC returns associated rpc service.
func (p *Plugin) RPC() any {
	return &rpc{srv: p}
}
