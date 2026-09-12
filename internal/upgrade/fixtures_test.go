package upgrade

import (
	"context"
	"os"
	"testing"

	"aigw-cli/internal/process"
)

// Private transport tests must never inherit a developer's release credentials.
func TestMain(m *testing.M) {
	for _, name := range []string{"AIGW_GITHUB_TOKEN", "GITHUB_TOKEN", "GH_TOKEN", "GITLAB_TOKEN"} {
		if err := os.Unsetenv(name); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}

// recordingRunner exposes capture only; file-capability admission stays observable.
type recordingRunner struct {
	output []byte
	err    error
	plans  []process.Plan
}

func (runner *recordingRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	runner.plans = append(runner.plans, plan)
	return append([]byte(nil), runner.output...), runner.err
}

type recordingFileRunner struct {
	recordingRunner
	fileErr      error
	content      []byte
	destinations []string
}

func (runner *recordingFileRunner) RunToFile(_ context.Context, destination string, plan process.Plan) error {
	runner.plans = append(runner.plans, plan)
	runner.destinations = append(runner.destinations, destination)
	if runner.fileErr != nil {
		return runner.fileErr
	}
	if runner.content == nil {
		return nil
	}
	return os.WriteFile(destination, runner.content, 0o600)
}
