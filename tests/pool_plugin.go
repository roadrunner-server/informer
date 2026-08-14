package tests

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/roadrunner-server/pool/v2/pool"
	staticPool "github.com/roadrunner-server/pool/v2/pool/static_pool"
	"github.com/roadrunner-server/pool/v2/state/process"
	"github.com/roadrunner-server/pool/v2/worker"
)

// poolWorkers is the size of every pool the test plugins allocate.
const poolWorkers = 2

// Server creates workers for the application.
type Server interface {
	NewPool(ctx context.Context, cfg *pool.Config, env map[string]string, _ *slog.Logger) (*staticPool.Pool, error)
	NewWorker(ctx context.Context, env map[string]string) (*worker.Process, error)
}

// Pool is the part of the static pool the test plugins rely on.
type Pool interface {
	// Workers return a worker list associated with the pool.
	Workers() (workers []*worker.Process)
	// Destroy all underlying stacks (but let them complete the task).
	Destroy(ctx context.Context)
}

func poolConfig() *pool.Config {
	return &pool.Config{
		NumWorkers:      poolWorkers,
		MaxJobs:         100,
		AllocateTimeout: time.Second * 10,
		DestroyTimeout:  time.Second,
	}
}

// Plugin1 owns a pool allocated while the container boots, so its workers are
// reported from the moment the container serves.
type Plugin1 struct {
	mu sync.Mutex

	server Server
	pool   Pool
}

func (p1 *Plugin1) Init(server Server) error {
	p1.server = server

	return nil
}

func (p1 *Plugin1) Serve() chan error {
	errCh := make(chan error, 1)

	p, err := p1.server.NewPool(context.Background(), poolConfig(), nil, nil)
	if err != nil {
		errCh <- err

		return errCh
	}

	p1.mu.Lock()
	p1.pool = p
	p1.mu.Unlock()

	return errCh
}

func (p1 *Plugin1) Stop(ctx context.Context) error {
	p1.mu.Lock()
	defer p1.mu.Unlock()

	if p1.pool != nil {
		p1.pool.Destroy(ctx)
		p1.pool = nil
	}

	return nil
}

func (p1 *Plugin1) Name() string {
	return "informer.plugin1"
}

func (p1 *Plugin1) Workers() []*process.State {
	p1.mu.Lock()
	defer p1.mu.Unlock()

	return workerStates(p1.pool)
}

// workerStates collects the process state of every worker in the pool.
func workerStates(p Pool) []*process.State {
	if p == nil {
		return nil
	}

	workers := p.Workers()
	states := make([]*process.State, 0, len(workers))

	for i := range workers {
		state, err := process.WorkerProcessState(workers[i])
		if err != nil {
			return nil
		}

		states = append(states, state)
	}

	return states
}
