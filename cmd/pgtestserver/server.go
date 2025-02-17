package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/willabides/pgtestserver"
	"github.com/willabides/pgtestserver/internal/bdcache"
)

type serverCmds struct {
	Start startCmd    `kong:"cmd,help='Start a server.'"`
	List  listCmd     `kong:"cmd,help='List servers.'"`
	Stop  stopCmd     `kong:"cmd,help='Stop a server.'"`
	Rm    rmServerCmd `kong:"cmd,help='Remove a server.'"`
}

type listCmd struct {
	CacheParams cacheParams `kong:"embed"`
	Status      bool        `kong:"help='Show server status.'"`
	URL         bool        `kong:"help='Show server connection URL for started servers.'"`
	PG          bool        `kong:"help='Show postgres version.'"`
	NoHeaders   bool        `kong:"help='Do not show headers.'"`
}

func (c *listCmd) Run() (errOut error) {
	ctx := context.Background()
	servers, err := pgtestserver.ServersFromCache(c.CacheParams.cacheDir())
	if err != nil {
		return err
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer func() { errOut = errors.Join(errOut, tw.Flush()) }()

	// Print header if needed
	header := []string{"ID"}
	if c.PG {
		header = append(header, "Postgres")
	}
	if c.Status {
		header = append(header, "Status")
	}
	if c.URL {
		header = append(header, "URL")
	}
	if !c.NoHeaders {
		_, err = fmt.Fprintln(tw, strings.Join(header, "\t"))
		if err != nil {
			return err
		}
	}

	for _, server := range servers {
		err = c.listServer(ctx, server, tw)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *listCmd) listServer(ctx context.Context, server *pgtestserver.Server, tw *tabwriter.Writer) error {
	line := []string{server.ID()}
	var (
		status     pgtestserver.Status
		statusOnce sync.Once
	)
	getStatus := func() pgtestserver.Status {
		statusOnce.Do(func() {
			var err error
			status, err = server.Status(ctx)
			if err != nil {
				status = pgtestserver.StatusUnknown
			}
		})
		return status
	}
	if c.PG {
		line = append(line, server.Config().PostgresVersion)
	}
	if c.Status {
		line = append(line, getStatus().String())
	}
	if c.URL && getStatus() == pgtestserver.StatusRunning {
		u, err := server.ConnectionURL(ctx)
		if err != nil {
			u = "unknown"
		}
		line = append(line, u)
	}
	_, err := fmt.Fprintln(tw, strings.Join(line, "\t"))
	return err
}

type startCmd struct {
	ServerParams serverParams `kong:"embed,group='Server Options'"`
	CacheParams  cacheParams  `kong:"embed"`
}

func (c *startCmd) Run() error {
	ctx := context.Background()
	srv, err := c.ServerParams.server(c.CacheParams.cacheDir())
	if err != nil {
		return err
	}
	err = srv.Start(ctx)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			fmt.Println(string(exitErr.Stderr))
		}
		return err
	}
	pgURL, err := srv.ConnectionURL(ctx)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			fmt.Println(string(exitErr.Stderr))
		}
		return err
	}
	fmt.Println(pgURL)
	return nil
}

type stopCmd struct {
	ServerParams serverParams `kong:"embed,group='Server Options'"`
	CacheParams  cacheParams  `kong:"embed"`
}

func (c *stopCmd) Run() error {
	ctx := context.Background()
	srv, err := c.ServerParams.server(c.CacheParams.cacheDir())
	if err != nil {
		return err
	}
	return srv.Stop(ctx)
}

type rmServerCmd struct {
	ID          string      `kong:"help='ID of the server to remove.'"`
	Force       bool        `kong:"help='Remove the server even if it is running.'"`
	CacheParams cacheParams `kong:"embed"`
}

func (c rmServerCmd) Run() error {
	ctx := context.Background()
	srv, err := pgtestserver.ServerFromCache(c.CacheParams.cacheDir(), c.ID)
	if err != nil {
		return err
	}
	status, err := srv.Status(ctx)
	if err != nil {
		status = pgtestserver.StatusUnknown
	}
	if status != pgtestserver.StatusStopped && !c.Force {
		return fmt.Errorf("server %s is not stopped. Use --force to remove it anyway", c.ID)
	}
	serverCache := bdcache.Cache{Root: filepath.Join(c.CacheParams.cacheDir(), "server")}
	return serverCache.Evict(c.ID)
}
