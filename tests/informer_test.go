package tests

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"connectrpc.com/connect"
	informerV1 "github.com/roadrunner-server/api-go/v6/informer/v1"
	"github.com/roadrunner-server/api-go/v6/informer/v1/informerV1connect"
	"github.com/roadrunner-server/config/v6"
	"github.com/roadrunner-server/endure/v2"
	httpPlugin "github.com/roadrunner-server/http/v6"
	"github.com/roadrunner-server/informer/v6"
	"github.com/roadrunner-server/logger/v6"
	"github.com/roadrunner-server/resetter/v6"
	rpcPlugin "github.com/roadrunner-server/rpc/v6"
	"github.com/roadrunner-server/server/v6"
	"github.com/roadrunner-server/status/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

func newInformerClient(t *testing.T) informerV1connect.InformerServiceClient {
	t.Helper()
	httpc := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				return (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, network, addr)
			},
		},
	}
	return informerV1connect.NewInformerServiceClient(httpc, "http://127.0.0.1:6001")
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
	wg.Add(1)

	go func() {
		defer wg.Done()
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
	}()

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
	listResp, err := client.GetWorkers(t.Context(), connect.NewRequest(&informerV1.GetWorkersRequest{Plugin: "informer.plugin2"}))
	require.NoError(t, err)
	require.Empty(t, listResp.Msg.GetWorkers())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	stopCh := make(chan struct{}, 1)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
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
	}()

	time.Sleep(time.Second)
	stopCh <- struct{}{}
	wg.Wait()
}

func informerPluginWOWorkersRPCTest(t *testing.T) {
	client := newInformerClient(t)
	// "informer.config" is not a registered Informer in this container — the
	// handler must surface this as CodeNotFound rather than silently returning
	// an empty list (which is indistinguishable from a registered plugin
	// with zero workers).
	_, err := client.GetWorkers(t.Context(), connect.NewRequest(&informerV1.GetWorkersRequest{Plugin: "informer.config"}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func informerWorkersRPCTest(service string) func(t *testing.T) {
	return func(t *testing.T) {
		client := newInformerClient(t)
		resp, err := client.GetWorkers(t.Context(), connect.NewRequest(&informerV1.GetWorkersRequest{Plugin: service}))
		assert.NoError(t, err)
		assert.Len(t, resp.Msg.GetWorkers(), 10)
	}
}

func informerListRPCTest(t *testing.T) {
	client := newInformerClient(t)
	resp, err := client.ListPlugins(t.Context(), connect.NewRequest(&informerV1.ListPluginsRequest{}))
	assert.NoError(t, err)
	assert.ElementsMatch(t, resp.Msg.GetPlugins(), []string{"informer.plugin1"})
}
