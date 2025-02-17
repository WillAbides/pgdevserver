package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/willabides/pgtestserver/internal"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: writeknownversions <output dir>")
		os.Exit(1)
	}
	outputDir := os.Args[1]
	err := os.RemoveAll(outputDir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = WriteKnownVersionsFiles(context.Background(), outputDir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// WriteKnownVersionsFiles writes a file for each supported system containing the known versions of
// embedded postgres binaries.
func WriteKnownVersionsFiles(ctx context.Context, outputDir string) error {
	mavenURL := "https://repo1.maven.org/maven2"
	err := os.MkdirAll(outputDir, 0o700)
	if err != nil {
		return err
	}
	var versions []string
	for _, system := range internal.SupportedSystems {
		versions, err = internal.AvailableMavenVersions(ctx, mavenURL, internal.ZonkyGroupID, internal.SystemArtifactID(system))
		if err != nil {
			return err
		}
		versions = internal.FilterVersions(versions)
		internal.SortVersions(versions)
		filename := filepath.Join(
			outputDir,
			fmt.Sprintf("%s.txt", strings.ReplaceAll(system, "/", "_")),
		)
		err = os.WriteFile(filename, []byte(strings.Join(versions, "\n")+"\n"), 0o600)
		if err != nil {
			return err
		}
	}
	return nil
}
