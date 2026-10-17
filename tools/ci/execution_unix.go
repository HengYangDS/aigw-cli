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
	denial := "direct_read=denied"
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, os.ErrPermission):
		// A directory the review identity cannot traverse hides the resource
		// itself, so the operator's exact path is its only identity.
		denial = "direct_read=denied traversal=denied identity=operator-asserted"
	case err != nil:
		return fmt.Errorf("protected resource identity is unproved: %w", err)
	case !info.Mode().IsRegular():
		return errors.New("protected resource identity is unproved: not a regular file")
	default:
		file, err := os.Open(path)
		if err == nil {
			return errors.Join(errors.New("protected resource is readable by native review identity"), file.Close())
		}
		if !errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("protected resource denial is unproved: %w", err)
		}
	}
	output, err := inquiry(command{Name: "sudo", Args: []string{"-n", "-l"}, Env: []string{"LC_ALL=C", "LANG=C"}, Timeout: 5 * time.Second})
	noGrant := strings.Contains(string(output), "is not allowed to run sudo") || strings.Contains(string(output), "is not in the sudoers file")
	if err == nil || errors.Is(err, context.DeadlineExceeded) || !noGrant {
		return errors.New("native review identity has an elevation grant or its absence is unproved")
	}
	_, err = fmt.Fprintf(stdout, "Native review resource=%s uid=%d %s sudo_grant=none\n", path, os.Geteuid(), denial)
	return err
}
