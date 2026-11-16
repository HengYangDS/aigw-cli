package construction

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

// goReleaserGitEnvironment returns the Git configuration that GoReleaser needs
// to read a remote's URL in this checkout (`git ls-remote --get-url`), as
// process environment. It writes no Git configuration, and it adds nothing
// where Git already answers.
func goReleaserGitEnvironment(root string, environment []string, run toolRunner) ([]string, error) {
	var branch bytes.Buffer
	if err := run(toolCall{Name: "git", Directory: root, Args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, Env: environment, Stdout: &branch}); err != nil {
		return nil, fmt.Errorf("read release source branch: %w", err)
	}
	name := strings.TrimSpace(branch.String())
	detached := name == "" || name == "HEAD"
	key := "branch." + name + ".remote"
	if !detached {
		var selected bytes.Buffer
		if err := run(toolCall{Name: "git", Directory: root, Args: []string{"config", "get", "--default=", key}, Env: environment, Stdout: &selected}); err != nil {
			return nil, fmt.Errorf("read release source peer: %w", err)
		}
		if strings.TrimSpace(selected.String()) != "" {
			return nil, nil
		}
	}
	var remoteOutput bytes.Buffer
	if err := run(toolCall{Name: "git", Directory: root, Args: []string{"remote"}, Env: environment, Stdout: &remoteOutput}); err != nil {
		return nil, fmt.Errorf("read release source peers: %w", err)
	}
	remotes := strings.Fields(remoteOutput.String())
	if len(remotes) == 0 {
		// No Forge is configured: the checkout itself, as Git's relative path
		// from the directory where GoReleaser runs, is the remote's URL.
		return gitConfigurationEnvironment("remote.origin.url", ".")
	}
	if detached || len(remotes) < 2 || slices.Contains(remotes, "origin") {
		return nil, nil // Detached Forge checkouts retain their native clone remote.
	}
	for _, peer := range []string{"github", "gitlab"} {
		if slices.Contains(remotes, peer) {
			return gitConfigurationEnvironment(key, peer)
		}
	}
	return nil, errors.New("release source needs an explicitly selected native branch peer")
}

// gitConfigurationEnvironment adds one Git configuration value after those
// that the process environment already carries.
func gitConfigurationEnvironment(key, value string) ([]string, error) {
	count := 0
	if inherited := os.Getenv("GIT_CONFIG_COUNT"); inherited != "" {
		var err error
		count, err = strconv.Atoi(inherited)
		if err != nil || count < 0 {
			return nil, errors.New("invalid inherited Git configuration count")
		}
	}
	return []string{fmt.Sprintf("GIT_CONFIG_COUNT=%d", count+1), fmt.Sprintf("GIT_CONFIG_KEY_%d=%s", count, key), fmt.Sprintf("GIT_CONFIG_VALUE_%d=%s", count, value)}, nil
}
