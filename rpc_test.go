package informer

import (
	"errors"
	"sort"
	"testing"

	"github.com/roadrunner-server/api-plugins/v6/jobs"
	"github.com/roadrunner-server/pool/v2/state/process"
	"github.com/stretchr/testify/require"
)

// newRPCService returns the rpc service of an initialized informer, together
// with the plugin behind it.
func newRPCService(t *testing.T) (*rpc, *Plugin) {
	t.Helper()

	p := newPlugin(t)

	svc, ok := p.RPC().(*rpc)
	require.True(t, ok, "RPC must serve the informer rpc service")

	return svc, p
}

func TestRPCList(t *testing.T) {
	svc, p := newRPCService(t)

	var names []string
	require.NoError(t, svc.List(true, &names))
	require.NotNil(t, names)
	require.Empty(t, names)

	p.withWorkers["first"] = &collectedPlugin{name: "first"}
	p.withWorkers["second"] = &collectedPlugin{name: "second"}

	require.NoError(t, svc.List(true, &names))
	require.ElementsMatch(t, []string{"first", "second"}, names)
}

func TestRPCWorkers(t *testing.T) {
	states := []*process.State{{Pid: 11}}

	svc, p := newRPCService(t)
	p.withWorkers["with-pool"] = &collectedPlugin{name: "with-pool", workers: states}

	var list WorkerList
	require.NoError(t, svc.Workers("unregistered", &list))
	require.Nil(t, list.Workers)

	require.NoError(t, svc.Workers("with-pool", &list))
	require.Equal(t, states, list.Workers)
}

func TestRPCJobs(t *testing.T) {
	states := []*jobs.State{{Pipeline: "first", Driver: "memory"}}

	svc, p := newRPCService(t)
	p.withJobs["with-jobs"] = &collectedPlugin{name: "with-jobs", jobsState: states}

	var out []*jobs.State
	require.NoError(t, svc.Jobs("with-jobs", &out))
	require.Equal(t, states, out)

	require.NoError(t, svc.Jobs("unregistered", &out))
	require.Empty(t, out)
}

func TestRPCAddWorker(t *testing.T) {
	manager := &collectedPlugin{name: "managed"}

	svc, p := newRPCService(t)
	p.workersManager[manager.name] = manager

	// the reply is left to the caller's zero value
	reply := false
	require.NoError(t, svc.AddWorker(manager.name, &reply))
	require.False(t, reply)
	require.Equal(t, 1, manager.addCalls)

	manager.addErr = errors.New("the pool is at its limit")
	require.ErrorIs(t, svc.AddWorker(manager.name, &reply), manager.addErr)

	require.ErrorIs(t, svc.AddWorker("unregistered", &reply), errNoWorkerManagement)
}

func TestRPCRemoveWorker(t *testing.T) {
	manager := &collectedPlugin{name: "managed"}

	svc, p := newRPCService(t)
	p.workersManager[manager.name] = manager

	reply := false
	require.NoError(t, svc.RemoveWorker(manager.name, &reply))
	require.False(t, reply)
	require.Equal(t, 1, manager.removeCalls)

	manager.removeErr = errors.New("the last worker is kept")
	require.ErrorIs(t, svc.RemoveWorker(manager.name, &reply), manager.removeErr)

	require.ErrorIs(t, svc.RemoveWorker("unregistered", &reply), errNoWorkerManagement)
}

func TestWorkerListSort(t *testing.T) {
	list := &WorkerList{Workers: []*process.State{{Pid: 30}, {Pid: 10}, {Pid: 20}}}

	sort.Sort(list)

	pids := make([]int64, 0, list.Len())
	for _, w := range list.Workers {
		pids = append(pids, w.Pid)
	}

	require.Equal(t, []int64{10, 20, 30}, pids)
}
