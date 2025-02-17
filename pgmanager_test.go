package pgtestserver

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testMgr(t testing.TB, cacheDir string) *PGManager {
	cacheDir = cmp.Or(cacheDir, filepath.Join(
		filepath.FromSlash(testCacheDir),
		strings.ReplaceAll(t.Name(), "/", "_"),
	))
	require.NoError(t, os.MkdirAll(cacheDir, 0o700))
	return NewPGManager(PGMConfig{CacheDir: cacheDir})
}

func TestManager_AvailableVersions(t *testing.T) {
	mgr := testMgr(t, t.TempDir())
	versions, err := mgr.AvailableVersions(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, versions)
}

func TestManager_InstalledVersions(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		mgr := testMgr(t, t.TempDir())
		versions, err := mgr.InstalledVersions()
		require.NoError(t, err)
		require.Empty(t, versions)
	})

	t.Run("populated", func(t *testing.T) {
		cacheDir := t.TempDir()
		mgr := testMgr(t, cacheDir)
		versions := []string{"17.1.0", "17.2.0"}
		for _, version := range versions {
			filename := versionFile(filepath.Join(cacheDir, pgCacheKey(version)))
			require.NoError(t, os.MkdirAll(filepath.Dir(filename), 0o700))
			require.NoError(t, os.WriteFile(filename, []byte(version+"\n"), 0o600))
		}
		gotVersions, err := mgr.InstalledVersions()
		require.NoError(t, err)
		require.Equal(t, versions, gotVersions)
	})

	t.Run("skips entries without version files", func(t *testing.T) {
		cacheDir := t.TempDir()
		mgr := testMgr(t, cacheDir)
		versions := []string{"17.1.0", "17.2.0"}
		for _, version := range versions {
			filename := versionFile(filepath.Join(cacheDir, pgCacheKey(version)))
			filename += ".tmp"
			require.NoError(t, os.MkdirAll(filepath.Dir(filename), 0o700))
			require.NoError(t, os.WriteFile(filename, []byte(version+"\n"), 0o600))
		}
		gotVersions, err := mgr.InstalledVersions()
		require.NoError(t, err)
		require.Empty(t, gotVersions)
	})
}

func TestManager_Install(t *testing.T) {
	t.Run("cached", func(t *testing.T) {
		mgr := testMgr(t, "")
		version := "17.1.0"
		err := mgr.Install(t.Context(), version)
		require.NoError(t, err)
		gotVersions, err := mgr.InstalledVersions()
		require.NoError(t, err)
		require.Contains(t, gotVersions, version)
	})

	t.Run("uncached", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping in short mode")
		}
		mgr := testMgr(t, t.TempDir())
		version := "17.1.0"
		err := mgr.Install(t.Context(), version)
		require.NoError(t, err)
		gotVersions, err := mgr.InstalledVersions()
		require.NoError(t, err)
		require.Contains(t, gotVersions, version)
	})
}

func TestManager_Bin(t *testing.T) {
	t.Run("cached", func(t *testing.T) {
		mgr := testMgr(t, "")
		version := "17.1.0"
		err := mgr.Install(t.Context(), version)
		require.NoError(t, err)
		bin, unlock, err := mgr.Bin(t.Context(), version)
		t.Cleanup(func() { assert.NoError(t, unlock()) })
		require.NoError(t, err)
		require.NotEmpty(t, bin)
		pg := filepath.Join(bin, "postgres")
		cmd := exec.Command(pg, "--version")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err)
		assert.Contains(t, string(out), "17.1")
		fmt.Println(string(out))
	})

	t.Run("uncached", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping in short mode")
		}
		mgr := testMgr(t, t.TempDir())
		version := "17.1.0"
		err := mgr.Install(t.Context(), version)
		require.NoError(t, err)
		bin, unlock, err := mgr.Bin(t.Context(), version)
		t.Cleanup(func() { assert.NoError(t, unlock()) })
		require.NoError(t, err)
		require.NotEmpty(t, bin)
		pg := filepath.Join(bin, "postgres")
		cmd := exec.Command(pg, "--version")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err)
		assert.Contains(t, string(out), "17.1")
		fmt.Println(string(out))
	})
}
