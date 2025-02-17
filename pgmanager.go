package pgtestserver

import (
	"cmp"
	"context"
	"embed"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/adrg/xdg"
	"github.com/willabides/pgtestserver/internal"
	"github.com/willabides/pgtestserver/internal/bdcache"
)

//go:generate go run ./internal/writeknownversions knownversions

//go:embed knownversions/*
var knownVersionsFS embed.FS

const defaultMavenURL = "https://repo1.maven.org/maven2"

// knownSystemVersions returns the known versions of embedded postgres binaries for the given system
// without querying maven.
func knownSystemVersions(system string) []string {
	system = strings.ReplaceAll(system, "/", "_")
	b, err := fs.ReadFile(knownVersionsFS, path.Join("knownversions", system+".txt"))
	if err != nil {
		return nil
	}
	s := strings.TrimSpace(string(b))
	return strings.Split(s, "\n")
}

func pgCacheKey(version string) string {
	return "v" + strings.ReplaceAll(version, ".", "_")
}

func versionFile(cacheDir string) string {
	return filepath.Join(cacheDir, "version.txt")
}

type PGMConfig struct {
	// MavenURL is the base URL for maven repositories. Default is https://repo1.maven.org/maven2.
	MavenURL string

	// CacheDir is the directory containing the cache. Default is pgm under the xdg cache directory
	CacheDir string

	// HTTPClient is the http client to use for downloading files.
	HTTPClient *http.Client
}

type PGManager struct {
	config PGMConfig
	cache  bdcache.Cache

	initOnce sync.Once
}

func NewPGManager(cfg PGMConfig) *PGManager {
	return &PGManager{config: cfg}
}

func (m *PGManager) init() {
	m.initOnce.Do(func() {
		m.config.MavenURL = cmp.Or(m.config.MavenURL, defaultMavenURL)
		m.config.CacheDir = cmp.Or(m.config.CacheDir, filepath.Join(xdg.CacheHome, "pgm"))
		m.cache = bdcache.Cache{Root: m.config.CacheDir}
		if m.config.HTTPClient == nil {
			m.config.HTTPClient = &http.Client{Timeout: time.Minute}
		}
	})
}

// AvailableVersions returns a list of available versions of postgres
func (m *PGManager) AvailableVersions(ctx context.Context) ([]string, error) {
	m.init()
	system := runtime.GOOS + "/" + runtime.GOARCH
	versions, err := m.availableVersions(ctx, m.config.MavenURL, system)
	if err != nil {
		return nil, err
	}

	// make sure old darwin/amd64 versions are available on darwin/arm64
	if system != "darwin/arm64" {
		return versions, nil
	}
	extraVersions, err := m.availableVersions(ctx, m.config.MavenURL, "darwin/amd64")
	if err != nil {
		return nil, err
	}
	versions = append(versions, extraVersions...)
	internal.SortVersions(versions)
	return slices.Compact(versions), nil
}

// InstalledVersions returns a list of installed postgres versions
func (m *PGManager) InstalledVersions() ([]string, error) {
	m.init()
	var versions []string
	err := m.cache.Walk(func(dir string) error {
		filename := versionFile(dir)
		b, err := os.ReadFile(filename)
		switch {
		case errors.Is(err, os.ErrNotExist):
			return nil
		case err != nil:
			return err
		}
		versions = append(versions, strings.TrimSpace(string(b)))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return versions, nil
}

// rlockVersion obtains a read lock on the given version of postgres.
func (m *PGManager) rlockVersion(ctx context.Context, version string) (dir string, unlock func() error, _ error) {
	_, err := semver.NewVersion(version)
	if err != nil {
		return "", nil, fmt.Errorf("invalid version: %w", err)
	}
	populator := func(cacheDir string) error {
		return m.pgmPopulateCache(ctx, cacheDir, m.config.MavenURL, version)
	}
	return m.cache.Dir(pgCacheKey(version), pgmValidateCache, populator)
}

// Install assures that the given version of postgres is installed
func (m *PGManager) Install(ctx context.Context, version string) error {
	m.init()
	_, unlock, err := m.rlockVersion(ctx, version)
	if err != nil {
		return err
	}
	return unlock()
}

// Bin obtains a read lock for the given version and returns a path to the bin directory.
// The caller must call the returned unlock function when done with the bin directory.
// Use this to run pg_ctl, initdb, etc.
func (m *PGManager) Bin(ctx context.Context, version string) (binDir string, unlock func() error, _ error) {
	m.init()
	cacheDir, unlock, err := m.rlockVersion(ctx, version)
	if err != nil {
		return "", nil, err
	}
	return filepath.Join(cacheDir, "bin"), unlock, nil
}

func pgmValidateCache(cacheDir string) error {
	_, err := os.Stat(filepath.Join(cacheDir, "bin", "pg_ctl"))
	return err
}

func (m *PGManager) pgmPopulateCache(ctx context.Context, cacheDir, mavenURL, version string) error {
	jarBytes, err := m.download(ctx, mavenURL, version)
	if err != nil {
		return err
	}
	err = extractJar(ctx, cacheDir, jarBytes)
	if err != nil {
		return err
	}
	return os.WriteFile(versionFile(cacheDir), []byte(version+"\n"), 0o600)
}

// availableMavenVersions queries maven metadata for available versions of a maven artifact.
func (m *PGManager) availableMavenVersions(
	ctx context.Context,
	mavenURL, groupID, artifactID string,
) (_ []string, errOut error) {
	u := fmt.Sprintf("%s/%s/%s/maven-metadata.xml", mavenURL, groupID, artifactID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := m.config.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { errOut = errors.Join(errOut, resp.Body.Close()) }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected http status code %d", resp.StatusCode)
	}
	var metadata struct {
		Versioning struct {
			Versions []string `xml:"versions>version"`
		} `xml:"versioning"`
	}
	err = xml.NewDecoder(resp.Body).Decode(&metadata)
	if err != nil {
		return nil, err
	}
	return metadata.Versioning.Versions, nil
}

// availableVersions returns the available versions for the given system.
func (m *PGManager) availableVersions(ctx context.Context, mavenURL, system string) ([]string, error) {
	if !slices.Contains(internal.SupportedSystems, system) {
		return nil, fmt.Errorf("system %s not supported", system)
	}
	versions, err := m.availableMavenVersions(ctx, mavenURL, internal.ZonkyGroupID, internal.SystemArtifactID(system))
	if err != nil {
		return nil, err
	}
	versions = internal.FilterVersions(versions)
	internal.SortVersions(versions)
	return versions, nil
}

// getArtifactID returns the maven artifact id for the given system and version.
func (m *PGManager) getArtifactID(ctx context.Context, mavenURL, system, version string) (string, error) {
	if slices.Contains(knownSystemVersions(system), version) {
		return internal.SystemArtifactID(system), nil
	}

	// check for versions newer than the last build
	versions, err := m.availableVersions(ctx, mavenURL, system)
	if err != nil {
		return "", err
	}
	if slices.Contains(versions, version) {
		return internal.SystemArtifactID(system), nil
	}

	// assume rosetta2 is available amd darwin/arm64 can run darwin/amd64 binaries in a pinch
	if system == "darwin/arm64" {
		return m.getArtifactID(ctx, mavenURL, "darwin/amd64", version)
	}
	return "", fmt.Errorf("version %s not found for system %s", version, system)
}
