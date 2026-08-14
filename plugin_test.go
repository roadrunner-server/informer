package informer

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/roadrunner-server/api-plugins/v6/jobs"
	"github.com/roadrunner-server/pool/v2/state/process"
	"github.com/stretchr/testify/require"
)

// collectedPlugin implements every interface the informer collects and records
// what the informer passed to it.
type collectedPlugin struct {
	name string

	workers   []*process.State
	jobsState []*jobs.State
	jobsErr   error
	addErr    error
	removeErr error

	jobsDeadline      time.Time
	addCalls          int
	removeCalls       int
	removeDeadlineSet bool
}

func (c *collectedPlugin) Name() string {
	return c.name
}

func (c *collectedPlugin) Workers() []*process.State {
	return c.workers
}

func (c *collectedPlugin) JobsState(ctx context.Context) ([]*jobs.State, error) {
	c.jobsDeadline, _ = ctx.Deadline()

	if c.jobsErr != nil {
		return nil, c.jobsErr
	}

	return c.jobsState, nil
}

func (c *collectedPlugin) AddWorker() error {
	c.addCalls++

	return c.addErr
}

func (c *collectedPlugin) RemoveWorker(ctx context.Context) error {
	c.removeCalls++
	_, c.removeDeadlineSet = ctx.Deadline()

	return c.removeErr
}

// newPlugin returns an initialized informer with no collected plugins.
func newPlugin(t *testing.T) *Plugin {
	t.Helper()

	p := &Plugin{}
	require.NoError(t, p.Init())

	return p
}

func TestPluginWorkers(t *testing.T) {
	states := []*process.State{{Pid: 42}, {Pid: 43}}

	withPool := &collectedPlugin{name: "with-pool", workers: states}
	idle := &collectedPlugin{name: "idle"}

	p := newPlugin(t)
	p.withWorkers[withPool.name] = withPool
	p.withWorkers[idle.name] = idle

	require.Equal(t, states, p.Workers(withPool.name))
	require.Nil(t, p.Workers(idle.name))
	require.Nil(t, p.Workers("unregistered"))
}

func TestPluginJobs(t *testing.T) {
	states := []*jobs.State{
		{Pipeline: "first", Driver: "memory", Queue: "default", Ready: true},
		{Pipeline: "second", Driver: "amqp", Queue: "events", Active: 7},
	}

	t.Run("RegisteredPlugin", func(t *testing.T) {
		driver := &collectedPlugin{name: "with-jobs", jobsState: states}

		p := newPlugin(t)
		p.withJobs[driver.name] = driver

		before := time.Now()
		require.Equal(t, states, p.Jobs(driver.name))
		after := time.Now()

		// the driver call carries the jobsTimeout budget
		require.WithinRange(t, driver.jobsDeadline, before.Add(jobsTimeout), after.Add(jobsTimeout))
	})

	t.Run("DriverError", func(t *testing.T) {
		driver := &collectedPlugin{name: "with-jobs", jobsErr: errors.New("driver is unreachable")}

		p := newPlugin(t)
		p.withJobs[driver.name] = driver

		// the driver error is not propagated, the caller sees no pipelines
		require.Nil(t, p.Jobs(driver.name))
	})

	t.Run("UnregisteredPlugin", func(t *testing.T) {
		p := newPlugin(t)
		require.Nil(t, p.Jobs("unregistered"))
	})
}

func TestPluginAddWorker(t *testing.T) {
	manager := &collectedPlugin{name: "managed"}

	p := newPlugin(t)
	p.workersManager[manager.name] = manager

	require.NoError(t, p.AddWorker(manager.name))
	require.Equal(t, 1, manager.addCalls)

	err := p.AddWorker("unregistered")
	require.ErrorIs(t, err, errNoWorkerManagement)
	require.ErrorContains(t, err, "unregistered")
	require.Equal(t, 1, manager.addCalls)

	manager.addErr = errors.New("the pool is at its limit")
	require.ErrorIs(t, p.AddWorker(manager.name), manager.addErr)
}

func TestPluginRemoveWorker(t *testing.T) {
	manager := &collectedPlugin{name: "managed"}

	p := newPlugin(t)
	p.workersManager[manager.name] = manager

	require.NoError(t, p.RemoveWorker(manager.name))
	require.Equal(t, 1, manager.removeCalls)

	// the manager call carries no deadline, while Jobs caps its driver call
	require.False(t, manager.removeDeadlineSet)

	err := p.RemoveWorker("unregistered")
	require.ErrorIs(t, err, errNoWorkerManagement)
	require.ErrorContains(t, err, "unregistered")
	require.Equal(t, 1, manager.removeCalls)

	manager.removeErr = errors.New("the last worker is kept")
	require.ErrorIs(t, p.RemoveWorker(manager.name), manager.removeErr)
}

func TestPluginCollects(t *testing.T) {
	p := newPlugin(t)

	collects := p.Collects()
	require.Len(t, collects, 3)

	types := make([]reflect.Type, 0, len(collects))
	for _, in := range collects {
		types = append(types, in.Type)
	}

	require.ElementsMatch(t, []reflect.Type{
		reflect.TypeOf((*JobsStat)(nil)).Elem(),
		reflect.TypeOf((*Informer)(nil)).Elem(),
		reflect.TypeOf((*WorkerManager)(nil)).Elem(),
	}, types)

	collected := &collectedPlugin{
		name:      "everything",
		workers:   []*process.State{{Pid: 7}},
		jobsState: []*jobs.State{{Pipeline: "first"}},
	}

	for _, in := range collects {
		in.Callback(collected)
	}

	// every collector routes the calls of its own kind to the collected plugin
	require.Equal(t, collected.workers, p.Workers(collected.name))
	require.Equal(t, collected.jobsState, p.Jobs(collected.name))
	require.NoError(t, p.AddWorker(collected.name))
	require.Equal(t, 1, collected.addCalls)
}

func TestPluginName(t *testing.T) {
	// the rpc plugin registers the service under this name, so it is the
	// namespace of every call the clients make, as in informer.Workers
	require.Equal(t, "informer", newPlugin(t).Name())
}
