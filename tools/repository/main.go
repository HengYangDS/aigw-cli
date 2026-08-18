// Command repository validates repository-owned release and text contracts.
package main

import (
	"flag"
	"fmt"
	"os"
)

var exit = os.Exit

func main() { exit(execute(os.Args[1:], os.Stderr)) }

func execute(args []string, stderr *os.File) int {
	if err := run(args); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func run(args []string) error {
	flags := flag.NewFlagSet("repository", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() == 0 {
		return fmt.Errorf("usage: repository [--root path] <changelog|release-epoch|go-format|protected-lifecycle>")
	}
	switch flags.Arg(0) {
	case "changelog":
		return checkChangelog(*root, flags.Args()[1:])
	case "release-epoch":
		return printReleaseEpoch(*root, flags.Args()[1:])
	case "go-format":
		return checkGoFormat(*root)
	case "protected-lifecycle":
		return checkProtectedLifecycle(*root)
	default:
		return fmt.Errorf("unknown repository check: %s", flags.Arg(0))
	}
}
