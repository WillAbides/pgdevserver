package pgdevserver

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/willabides/pgdevserver/internal/bdcache"
)

// getTcpPortFromFile gets the port from a file in the cache directory. If the file does not exist, it creates the file
// and writes an available port to it.
func getTcpPortFromFile(cacheDir string) (string, error) {
	configDir := filepath.Join(cacheDir, "config")
	portFile := filepath.Join(configDir, "tcp_port")
	b, err := os.ReadFile(portFile)
	switch {
	case err == nil:
		return strings.TrimSpace(string(b)), nil
	case errors.Is(err, os.ErrNotExist):
		// handled below
	default:
		return "", err
	}
	port, err := availableTcpPort("")
	if err != nil {
		return "", err
	}
	err = os.MkdirAll(configDir, 0o700)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(portFile, []byte(port), 0o600)
	if err != nil {
		return "", err
	}
	return port, nil
}

func logfilePath(cacheDir string) string {
	return filepath.Join(cacheDir, "log", "server.log")
}

func configJSONPath(cacheDir string) string {
	return filepath.Join(cacheDir, "config", "config.json")
}

// ServersFromCache returns all the servers in the cache.
func ServersFromCache(cacheDir string) ([]*Server, error) {
	serverCache := bdcache.Cache{Root: filepath.Join(cacheDir, "server")}
	var servers []*Server
	err := serverCache.Walk(func(serverCacheDir string) error {
		server, err := serverFromCacheDir(cacheDir, serverCacheDir)
		if err != nil {
			return err
		}
		servers = append(servers, server)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return servers, nil
}

// ServerFromCache returns a server from the cache by ID.
func ServerFromCache(rootCache, id string) (_ *Server, errOut error) {
	serverCache := bdcache.Cache{Root: filepath.Join(rootCache, "server")}
	dir, unlock, err := serverCache.Dir(id, nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { errOut = errors.Join(errOut, unlock()) }()
	return serverFromCacheDir(rootCache, dir)
}

func serverFromCacheDir(rootCache, serverCacheDir string) (*Server, error) {
	configJSON, err := os.ReadFile(configJSONPath(serverCacheDir))
	if err != nil {
		return nil, err
	}
	var config Config
	err = json.Unmarshal(configJSON, &config)
	if err != nil {
		return nil, err
	}
	config.CacheDir = rootCache
	return New(config), nil
}
