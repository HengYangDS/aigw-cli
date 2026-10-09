//go:build !windows

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func qualifyProtectedResource(path string, stdout io.Writer, inquiry outputRunner) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("native review requires an exact absolute protected resource")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("protected resource identity is unproved: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("protected resource identity is unproved: not a regular file")
	}
	file, err := os.Open(path)
	if err == nil {
		return errors.Join(errors.New("protected resource is readable by native review identity"), file.Close())
	}
	if !errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("protected resource denial is unproved: %w", err)
	}
	output, err := inquiry(command{Name: "sudo", Args: []string{"-n", "-l"}, Env: []string{"LC_ALL=C", "LANG=C"}, Timeout: 5 * time.Second})
	noGrant := strings.Contains(string(output), "is not allowed to run sudo") || strings.Contains(string(output), "is not in the sudoers file")
	if err == nil || errors.Is(err, context.DeadlineExceeded) || !noGrant {
		return errors.New("native review identity has an elevation grant or its absence is unproved")
	}
	_, err = fmt.Fprintf(stdout, "Native review resource=%s uid=%d direct_read=denied sudo_grant=none\n", path, os.Geteuid())
	return err
}
