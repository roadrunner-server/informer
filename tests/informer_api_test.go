package tests

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	informerV1 "github.com/roadrunner-server/api-go/v6/informer/v1"
	"github.com/roadrunner-server/api-go/v6/informer/v1/informerV1connect"
	"github.com/roadrunner-server/config/v6"
	"github.com/roadrunner-server/endure/v2"
	"github.com/roadrunner-server/informer/v6"
	"github.com/roadrunner-server/logger/v6"
	rpcPlugin "github.com/roadrunner-server/rpc/v6"
	"github.com/roadrunner-server/server/v6"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const informerAPIAddr = "127.0.0.1:6001"

// startInformerAPIContainer brings up rpc + server + logger + informer +
// Plugin1. Plugin1 satisfies Informer (Name + Workers) but not WorkerManager
// or JobsStat, which lets us exercise both the success branches
// (ListPlugins, GetWorkers, GetJobs) and the error branch (AddWorker /
// RemoveWorker against a plugin that does not support worker management).
func startInformerAPIContainer(t *testing.T) func() {
	t.Helper()

	cont := endure.New(slog.LevelError)
	cfg := &config.Plugin{
		Version: "2024.2.0",
		Path:    "configs/.rr-informer-api.yaml",
	}

	require.NoError(t, cont.RegisterAll(
		cfg,
		&logger.Plugin{},
		&server.Plugin{},
		&rpcPlugin.Plugin{},
		&informer.Plugin{},
		&Plugin1{},
	))
	require.NoError(t, cont.Init())

	ch, err := cont.Serve()
	require.NoError(t, err)

	wg := &sync.WaitGroup{}
	stop := make(chan struct{})
	wg.Go(func() {
		select {
		case e := <-ch:
			t.Errorf("container reported error: %v", e.Error)
		case <-stop:
		}
	})

	time.Sleep(500 * time.Millisecond)

	return func() {
		close(stop)
		require.NoError(t, cont.Stop())
		wg.Wait()
	}
}

func TestInformerConnectAPI(t *testing.T) {
	stop := startInformerAPIContainer(t)
	defer stop()

	httpc := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				return (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, network, addr)
			},
		},
	}
	client := informerV1connect.NewInformerServiceClient(httpc, "http://"+informerAPIAddr)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	listResp, err := client.ListPlugins(ctx, connect.NewRequest(&informerV1.ListPluginsRequest{}))
	require.NoError(t, err)
	require.Contains(t, listResp.Msg.GetPlugins(), "informer.plugin1")

	workersResp, err := client.GetWorkers(ctx, connect.NewRequest(&informerV1.GetWorkersRequest{Plugin: "informer.plugin1"}))
	require.NoError(t, err)
	require.NotEmpty(t, workersResp.Msg.GetWorkers())

	// Plugin1 isn't a JobsStat, so it's unknown to the jobs map and
	// surfaces as CodeNotFound.
	_, err = client.GetJobs(ctx, connect.NewRequest(&informerV1.GetJobsRequest{Plugin: "informer.plugin1"}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	// Unknown plugin → CodeNotFound on the read-path RPCs.
	_, err = client.GetWorkers(ctx, connect.NewRequest(&informerV1.GetWorkersRequest{Plugin: "does-not-exist"}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	// Plugin1 doesn't implement WorkerManager — AddWorker/RemoveWorker
	// surface "does not support workers management" as CodeFailedPrecondition
	// (caller-input class, not internal).
	_, err = client.AddWorker(ctx, connect.NewRequest(&informerV1.AddWorkerRequest{Plugin: "informer.plugin1"}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	_, err = client.RemoveWorker(ctx, connect.NewRequest(&informerV1.RemoveWorkerRequest{Plugin: "informer.plugin1"}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// TestInformerHTTPApi exercises the RPCs through plain HTTP/1.1 with a
// protojson body — the shape any non-Connect HTTP client uses against this
// handler.
func TestInformerHTTPApi(t *testing.T) {
	stop := startInformerAPIContainer(t)
	defer stop()

	httpc := &http.Client{Timeout: 30 * time.Second}
	ctx := t.Context()

	call := func(method string, in proto.Message, out proto.Message) {
		body, err := protojson.Marshal(in)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			"http://"+informerAPIAddr+"/informer.v1.InformerService/"+method, bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpc.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		respBody, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equalf(t, http.StatusOK, resp.StatusCode, "method=%s body=%s", method, respBody)
		require.NoError(t, protojson.Unmarshal(respBody, out))
	}

	var listResp informerV1.PluginsList
	call("ListPlugins", &informerV1.ListPluginsRequest{}, &listResp)
	require.Contains(t, listResp.GetPlugins(), "informer.plugin1")

	var workersResp informerV1.WorkersList
	call("GetWorkers", &informerV1.GetWorkersRequest{Plugin: "informer.plugin1"}, &workersResp)
	require.NotEmpty(t, workersResp.GetWorkers())
}

// TestInformerGRPCApi exercises the RPCs through a regular gRPC client. The
// same Connect handler serves gRPC framing off the same port.
func TestInformerGRPCApi(t *testing.T) {
	stop := startInformerAPIContainer(t)
	defer stop()

	conn, err := grpc.NewClient(informerAPIAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := informerV1.NewInformerServiceClient(conn)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	listResp, err := client.ListPlugins(ctx, &informerV1.ListPluginsRequest{})
	require.NoError(t, err)
	require.Contains(t, listResp.GetPlugins(), "informer.plugin1")

	workersResp, err := client.GetWorkers(ctx, &informerV1.GetWorkersRequest{Plugin: "informer.plugin1"})
	require.NoError(t, err)
	require.NotEmpty(t, workersResp.GetWorkers())
}

// TestInformerHTTPGetIdempotency verifies that the three read-only methods
// (ListPlugins, GetWorkers, GetJobs) accept HTTP GET — marked NO_SIDE_EFFECTS
// in the proto — while the mutating methods (AddWorker, RemoveWorker) return
// 405 Method Not Allowed.
func TestInformerHTTPGetIdempotency(t *testing.T) {
	stop := startInformerAPIContainer(t)
	defer stop()

	body, err := protojson.Marshal(&informerV1.GetWorkersRequest{Plugin: "probe"})
	require.NoError(t, err)

	q := url.Values{}
	q.Set("encoding", "json")
	q.Set("base64", "1")
	q.Set("message", base64.URLEncoding.EncodeToString(body))

	cases := []struct {
		method      string
		expectAllow bool
	}{
		{"ListPlugins", true},
		{"GetWorkers", true},
		{"GetJobs", true},
		{"AddWorker", false},
		{"RemoveWorker", false},
	}

	httpc := &http.Client{Timeout: 30 * time.Second}
	for _, c := range cases {
		t.Run(c.method, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
				"http://"+informerAPIAddr+"/informer.v1.InformerService/"+c.method+"?"+q.Encode(), nil)
			require.NoError(t, err)

			resp, err := httpc.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			respBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if c.expectAllow {
				require.NotEqualf(t, http.StatusMethodNotAllowed, resp.StatusCode,
					"%s via GET should be allowed; got 405\n%s", c.method, respBody)
				return
			}
			require.Equalf(t, http.StatusMethodNotAllowed, resp.StatusCode,
				"%s via GET should be rejected; got %s\n%s", c.method, resp.Status, respBody)
		})
	}
}
