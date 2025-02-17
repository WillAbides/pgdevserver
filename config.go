package pgdevserver

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
)

type Config struct {
	// PostgresVersion is the version of the postgres binaries to use. Default is "17.2.0"
	PostgresVersion string `json:"postgres_version,omitempty"`

	// CacheDir is the directory containing the cache. Default is pgdevserver under the xdg cache directory
	CacheDir string `json:"-"`

	// Name is a way to distinguish between multiple servers that otherwise have the same configuration.
	// Default is "default".
	Name string `json:"name,omitempty"`

	// PostgresOptions are additional options to pass to postgres on startup.
	PostgresOptions []string `json:"postgres_options,omitempty"`

	// InitDBArgs are additional arguments to pass to initdb when creating the cluster.
	InitDBArgs []string `json:"init_db_args,omitempty"`

	// Port is the port to use for the cluster. If empty, a random port will be selected.
	Port string `json:"port,omitempty"`

	// PGManager is the PGManager to use for installing postgres. If nil, a default PGManager will be used.
	PGManager *PGManager
}

func (c Config) clone() Config {
	clone := c
	clone.PostgresOptions = slices.Clone(c.PostgresOptions)
	clone.InitDBArgs = slices.Clone(c.InitDBArgs)
	return clone
}

func (c Config) cacheKey() string {
	const keyWidth = 10
	h := sha256.New()
	for _, kv := range [][2]string{
		{"Name", c.Name},
		{"Port", c.Port},
		{"InitDBArgs", strings.Join(c.InitDBArgs, "\x00")},
		{"Postgres", c.PostgresVersion},
		{"PostgresOptions", strings.Join(c.PostgresOptions, "\x00")},
	} {
		h.Write([]byte(kv[0]))
		h.Write([]byte{0})
		h.Write([]byte(kv[1]))
	}
	return fmt.Sprintf("%s-%x", c.Name, h.Sum(nil)[:keyWidth])
}
