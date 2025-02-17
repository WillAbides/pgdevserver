package pgtestserver

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"

	"github.com/adrg/xdg"
	"github.com/willabides/pgtestserver/internal/bdcache"
)

const defaultPostgresVersion = "17.2.0"

type Server struct {
	config   Config
	cache    bdcache.Cache
	initOnce sync.Once
}

func New(cfg Config) *Server {
	s := Server{config: cfg}
	s.init()
	return &s
}

func (s *Server) Status(ctx context.Context) (Status, error) {
	s.init()
	var status Status
	err := s.withCacheLock(ctx, func(cacheDir string) error {
		var err error
		status, err = s.status(ctx, cacheDir)
		return err
	})
	if err != nil {
		return 0, err
	}
	return status, nil
}

func (s *Server) Start(ctx context.Context) error {
	s.init()
	return s.withCacheLock(ctx, func(cacheDir string) error {
		return s.start(ctx, cacheDir)
	})
}

// ConnectionURL returns the current connection URL of this server.
// When using dynamic ports, the ConnectionURL could change each time the server is started from a stopped state.
func (s *Server) ConnectionURL(ctx context.Context) (string, error) {
	s.init()
	port, err := s.getPort(ctx)
	if err != nil {
		return "", fmt.Errorf("getting port: %w", err)
	}
	return fmt.Sprintf("postgresql://postgres@localhost:%s", port), nil
}

// Logfile returns the path to the log file for the server.
// The log file is created the first time the server is started.
func (s *Server) Logfile(ctx context.Context) (string, error) {
	s.init()
	var logFile string
	err := s.withCacheLock(ctx, func(cacheDir string) error {
		logFile = logfilePath(cacheDir)
		return nil
	})
	if err != nil {
		return "", err
	}
	return logFile, nil
}

func (s *Server) init() {
	s.initOnce.Do(func() {
		s.config.PostgresVersion = cmp.Or(s.config.PostgresVersion, defaultPostgresVersion)
		s.config.Name = cmp.Or(s.config.Name, "default")
		s.config.CacheDir = cmp.Or(s.config.CacheDir, filepath.Join(xdg.CacheHome, "pgtestserver"))
		s.config.InitDBArgs = slices.Clone(s.config.InitDBArgs)
		s.config.PostgresOptions = slices.Clone(s.config.PostgresOptions)
		s.cache = bdcache.Cache{Root: filepath.Join(s.config.CacheDir, "server")}
		if s.config.PGManager == nil {
			s.config.PGManager = NewPGManager(PGMConfig{
				CacheDir: filepath.Join(s.config.CacheDir, "postgres"),
			})
		}
	})
}

// Config returns the configuration of the server after it has been initialized with defaults.
func (s *Server) Config() Config {
	s.init()
	return s.config.clone()
}

// ID returns a unique identifier for the server within the cache.
func (s *Server) ID() string {
	s.init()
	return s.Config().cacheKey()
}

func (s *Server) withCacheLock(ctx context.Context, fn func(cacheDir string) error) (errOut error) {
	populator := func(cacheDir string) error { return s.populateCache(ctx, cacheDir) }
	cacheDir, unlock, err := s.cache.Dir(s.config.cacheKey(), validateServerCache, populator)
	if err != nil {
		return err
	}
	defer func() { errOut = errors.Join(errOut, unlock()) }()
	return fn(cacheDir)
}

func (s *Server) status(ctx context.Context, cacheDir string) (_ Status, errOut error) {
	binDir, unlock, err := s.config.PGManager.Bin(ctx, s.config.PostgresVersion)
	if err != nil {
		return 0, err
	}
	defer func() { errOut = errors.Join(errOut, unlock()) }()
	pgCtl := filepath.Join(binDir, "pg_ctl")
	dataDir := filepath.Join(cacheDir, "data")
	cmd := exec.CommandContext(ctx, pgCtl,
		"status",
		"--silent",
		"-D", dataDir,
	)
	err = execRun(cmd)
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return StatusRunning, nil
	case errors.As(err, &exitErr):
		if exitErr.ExitCode() == 3 {
			return StatusStopped, nil
		}
		return StatusInvalid, nil
	default:
		return 0, err
	}
}

func (s *Server) start(ctx context.Context, cacheDir string) (errOut error) {
	dataDir := filepath.Join(cacheDir, "data")
	status, err := s.status(ctx, cacheDir)
	if err != nil {
		return err
	}
	switch status {
	case StatusRunning:
		return nil
	case StatusStopped:
	default:
		return errors.New("cluster is in an invalid state")
	}
	port, err := getTcpPortFromFile(cacheDir)
	if err != nil {
		return fmt.Errorf("getting port: %w", err)
	}
	logfile := logfilePath(cacheDir)
	err = os.MkdirAll(filepath.Dir(logfile), 0o700)
	if err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}
	args := []string{
		"start",
		"--silent",
		"--pgdata", dataDir,
		"--options", fmt.Sprintf("-p %s", port),
		"--log", logfile,
	}
	for _, o := range s.config.PostgresOptions {
		args = append(args, "--option", o)
	}
	binDir, unlock, err := s.config.PGManager.Bin(ctx, s.config.PostgresVersion)
	if err != nil {
		return err
	}
	defer func() { errOut = errors.Join(errOut, unlock()) }()
	pgCtl := filepath.Join(binDir, "pg_ctl")
	cmd := exec.CommandContext(ctx, pgCtl, args...)
	err = execRun(cmd)
	if err != nil {
		return fmt.Errorf("running pg_ctl start: %w", err)
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.init()
	return s.withCacheLock(ctx, func(cacheDir string) error {
		return s.stop(ctx, cacheDir)
	})
}

func (s *Server) stop(ctx context.Context, cacheDir string) (errOut error) {
	dataDir := filepath.Join(cacheDir, "data")
	status, err := s.status(ctx, cacheDir)
	if err != nil {
		return err
	}
	if status == StatusStopped {
		return nil
	}
	binDir, unlock, err := s.config.PGManager.Bin(ctx, s.config.PostgresVersion)
	if err != nil {
		return err
	}
	defer func() { errOut = errors.Join(errOut, unlock()) }()
	pgCtl := filepath.Join(binDir, "pg_ctl")
	cmd := exec.CommandContext(ctx, pgCtl,
		"stop",
		"--silent",
		"-D", dataDir,
	)
	err = execRun(cmd)
	if err != nil {
		return fmt.Errorf("running pg_ctl stop: %w", err)
	}
	return nil
}

func (s *Server) getPort(ctx context.Context) (string, error) {
	if s.config.Port != "" {
		return s.config.Port, nil
	}
	var port string
	err := s.withCacheLock(ctx, func(cacheDir string) error {
		var err error
		port, err = getTcpPortFromFile(cacheDir)
		return err
	})
	if err != nil {
		return "", err
	}
	return port, nil
}

func (s *Server) writeConfigFile(cacheDir string) (errOut error) {
	configFile := configJSONPath(cacheDir)
	err := os.MkdirAll(filepath.Dir(configFile), 0o700)
	if err != nil {
		return err
	}
	f, err := os.Create(configFile)
	if err != nil {
		return err
	}
	defer func() { errOut = errors.Join(errOut, f.Close()) }()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(s.config)
}

func (s *Server) populateCache(ctx context.Context, cacheDir string) (errOut error) {
	dataDir := filepath.Join(cacheDir, "data")
	err := s.writeConfigFile(cacheDir)
	if err != nil {
		return err
	}
	var args []string
	args = appendFlagArg(args, "--pgdata", dataDir)
	args = appendFlagArg(args, "--username", "postgres")
	args = append(args, s.config.InitDBArgs...)
	binDir, unlock, err := s.config.PGManager.Bin(ctx, s.config.PostgresVersion)
	if err != nil {
		return err
	}
	defer func() { errOut = errors.Join(errOut, unlock()) }()
	initdb := filepath.Join(binDir, "initdb")
	cmd := exec.CommandContext(ctx, initdb, args...)
	err = execRun(cmd)
	if err != nil {
		return fmt.Errorf("running initdb: %w", err)
	}
	return nil
}

func validateServerCache(cacheDir string) error {
	_, err := os.Stat(filepath.Join(cacheDir, "data", "PG_VERSION"))
	return err
}
