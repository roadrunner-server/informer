package tests

import (
	"context"
	"log/slog"
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/roadrunner-server/config/v6"
	"github.com/roadrunner-server/endure/v2"
	goridgeRpc "github.com/roadrunner-server/goridge/v4/pkg/rpc"
	httpPlugin "github.com/roadrunner-server/http/v6"
	"github.com/roadrunner-server/informer/v6"
	"github.com/roadrunner-server/logger/v6"
	"github.com/roadrunner-server/pool/v2/state/process"
	"github.com/roadrunner-server/resetter/v6"
	rpcPlugin "github.com/roadrunner-server/rpc/v6"
	"github.com/roadrunner-server/server/v6"
	"github.com/roadrunner-server/status/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// workerList mirrors the payload the RPC clients decode.
type workerList struct {
	Workers []process.State `json:"workers"`
}

func newInformerClient(t *testing.T) *rpc.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", "127.0.0.1:6001")
	require.NoError(t, err)
	return rpc.NewClientWithCodec(goridgeRpc.NewClientCodec(conn))
}

func TestInformerInit(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	cfg := &config.Plugin{
		Version: "2024.1.0",
		Path:    "configs/.rr-informer.yaml",
	}

	err := cont.RegisterAll(
		cfg,
		&server.Plugin{},
		&logger.Plugin{},
		&informer.Plugin{},
		&rpcPlugin.Plugin{},
		&Plugin1{},
	)
	assert.NoError(t, err)

	err = cont.Init()
	if err != nil {
		t.Fatal(err)
	}

	ch, err := cont.Serve()
	assert.NoError(t, err)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	stopCh := make(chan struct{}, 1)

	wg := &sync.WaitGroup{}

	wg.Go(func() {
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
				return
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	})

	time.Sleep(time.Second)
	t.Run("InformerWorkersRpcTest", informerWorkersRPCTest("informer.plugin1"))
	t.Run("InformerListRpcTest", informerListRPCTest)
	t.Run("InformerPluginWithoutWorkersRpcTest", informerPluginWOWorkersRPCTest)

	stopCh <- struct{}{}
	wg.Wait()
}

func TestInformerEarlyCall(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	cfg := &config.Plugin{
		Version: "2024.1.0",
		Path:    "configs/.rr-informer-early-call.yaml",
	}

	err := cont.RegisterAll(
		cfg,
		&server.Plugin{},
		&logger.Plugin{},
		&httpPlugin.Plugin{},
		&informer.Plugin{},
		&resetter.Plugin{},
		&status.Plugin{},
		&rpcPlugin.Plugin{},
		&Plugin2{},
	)

	require.NoError(t, err)

	err = cont.Init()
	if err != nil {
		t.Fatal(err)
	}

	ch, err := cont.Serve()
	require.NoError(t, err)

	client := newInformerClient(t)
	defer func() { _ = client.Close() }()
	list := &workerList{}
	err = client.Call("informer.Workers", "informer.plugin2", list)
	require.NoError(t, err)
	require.Empty(t, list.Workers)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	stopCh := make(chan struct{}, 1)

	wg := &sync.WaitGroup{}

	wg.Go(func() {
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
				return
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	})

	time.Sleep(time.Second)
	stopCh <- struct{}{}
	wg.Wait()
}

func informerPluginWOWorkersRPCTest(t *testing.T) {
	client := newInformerClient(t)
	defer func() { _ = client.Close() }()
	list := &workerList{}
	err := client.Call("informer.Workers", "informer.config", list)
	assert.NoError(t, err)
	assert.Empty(t, list.Workers)
}

func informerWorkersRPCTest(service string) func(t *testing.T) {
	return func(t *testing.T) {
		client := newInformerClient(t)
		defer func() { _ = client.Close() }()
		list := &workerList{}
		err := client.Call("informer.Workers", service, list)
		assert.NoError(t, err)
		assert.Len(t, list.Workers, 10)
	}
}

func informerListRPCTest(t *testing.T) {
	client := newInformerClient(t)
	defer func() { _ = client.Close() }()
	list := make([]string, 0, 5)
	err := client.Call("informer.List", true, &list)
	assert.NoError(t, err)
	assert.ElementsMatch(t, list, []string{"informer.plugin1"})
}
