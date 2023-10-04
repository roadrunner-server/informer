package informer

import (
	"context"

	"github.com/roadrunner-server/endure/v2/dep"
	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v4/state/process"
)

const PluginName = "informer"

type WorkerManager interface {
	// RemoveWorker removes worker from the pool.
	RemoveWorker(ctx context.Context) error
	// AddWorker adds worker to the pool.
	AddWorker() error
	// Name returns user-friendly name of the plugin
	Name() string
}

// Informer used to get workers from a particular plugin or set of plugins
type Informer interface {
	Workers() []*process.State
	// Name returns user-friendly name of the plugin
	Name() string
}

type Plugin struct {
	workersManager map[string]WorkerManager
	withWorkers    map[string]Informer
}

func (p *Plugin) Init() error {
	p.withWorkers = make(map[string]Informer)
	p.workersManager = make(map[string]WorkerManager)

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

// Collects declare services to be collected.
func (p *Plugin) Collects() []*dep.In {
	return []*dep.In{
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
		srv: p,
	}
}
