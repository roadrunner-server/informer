// Package helpers boots RoadRunner containers for the informer e2e tests.
package helpers

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/roadrunner-server/config/v6"
	"github.com/roadrunner-server/endure/v2"
	"github.com/roadrunner-server/logger/v6"
	"github.com/stretchr/testify/require"
)

const (
	// defaultConfigVersion is the config schema version the test configs are written against.
	defaultConfigVersion = "2024.1.0"
	// probeTimeout caps how long Start waits for the probe to answer.
	probeTimeout = time.Second * 15
	probeTick    = time.Millisecond * 20
	probeDial    = time.Second
)

// bootCfg holds the options applied to a container before it is started.
type bootCfg struct {
	probe func(ctx context.Context) bool
}

// Option customizes the container built by Start.
type Option func(*bootCfg)

// WithRPCProbe makes Start return only once addr accepts a connection, which is
// the point the rpc plugin can answer calls.
func WithRPCProbe(addr string) Option {
	return func(b *bootCfg) {
		b.probe = func(ctx context.Context) bool {
			d := net.Dialer{Timeout: probeDial}
			conn, err := d.DialContext(ctx, "tcp", addr)
			if err != nil {
				return false
			}

			_ = conn.Close()

			return true
		}
	}
}

// Start registers the plugins, boots the container and waits for the probe, if
// any, to answer. Errors arriving on the container channel are reported through
// t.Errorf and stop the container, but they do not abort the test.
//
// The returned stop is idempotent and also registered with t.Cleanup, so a test
// can shut the container down in the middle of its body.
func Start(t *testing.T, cfgPath string, plugins []any, opts ...Option) func() {
	t.Helper()

	bc := &bootCfg{}
	for _, o := range opts {
		o(bc)
	}

	all := make([]any, 0, len(plugins)+2)
	all = append(all, &config.Plugin{Version: defaultConfigVersion, Path: cfgPath}, &logger.Plugin{})
	all = append(all, plugins...)

	cont := endure.New(slog.LevelDebug)
	require.NoError(t, cont.RegisterAll(all...))
	require.NoError(t, cont.Init())

	ch, err := cont.Serve()
	require.NoError(t, err)

	stopCont := sync.OnceValue(cont.Stop)
	done := make(chan struct{})
	wg := &sync.WaitGroup{}

	wg.Go(func() {
		for {
			select {
			case res := <-ch:
				if res == nil {
					return
				}

				t.Errorf("plugin %s reported an error: %v", res.VertexID, res.Error)

				if errS := stopCont(); errS != nil {
					t.Errorf("container stop: %v", errS)
				}
			case <-done:
				if errS := stopCont(); errS != nil {
					t.Errorf("container stop: %v", errS)
				}

				return
			}
		}
	})

	// The drain goroutine calls t.Errorf, so it has to be joined while the test
	// is still running.
	stop := sync.OnceFunc(func() {
		close(done)
		wg.Wait()
	})
	t.Cleanup(stop)

	if bc.probe != nil {
		require.Eventually(t, func() bool { return bc.probe(t.Context()) }, probeTimeout, probeTick, "the container did not become ready")
	}

	return stop
}
