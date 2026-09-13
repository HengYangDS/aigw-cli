// Package install owns the executable lifecycle exposed by the AIGW product.
package install

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/transaction"
	"aigw-cli/internal/upgrade"

	"github.com/spf13/cobra"
)

var writeFileAtomic = transaction.WriteFileAtomic

// NewInspectionCommand constructs read-only observation of the running installation.
func NewInspectionCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	command := &cobra.Command{
		Use: "installation", Short: "Inspect the current portable program and retained predecessor",
		Long: "Observe program paths, byte counts and SHA-256 without changing files or reading Account configuration. This does not verify release trust, rollback readiness or client connectivity.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			result, err := upgrade.InspectInstallation(runtime.Executable, runtime.Version)
			if err != nil {
				return err
			}
			if jsonMode {
				return presentation.WriteJSON(runtime.Out, result)
			}
			render := invocation.Renderer(runtime)
			render.ProductTitle("Portable installation")
			render.Row("Version", result.Version)
			render.Row("Command", result.CommandPath)
			render.Row("Payload", result.Payload.Path)
			render.Row("SHA-256", result.Payload.SHA256)
			if result.Rollback == nil {
				render.Row("Rollback", "No retained predecessor")
			} else {
				render.Row("Rollback", result.Rollback.Path)
				render.Row("SHA-256", result.Rollback.SHA256)
			}
			return render.Err()
		},
	}
	command.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return command
}

// NewInstallCommand constructs the portable executable installation command.
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

// NewUninstallCommand constructs the command that removes only the installed portable executable and its owned backup.
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
			_, statErr := os.Stat(runtime.Config.Path())
			if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
				return fmt.Errorf("inspect AIGW configuration: %w", statErr)
			}
			if statErr == nil {
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

// Install atomically places the current executable at the requested portable target while retaining one rollback copy.
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
	previous, err := os.ReadFile(targetPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read installed portable AIGW executable: %w", err)
	}
	if err == nil && bytes.Equal(data, previous) {
		return os.Chmod(targetPath, 0o755)
	}
	if err == nil {
		mode := os.FileMode(0o755)
		if current, statErr := os.Stat(targetPath); statErr == nil {
			mode = current.Mode().Perm()
		}
		if err := writeFileAtomic(upgrade.RollbackPath(targetPath), previous, mode); err != nil {
			return fmt.Errorf("save previous portable AIGW executable: %w", err)
		}
	}
	if err := writeFileAtomic(targetPath, data, 0o755); err != nil {
		return err
	}
	return os.Chmod(targetPath, 0o755)
}

// Uninstall removes the portable executable and its owned rollback copy while tolerating absence.
func Uninstall(target string) error {
	if strings.TrimSpace(target) == "" {
		return errors.New("portable uninstall target is empty")
	}
	for _, path := range []string{target, upgrade.RollbackPath(target)} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove portable AIGW file %s: %w", path, err)
		}
	}
	return nil
}
