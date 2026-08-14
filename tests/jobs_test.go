package tests

import (
	"testing"

	"tests/helpers"

	"github.com/roadrunner-server/api-plugins/v6/jobs"
	"github.com/roadrunner-server/informer/v6"
	rpcPlugin "github.com/roadrunner-server/rpc/v6"
	"github.com/stretchr/testify/require"
)

// jobsRPC is the rpc listener of configs/.rr-informer-jobs.yaml.
const jobsRPC = "127.0.0.1:6333"

// TestInformerJobs checks that the jobs state survives the goridge codec on the
// way out of the plugin.
func TestInformerJobs(t *testing.T) {
	helpers.Start(t, "configs/.rr-informer-jobs.yaml", []any{
		&informer.Plugin{},
		&rpcPlugin.Plugin{},
		&Plugin3{},
	}, helpers.WithRPCProbe(jobsRPC))

	client := helpers.RPC(t, jobsRPC)

	t.Run("PluginWithJobs", func(t *testing.T) {
		var states []*jobs.State
		require.NoError(t, client.Call("informer.Jobs", "informer.plugin3", &states))
		require.Len(t, states, 1)

		require.Equal(t, jobsPipeline, states[0].Pipeline)
		require.Equal(t, jobsDriver, states[0].Driver)
		require.Equal(t, jobsQueue, states[0].Queue)
		require.Equal(t, int64(3), states[0].Active)
		require.Equal(t, int64(2), states[0].Delayed)
		require.Equal(t, int64(1), states[0].Reserved)
		require.True(t, states[0].Ready)
	})

	t.Run("UnregisteredPlugin", func(t *testing.T) {
		var states []*jobs.State
		require.NoError(t, client.Call("informer.Jobs", "informer.unregistered", &states))
		require.Empty(t, states)
	})
}
