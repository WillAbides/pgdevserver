package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/willabides/pgdevserver"
)

type pgCmd struct {
	List      pgListCmd      `kong:"cmd,help='List installed postgres versions.'"`
	Available pgAvailableCmd `kong:"cmd,help='List postgres versions available to download.'"`
}

type pgListCmd struct {
	CacheParams cacheParams `kong:"embed"`
}

func (c *pgListCmd) Run() (errOut error) {
	mgr := pgdevserver.NewPGManager(pgdevserver.PGMConfig{
		CacheDir: filepath.Join(c.CacheParams.cacheDir(), "postgres"),
	})
	versions, err := mgr.InstalledVersions()
	if err != nil {
		return err
	}
	for _, version := range versions {
		fmt.Println(version)
	}
	return nil
}

type pgAvailableCmd struct {
	CacheParams cacheParams `kong:"embed"`
}

func (c *pgAvailableCmd) Run() error {
	ctx := context.Background()
	mgr := pgdevserver.NewPGManager(pgdevserver.PGMConfig{
		CacheDir: filepath.Join(c.CacheParams.cacheDir(), "postgres"),
	})
	versions, err := mgr.AvailableVersions(ctx)
	if err != nil {
		return err
	}
	for _, version := range versions {
		fmt.Println(version)
	}
	return nil
}
