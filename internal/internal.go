package internal

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

var SupportedSystems = []string{
	"linux/amd64",
	"linux/arm64",
	"darwin/amd64",
	"darwin/arm64",
}

const ZonkyGroupID = "io/zonky/test/postgres"

// AvailableMavenVersions queries maven metadata for available versions of a maven artifact.
func AvailableMavenVersions(ctx context.Context, mavenURL, groupID, artifactID string) (_ []string, errOut error) {
	u := fmt.Sprintf("%s/%s/%s/maven-metadata.xml", mavenURL, groupID, artifactID)
	client := &http.Client{Timeout: 300 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
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

// SystemArtifactID returns the maven artifact id for the given system (goos/goarch).
func SystemArtifactID(system string) string {
	system = strings.ReplaceAll(system, "arm64", "arm64v8")
	system = strings.ReplaceAll(system, "/", "-")
	return "embedded-postgres-binaries-" + system
}

// FilterVersions removes everything before 11.0.0.
func FilterVersions(versions []string) []string {
	// These are some early builds that don't work on my darwin/arm64 machine
	badBuilds, err := semver.NewConstraint("<11.7.0-0 || >=12.0.0-0 <12.2.0-0")
	if err != nil {
		panic(err)
	}
	var filtered []string
	for _, v := range versions {
		var ver *semver.Version
		ver, err = semver.NewVersion(v)
		if err != nil || badBuilds.Check(ver) {
			continue
		}
		filtered = append(filtered, v)
	}
	return filtered
}

// SortVersions sorts versions in ascending order.
func SortVersions(versions []string) {
	slices.SortFunc(versions, func(a, b string) int {
		left, leftErr := semver.NewVersion(a)
		right, rightErr := semver.NewVersion(b)
		if leftErr != nil && rightErr != nil {
			return strings.Compare(a, b)
		}
		if leftErr != nil {
			return 1
		}
		if rightErr != nil {
			return -1
		}
		return left.Compare(right)
	})
}

func mustNewConstraints(v string) *semver.Constraints {
	c, err := semver.NewConstraint(v)
	if err != nil {
		panic(err)
	}
	return c
}
