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

func goReleaserGitEnvironment(root string, environment []string, run toolRunner) ([]string, error) {
	var branch bytes.Buffer
	if err := run(toolCall{Name: "git", Directory: root, Args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, Env: environment, Stdout: &branch}); err != nil {
		return nil, fmt.Errorf("read release source branch: %w", err)
	}
	name := strings.TrimSpace(branch.String())
	if name == "" || name == "HEAD" {
		return nil, nil // Detached Forge checkouts retain their native clone remote.
	}
	var selected bytes.Buffer
	key := "branch." + name + ".remote"
	if err := run(toolCall{Name: "git", Directory: root, Args: []string{"config", "get", "--default=", key}, Env: environment, Stdout: &selected}); err != nil {
		return nil, fmt.Errorf("read release source peer: %w", err)
	}
	if strings.TrimSpace(selected.String()) != "" {
		return nil, nil
	}
	var remoteOutput bytes.Buffer
	if err := run(toolCall{Name: "git", Directory: root, Args: []string{"remote"}, Env: environment, Stdout: &remoteOutput}); err != nil {
		return nil, fmt.Errorf("read release source peers: %w", err)
	}
	remotes := strings.Fields(remoteOutput.String())
	if len(remotes) < 2 || slices.Contains(remotes, "origin") {
		return nil, nil
	}
	for _, peer := range []string{"github", "gitlab"} {
		if !slices.Contains(remotes, peer) {
			continue
		}
		count := 0
		if value := os.Getenv("GIT_CONFIG_COUNT"); value != "" {
			var err error
			count, err = strconv.Atoi(value)
			if err != nil || count < 0 {
				return nil, errors.New("invalid inherited Git configuration count")
			}
		}
		return []string{fmt.Sprintf("GIT_CONFIG_COUNT=%d", count+1), fmt.Sprintf("GIT_CONFIG_KEY_%d=%s", count, key), fmt.Sprintf("GIT_CONFIG_VALUE_%d=%s", count, peer)}, nil
	}
	return nil, errors.New("release source needs an explicitly selected native branch peer")
}
