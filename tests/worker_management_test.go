package tests

import (
	"testing"

	"tests/helpers"

	"github.com/roadrunner-server/informer/v6"
	rpcPlugin "github.com/roadrunner-server/rpc/v6"
	"github.com/stretchr/testify/require"
)

func TestInformerAddRemoveWorker(t *testing.T) {
	manager := &Plugin3{}

	helpers.Start(t, "configs/.rr-informer-jobs.yaml", []any{
		&informer.Plugin{},
		&rpcPlugin.Plugin{},
		manager,
	}, helpers.WithRPCProbe(jobsRPC))

	client := helpers.RPC(t, jobsRPC)

	var ok bool
	require.NoError(t, client.Call("informer.AddWorker", "informer.plugin3", &ok))
	require.Equal(t, 1, manager.AddCalls())

	require.NoError(t, client.Call("informer.RemoveWorker", "informer.plugin3", &ok))
	require.Equal(t, 1, manager.RemoveCalls())

	err := client.Call("informer.AddWorker", "informer.unregistered", &ok)
	require.ErrorContains(t, err, "plugin does not support workers management: informer.unregistered")

	err = client.Call("informer.RemoveWorker", "informer.unregistered", &ok)
	require.ErrorContains(t, err, "plugin does not support workers management: informer.unregistered")

	require.Equal(t, 1, manager.AddCalls())
	require.Equal(t, 1, manager.RemoveCalls())
}
