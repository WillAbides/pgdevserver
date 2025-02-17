package pgdevserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/mholt/archives"
)

const zonkyGroupID = "io/zonky/test/postgres"

// download downloads the pg jar file and returns its content.
func (m *PGManager) download(ctx context.Context, mavenURL, version string) (_ []byte, errOut error) {
	system := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	artifactID, err := m.getArtifactID(ctx, mavenURL, system, version)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf(
		"%s/%s/%s/%s/%s-%s.jar",
		mavenURL, zonkyGroupID, artifactID, version, artifactID, version,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { errOut = errors.Join(errOut, resp.Body.Close()) }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected http status code %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// extractJar double extracts the pg jar file. The jar file contains a single txz
// file which contains the actual pg binaries. extractJar extracts the txz file
// to the dest directory.
func extractJar(ctx context.Context, dest string, content []byte) (errOut error) {
	jarFS, err := archives.FileSystem(ctx, "", bytes.NewReader(content))
	if err != nil {
		return err
	}
	rootFiles, err := fs.ReadDir(jarFS, ".")
	if err != nil {
		return err
	}
	txzFiles := slices.DeleteFunc(slices.Clone(rootFiles), func(f fs.DirEntry) bool {
		name := f.Name()
		return !strings.HasSuffix(name, ".txz")
	})
	if len(txzFiles) != 1 {
		return fmt.Errorf("expected 1 txz file, got %d", len(txzFiles))
	}
	txzFilename := txzFiles[0].Name()
	txzFile, err := jarFS.Open(txzFilename)
	if err != nil {
		return err
	}
	defer func() { errOut = errors.Join(errOut, txzFile.Close()) }()
	txzFormat, txzReader, err := archives.Identify(ctx, txzFilename, txzFile)
	if err != nil {
		return err
	}
	txzExtractor, ok := txzFormat.(archives.Extractor)
	if !ok {
		return fmt.Errorf("txz file is not an extractor")
	}

	return txzExtractor.Extract(ctx, txzReader, func(_ context.Context, info archives.FileInfo) error {
		return handleTxzExtractFile(info, dest)
	})
}

func handleTxzExtractFile(info archives.FileInfo, destRoot string) (errOut error) {
	dest := filepath.Clean(string(os.PathSeparator) + info.NameInArchive)
	dest = strings.TrimPrefix(dest, string(os.PathSeparator))
	dest = filepath.Join(destRoot, dest)

	if !strings.HasPrefix(
		filepath.Clean(dest)+string(os.PathSeparator),
		filepath.Clean(destRoot)+string(os.PathSeparator),
	) {
		return fmt.Errorf("illegal file path: %s", dest)
	}

	parentDir := filepath.Dir(dest)
	err := os.MkdirAll(parentDir, 0o700)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return os.MkdirAll(dest, info.Mode())
	}

	if info.LinkTarget != "" {
		return os.Symlink(info.LinkTarget, dest)
	}

	file, err := info.Open()
	if err != nil {
		return err
	}
	defer func() { errOut = errors.Join(errOut, file.Close()) }()

	dstFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY, info.Mode())
	if err != nil {
		return err
	}
	defer func() { errOut = errors.Join(errOut, dstFile.Close()) }()

	_, err = io.Copy(dstFile, file)
	return err
}
