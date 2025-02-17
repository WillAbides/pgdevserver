package pgtestserver

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

const testCacheDir = "tmp/test-cache"

func TestServer(t *testing.T) {
	t.Run("lifecycle", func(t *testing.T) {
		ctx := context.Background()
		cfg := Config{
			PostgresVersion: "17.1.0",
			CacheDir:        filepath.Join(testCacheDir, "TestServer", "lifecycle"),
		}
		srv := New(cfg)
		err := srv.Start(ctx)
		require.NoError(t, err)
		status, err := srv.Status(ctx)
		require.NoError(t, err)
		require.Equal(t, StatusRunning, status)
		u, err := srv.ConnectionURL(ctx)
		require.NoError(t, err)
		fmt.Println(u)
		conn, err := pgx.Connect(ctx, u)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, conn.Close(ctx)) })
		err = conn.Ping(ctx)
		require.NoError(t, err)
		err = srv.Stop(ctx)
		require.NoError(t, err)
		status, err = srv.Status(ctx)
		require.NoError(t, err)
		require.Equal(t, StatusStopped, status)
	})

	// run this whole thing with no existing cache
	t.Run("uncached", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping in short mode")
		}
		ctx := context.Background()
		cfg := Config{
			PostgresVersion: "17.1.0",
			CacheDir:        filepath.Join(t.TempDir()),
		}
		srv := New(cfg)
		err := srv.Start(ctx)
		require.NoError(t, err)
		status, err := srv.Status(ctx)
		require.NoError(t, err)
		require.Equal(t, StatusRunning, status)
		u, err := srv.ConnectionURL(ctx)
		require.NoError(t, err)
		conn, err := pgx.Connect(ctx, u)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, conn.Close(ctx)) })
		err = conn.Ping(ctx)
		require.NoError(t, err)
		err = srv.Stop(ctx)
		require.NoError(t, err)
		status, err = srv.Status(ctx)
		require.NoError(t, err)
		require.Equal(t, StatusStopped, status)
	})
}
