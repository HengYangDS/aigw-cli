// Package install owns the executable lifecycle exposed by the AIGW product.
package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/transaction"
	"github.com/spf13/cobra"
)

var writeFileAtomic = transaction.WriteFileAtomic

func NewInstallCommand(runtime invocation.Context) *cobra.Command {
	target := runtime.InstallTarget
	command := &cobra.Command{
		Use:   "install",
		Short: "Install this portable AIGW executable",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if strings.TrimSpace(target) == "" {
				return errors.New("portable installation target is unavailable")
			}
			if err := Install(runtime.Executable, target); err != nil {
				return err
			}
			render := invocation.Renderer(runtime)
			render.ProductTitle("Portable installation")
			render.Success("Installed " + target)
			render.Next("aigw setup")
			return nil
		},
	}
	command.Flags().StringVar(&target, "target", target, "destination executable path")
	return command
}

func NewUninstallCommand(runtime invocation.Context) *cobra.Command {
	var target string
	command := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove one portable AIGW installation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(target) == "" {
				target = runtime.Executable
			}
			if _, err := os.Stat(runtime.Config.Path()); err == nil {
				before, err := runtime.Config.Load()
				if err != nil {
					return err
				}
				after := before.Clone()
				synchronizer := invocation.Synchronizer(runtime)
				if err := synchronizer.Withdraw(&after); err != nil {
					return err
				}
				if err := synchronizer.CommitProjection(cmd.Context(), before, after, "uninstall"); err != nil {
					return err
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("inspect AIGW configuration: %w", err)
			}
			if err := Uninstall(target); err != nil {
				return err
			}
			render := invocation.Renderer(runtime)
			render.ProductTitle("Portable uninstall")
			render.Success("Removed AIGW client projections, executable, and its single rollback copy")
			render.Text("Configuration and credential-store secrets were preserved.")
			return nil
		},
	}
	command.Flags().StringVar(&target, "target", "", "installed executable path; defaults to the running executable")
	return command
}

func Install(source, target string) error {
	sourcePath := filepath.Clean(source)
	targetPath := filepath.Clean(target)
	if sourcePath == targetPath {
		return errors.New("source and target resolve to the same path")
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read portable AIGW executable: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create portable installation directory: %w", err)
	}
	if previous, err := os.ReadFile(targetPath); err == nil {
		mode := os.FileMode(0o755)
		if current, statErr := os.Stat(targetPath); statErr == nil {
			mode = current.Mode().Perm()
		}
		if err := writeFileAtomic(backupPath(targetPath), previous, mode); err != nil {
			return fmt.Errorf("save previous portable AIGW executable: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read installed portable AIGW executable: %w", err)
	}
	if err := writeFileAtomic(targetPath, data, 0o755); err != nil {
		return err
	}
	return os.Chmod(targetPath, 0o755)
}

func Uninstall(target string) error {
	if strings.TrimSpace(target) == "" {
		return errors.New("portable uninstall target is empty")
	}
	for _, path := range []string{target, backupPath(target)} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove portable AIGW file %s: %w", path, err)
		}
	}
	return nil
}

func backupPath(target string) string {
	name := ".aigw.previous"
	if strings.EqualFold(filepath.Ext(target), ".exe") {
		name += ".exe"
	}
	return filepath.Join(filepath.Dir(target), name)
}
