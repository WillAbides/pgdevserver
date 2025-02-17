package main

import (
	"cmp"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/alecthomas/kong"
	"github.com/willabides/pgtestserver"
)

const (
	recommendedOptions = `-c 'shared_buffers=128MB' \
-c 'fsync=off' -c 'synchronous_commit=off' \
-c 'full_page_writes=off' \
-c 'max_connections=100' \
-c 'client_min_messages=warning'`
)

var help = kong.Vars{
	"serverNameHelp": "A name to distinguish this server from others that have the same configuration.",
	"cacheHelp":      "Cache for binaries and server data. Defaults to $XDG_CACHE_HOME/pgtestserver.",
	"initDBArgsHelp": "Extra arguments to pass to initdb. May be specified multiple times.",
	"postgresHelp":   "Postgres version.",
	"portHelp":       "Port to listen on. When left empty, a random port will be chosen.",
	"optionHelp":     "Extra options to pass to postgres. May be specified multiple times.",
}

type serverParams struct {
	ID              string   `kong:"help='Act on the server with this ID. When set, other server options are ignored.'"`
	PostgresVersion string   `kong:"name='pg',default='17.2.0',help=${postgresHelp}"`
	ServerName      string   `kong:"default='default',help=${serverNameHelp}"`
	InitDBArgs      []string `kong:"help=${initDBArgsHelp},placeholder='arg'"`
	Port            string   `kong:"help=${portHelp}"`
	PGOptions       []string `kong:"name='option',short='o',help='Extra options to pass to postgres. May be specified multiple times.',placeholder='option'"`
	Recommended     bool     `kong:"name='recommended',help='Use recommended options'"`
}

func (p *serverParams) server(rootCache string) (*pgtestserver.Server, error) {
	if p.ID != "" {
		return pgtestserver.ServerFromCache(rootCache, p.ID)
	}
	pgOptions := p.PGOptions
	if p.Recommended {
		pgOptions = append([]string{recommendedOptions}, pgOptions...)
	}
	return pgtestserver.New(pgtestserver.Config{
		PostgresVersion: p.PostgresVersion,
		CacheDir:        rootCache,
		Name:            p.ServerName,
		InitDBArgs:      p.InitDBArgs,
		Port:            p.Port,
		PostgresOptions: pgOptions,
	}), nil
}

type rootCmd struct {
	ServerCmds serverCmds `kong:"embed"`
	Pg         pgCmd      `kong:"cmd,help='Manage postgres binaries'"`
}

type cacheParams struct {
	Cache string `kong:"help=${cacheHelp}"`
}

func (p cacheParams) cacheDir() string {
	return cmp.Or(p.Cache, filepath.Join(xdg.CacheHome, "pgtestserver"))
}

func main() {
	cli := kong.Parse(&rootCmd{}, help)
	cli.FatalIfErrorf(cli.Run())
}
