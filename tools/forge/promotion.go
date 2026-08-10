package main

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
)

type releasePromotionOptions struct {
	repository, expectMain, expectDev string
}

func runReleasePromotion(args []string) error {
	var option releasePromotionOptions
	flags := flag.NewFlagSet("forge promote-release", flag.ContinueOnError)
	flags.StringVar(&option.repository, "repository", ".", "canonical Git repository")
	flags.StringVar(&option.expectMain, "expect-main", "", "exact current main commit")
	flags.StringVar(&option.expectDev, "expect-dev", "", "exact accepted dev commit")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || option.expectMain == "" || option.expectDev == "" {
		return errors.New("usage: forge promote-release --expect-main <commit> --expect-dev <commit> [--repository <path>]")
	}
	return promoteRelease(option)
}

func promoteRelease(option releasePromotionOptions) error {
	repository, err := filepath.Abs(option.repository)
	if err != nil {
		return err
	}
	status, err := gitOutput(repository, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return err
	}
	if status != "" {
		return errors.New("refusing release promotion with a dirty canonical worktree")
	}
	main, err := gitOutput(repository, "rev-parse", "--verify", "refs/heads/main^{commit}")
	if err != nil {
		return err
	}
	dev, err := gitOutput(repository, "rev-parse", "--verify", "refs/heads/dev^{commit}")
	if err != nil {
		return err
	}
	if main != option.expectMain || dev != option.expectDev {
		return fmt.Errorf("release coordinates changed: main=%s dev=%s", main, dev)
	}
	if err := git(repository, nil, "merge-base", "--is-ancestor", main, dev); err != nil {
		return errors.New("release main is not an ancestor of accepted dev")
	}
	if main == dev {
		fmt.Printf("local release root already current: main@%s\n", main)
		return nil
	}
	if err := git(repository, nil, "update-ref", "refs/heads/main", dev, main); err != nil {
		return err
	}
	fmt.Printf("local release root promoted: main@%s\n", dev)
	return nil
}
