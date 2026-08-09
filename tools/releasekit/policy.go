package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func parseEpoch(raw string) (time.Time, error) {
	epoch, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || epoch < 0 {
		return time.Time{}, errors.New("epoch must be a non-negative integer")
	}
	return time.Unix(epoch, 0).UTC(), nil
}

func validateToolchain(modulePath, actual string) error {
	data, err := os.ReadFile(modulePath)
	if err != nil {
		return err
	}
	expected := ""
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "go" {
			expected = "go" + fields[1]
			break
		}
	}
	if expected == "" {
		return errors.New("release toolchain: go.mod has no Go version")
	}
	if actual != expected {
		return fmt.Errorf("release toolchain: expected %s, found %s", expected, actual)
	}
	return nil
}

func validateReleaseReadiness(version string) error {
	if strings.Contains(version, "-rc.") || strings.Contains(version, "-beta.") || strings.Contains(version, "-alpha.") {
		return nil
	}
	return errors.New("GA release requires protected macOS notarization, Windows Authenticode, and artifact signature verification")
}

var staleReadiness = regexp.MustCompile(`Current status \(20[0-9]{2}-[0-9]{2}-[0-9]{2}\)|0\.1\.0-rc\.[0-9]+|codex/initial-product|GitLab SSH|GitLab API|e082b00`)

func validateReleaseReadinessDocument(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read release evidence contract: %w", err)
	}
	if staleReadiness.Match(data) {
		return errors.New("release evidence contract contains a stale release snapshot")
	}
	return nil
}
