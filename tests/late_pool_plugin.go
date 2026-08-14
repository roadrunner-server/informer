package tests

import (
	"context"
	"sync"

	"github.com/roadrunner-server/pool/v2/state/process"
)

// Plugin2 has no pool until the test calls CreatePool, which is how a plugin
// that allocates its workers after the container is up looks to the informer.
type Plugin2 struct {
	mu sync.Mutex

	server Server
	pool   Pool
}

func (p2 *Plugin2) Init(server Server) error {
	p2.server = server

	return nil
}

func (p2 *Plugin2) Serve() chan error {
	return make(chan error, 1)
}

func (p2 *Plugin2) Stop(ctx context.Context) error {
	p2.mu.Lock()
	defer p2.mu.Unlock()

	if p2.pool != nil {
		p2.pool.Destroy(ctx)
		p2.pool = nil
	}

	return nil
}

func (p2 *Plugin2) Name() string {
	return "informer.plugin2"
}

// CreatePool allocates the pool whose workers the plugin then reports.
func (p2 *Plugin2) CreatePool(ctx context.Context) error {
	p, err := p2.server.NewPool(ctx, poolConfig(), nil, nil)
	if err != nil {
		return err
	}

	p2.mu.Lock()
	p2.pool = p
	p2.mu.Unlock()

	return nil
}

func (p2 *Plugin2) Workers() []*process.State {
	p2.mu.Lock()
	defer p2.mu.Unlock()

	return workerStates(p2.pool)
}
