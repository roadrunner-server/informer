package tests

import (
	"testing"

	"tests/helpers"

	"github.com/roadrunner-server/informer/v6"
	rpcPlugin "github.com/roadrunner-server/rpc/v6"
	"github.com/roadrunner-server/server/v6"
	"github.com/stretchr/testify/require"
)

// workersRPC is the rpc listener of configs/.rr-informer.yaml.
const workersRPC = "127.0.0.1:6331"

func TestInformerWorkers(t *testing.T) {
	helpers.Start(t, "configs/.rr-informer.yaml", []any{
		&server.Plugin{},
		&informer.Plugin{},
		&rpcPlugin.Plugin{},
		&Plugin1{},
		&Plugin3{},
	}, helpers.WithRPCProbe(workersRPC))

	client := helpers.RPC(t, workersRPC)

	t.Run("PluginWithPool", func(t *testing.T) {
		var list helpers.WorkersList
		require.NoError(t, client.Call("informer.Workers", "informer.plugin1", &list))
		require.Len(t, list.Workers, poolWorkers)

		for _, w := range list.Workers {
			require.NotZero(t, w.Pid)
		}
	})

	t.Run("UnregisteredPlugin", func(t *testing.T) {
		var list helpers.WorkersList
		require.NoError(t, client.Call("informer.Workers", "informer.unregistered", &list))
		require.Empty(t, list.Workers)
	})

	t.Run("List", func(t *testing.T) {
		var plugins []string
		require.NoError(t, client.Call("informer.List", true, &plugins))
		// Plugin3 reports jobs and manages workers, but has no pool to report
		require.ElementsMatch(t, []string{"informer.plugin1"}, plugins)
	})
}
