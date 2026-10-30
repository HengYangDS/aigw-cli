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
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"
	"aigw-cli/internal/upgrade"

	"github.com/spf13/cobra"
)

var writeFileAtomic = store.WriteFileAtomic

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
				return terminal.WriteJSON(runtime.Out, result)
			}
			render := invocation.Renderer(runtime)
			render.ProductTitle("Portable installation")
			rows := []terminal.Field{
				{Label: "Version", Value: result.Version},
				{Label: "Command", Value: result.CommandPath},
				{Label: "Payload", Value: result.Payload.Path},
				{Label: "SHA-256", Value: result.Payload.SHA256},
			}
			if result.Rollback == nil {
				rows = append(rows, terminal.Field{Label: "Rollback", Value: "No retained predecessor"})
			} else {
				rows = append(rows,
					terminal.Field{Label: "Rollback", Value: result.Rollback.Path},
					terminal.Field{Label: "SHA-256", Value: result.Rollback.SHA256},
				)
			}
			render.Rows(rows...)
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
			render.Text("PATH is unchanged. Use the installed path directly, or add its directory to PATH before running the command below.")
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
		Short: "Withdraw managed projections and remove a portable program",
		Long: "Withdraw AIGW-owned client projections, then remove the portable\n" +
			"executable and its rollback copy. This retains Account and Route data\n" +
			"and stored Tokens. Homebrew-managed copies must be removed with\n" +
			"Homebrew; this command does not manage them.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(target) == "" {
				target = runtime.Executable
			}
			if err := upgrade.RequirePortableOwnership(target); err != nil {
				return err
			}
			_, targetErr := os.Stat(target)
			if targetErr != nil && !errors.Is(targetErr, os.ErrNotExist) {
				return fmt.Errorf("inspect installed AIGW executable: %w", targetErr)
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
				synchronizer := runtime.Synchronizer()
				if err := synchronizer.Withdraw(&after); err != nil {
					return err
				}
				if err := synchronizer.CommitProjection(cmd.Context(), before, after, "uninstall"); err != nil {
					return err
				}
			}
			if err := Uninstall(target); err != nil {
				title := "Portable program removal is incomplete"
				impact := "No AIGW configuration existed; no managed client withdrawal was needed. Tokens and versioned credential readers are retained."
				if statErr == nil {
					title = "Client withdrawal completed; program removal is incomplete"
					impact = "Managed client withdrawal is committed; configuration, Tokens and versioned credential readers are retained."
				}
				return invocation.Problem(runtime,
					title,
					"AIGW could not remove every selected portable program file.",
					impact,
					"Inspect the selected target and retained files; after resolving file access, retry `uninstall --target <path>` from another verified AIGW executable.", err)
			}
			render := invocation.Renderer(runtime)
			render.ProductTitle("Portable uninstall")
			render.Success("Removed AIGW client projections, executable, and its single rollback copy")
			render.Text("Configuration and credential-store secrets were preserved.")
			render.Text("Versioned credential readers were retained for cached or other-installation callers.")
			return nil
		},
	}
	command.Flags().StringVar(&target, "target", "", "installed executable path; defaults to the running executable")
	return command
}

// Install atomically places the current executable at the requested portable target while retaining one rollback copy.
func Install(source, target string) error {
	if err := upgrade.RequirePortableOwnership(source); err != nil {
		return err
	}
	if err := upgrade.RequirePortableOwnership(target); err != nil {
		return err
	}
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
	if err := upgrade.RequirePortableOwnership(target); err != nil {
		return err
	}
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
