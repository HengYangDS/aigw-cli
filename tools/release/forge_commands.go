package main

import (
	"fmt"
	"io"
)

func forgeCommands() commandSet {
	return commandSet{
		"write-gitlab-release": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 3, "usage: release write-gitlab-release <tag> <base-url> <output>"); err != nil {
				return err
			}
			return writeJSON(args[2], releaseDocument(args[0], args[1]))
		},
		"same-authority": func(args []string, stdout io.Writer) error {
			if err := requireArguments(args, 2, "usage: release same-authority <left-url> <right-url>"); err != nil {
				return err
			}
			same, err := sameAuthority(args[0], args[1])
			if err != nil {
				return err
			}
			if same {
				_, _ = fmt.Fprintln(stdout, "yes")
			} else {
				_, _ = fmt.Fprintln(stdout, "no")
			}
			return nil
		},
		"resolve-redirect": func(args []string, stdout io.Writer) error {
			if err := requireArguments(args, 2, "usage: release resolve-redirect <current-url> <headers-file>"); err != nil {
				return err
			}
			location, err := readLocation(args[1])
			if err != nil {
				return err
			}
			resolved, err := resolveRedirect(args[0], location)
			if err == nil {
				_, _ = fmt.Fprintln(stdout, resolved)
			}
			return err
		},
		"verify-gitlab-release": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 4, "usage: release verify-gitlab-release <expected-json> <actual-json> <asset-list> <tag>"); err != nil {
				return err
			}
			return verifyGitLabRelease(args[0], args[1], args[2], args[3])
		},
		"project-gitlab-response": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 3, "usage: release project-gitlab-response <release-json> <mode> <output>"); err != nil {
				return err
			}
			var expected releasePayload
			if err := readJSON(args[0], &expected); err != nil {
				return err
			}
			projected, err := projectGitLabResponse(expected, args[1])
			if err != nil {
				return err
			}
			return writeJSON(args[2], projected)
		},
		"validate-gitlab-release": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 2, "usage: release validate-gitlab-release <release-json> <tag>"); err != nil {
				return err
			}
			var payload releasePayload
			if err := readJSON(args[0], &payload); err != nil {
				return err
			}
			return validateReleaseDocument(payload, args[1])
		},
	}
}
