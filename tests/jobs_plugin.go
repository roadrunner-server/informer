package tests

import (
	"context"
	"sync"

	"github.com/roadrunner-server/api-plugins/v6/jobs"
)

// The pipeline Plugin3 reports, mirrored by the assertions in the jobs e2e.
const (
	jobsPipeline = "test-pipeline"
	jobsDriver   = "memory"
	jobsQueue    = "test-queue"
)

// Plugin3 serves a fixed jobs state and records the worker management calls it
// receives, all from memory: it needs neither a pool nor PHP.
type Plugin3 struct {
	mu          sync.Mutex
	addCalls    int
	removeCalls int
}

func (p3 *Plugin3) Init() error {
	return nil
}

func (p3 *Plugin3) Name() string {
	return "informer.plugin3"
}

func (p3 *Plugin3) JobsState(context.Context) ([]*jobs.State, error) {
	return []*jobs.State{
		{
			Pipeline: jobsPipeline,
			Driver:   jobsDriver,
			Queue:    jobsQueue,
			Active:   3,
			Delayed:  2,
			Reserved: 1,
			Ready:    true,
			Priority: 10,
		},
	}, nil
}

func (p3 *Plugin3) AddWorker() error {
	p3.mu.Lock()
	defer p3.mu.Unlock()

	p3.addCalls++

	return nil
}

func (p3 *Plugin3) RemoveWorker(context.Context) error {
	p3.mu.Lock()
	defer p3.mu.Unlock()

	p3.removeCalls++

	return nil
}

// AddCalls returns how many times AddWorker reached the plugin.
func (p3 *Plugin3) AddCalls() int {
	p3.mu.Lock()
	defer p3.mu.Unlock()

	return p3.addCalls
}

// RemoveCalls returns how many times RemoveWorker reached the plugin.
func (p3 *Plugin3) RemoveCalls() int {
	p3.mu.Lock()
	defer p3.mu.Unlock()

	return p3.removeCalls
}
