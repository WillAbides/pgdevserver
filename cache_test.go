package pgdevserver

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
	"github.com/stretchr/testify/require"
)

func TestCacheServers(t *testing.T) {
	cacheDir := filepath.Join(xdg.CacheHome, "pgdevserver")
	for i := range 3 {
		server := New(Config{
			Name: fmt.Sprintf("server%d", i),
		})
		_, err := server.ConnectionURL(t.Context())
		require.NoError(t, err)
	}
	got, err := ServersFromCache(cacheDir)
	require.NoError(t, err)
	for _, server := range got {
		cfg := server.Config()
		status, err := server.Status(t.Context())
		require.NoError(t, err)
		fmt.Println(server.ID(), cfg.PostgresVersion, status)
	}
}
