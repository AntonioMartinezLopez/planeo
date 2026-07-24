package postgres

import (
	"context"
	"testing"

	"go.uber.org/goleak"
)

// Client.Close must stop libs/db's background ping goroutine, not just close
// the pool - otherwise that goroutine keeps pinging an already-closed pool
// every 20s and panics after 5 consecutive failures. In integration test
// binaries that create many Clients across many test functions, a leaked
// goroutine from an earlier, already-cleaned-up test keeps running in the
// background and can panic mid-suite once a slow CI run gives it enough
// wall-clock time to hit that threshold.
func TestClientCloseStopsBackgroundPingGoroutine(t *testing.T) {
	defer goleak.VerifyNone(t)

	// pgxpool.New does not dial eagerly, so this succeeds without a real
	// Postgres server - only the background ping goroutine's lifecycle is
	// under test here.
	client := NewClient(context.Background(), "postgres://fake:fake@127.0.0.1:1/fake?sslmode=disable")
	client.Close()
}
