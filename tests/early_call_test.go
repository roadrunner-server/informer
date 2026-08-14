package tests

import (
	"testing"

	"tests/helpers"

	"github.com/roadrunner-server/informer/v6"
	rpcPlugin "github.com/roadrunner-server/rpc/v6"
	"github.com/roadrunner-server/server/v6"
	"github.com/stretchr/testify/require"
)

// earlyCallRPC is the rpc listener of configs/.rr-informer-early-call.yaml.
const earlyCallRPC = "127.0.0.1:6332"

// TestInformerEarlyCall queries a plugin whose pool does not exist yet: the call
// has to answer with an empty list instead of failing or blocking. The container
// runs without a readiness probe, so the first call lands the moment Serve
// returns; the rpc plugin binds its listener inside Serve, which is what makes
// the dial below reliable.
func TestInformerEarlyCall(t *testing.T) {
	latePool := &Plugin2{}

	helpers.Start(t, "configs/.rr-informer-early-call.yaml", []any{
		&server.Plugin{},
		&informer.Plugin{},
		&rpcPlugin.Plugin{},
		latePool,
	})

	client := helpers.RPC(t, earlyCallRPC)

	var list helpers.WorkersList
	require.NoError(t, client.Call("informer.Workers", "informer.plugin2", &list))
	require.Empty(t, list.Workers)

	require.NoError(t, latePool.CreatePool(t.Context()))

	list = helpers.WorkersList{}
	require.NoError(t, client.Call("informer.Workers", "informer.plugin2", &list))
	require.Len(t, list.Workers, poolWorkers)
}
