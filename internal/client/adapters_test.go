package client

import (
	"context"
	"errors"
	"strings"
	"testing"

	"aigw-cli/internal/platform/process"
)

type captureAdapterRunner struct {
	err       error
	calls     int
	deadlines []bool
	plans     []process.Plan
	outputs   [][]byte
	stderr    []byte
	observe   func(process.Plan)
}

func (runner *captureAdapterRunner) RunCaptureStreams(ctx context.Context, plan process.Plan) ([]byte, []byte, error) {
	output, err := runner.RunCapture(ctx, plan)
	if err != nil {
		return nil, output, err
	}
	return output, runner.stderr, nil
}

func (runner *captureAdapterRunner) RunCapture(ctx context.Context, plan process.Plan) ([]byte, error) {
	if runner.observe != nil {
		runner.observe(plan)
	}
	runner.plans = append(runner.plans, plan)
	runner.calls++
	_, hasDeadline := ctx.Deadline()
	runner.deadlines = append(runner.deadlines, hasDeadline)
	if runner.calls <= len(runner.outputs) {
		return runner.outputs[runner.calls-1], runner.err
	}
	return nil, runner.err
}

func TestExternalCredentialRunnerRetainsFailureWithoutUnknownSecrets(t *testing.T) {
	for _, diagnostic := range []string{
		"warning: external client secret=public-secret-marker\n",
		"ERROR external client secret=public-secret-marker\n",
		"INFO external client secret=public-secret-marker\n",
	} {
		runner := externalCredentialRunner{runner: &captureAdapterRunner{outputs: [][]byte{[]byte("AIGW_OK\n")}, stderr: []byte(diagnostic)}}
		output, stderr, err := runner.RunCaptureStreams(t.Context(), process.Plan{})
		if err != nil || string(output) != "AIGW_OK\n" || strings.Contains(string(stderr), "public-secret-marker") {
			t.Fatalf("external diagnostic privacy = %q, %v", stderr, err)
		}
		if process.DiagnosticFailure(stderr) != process.DiagnosticFailure([]byte(diagnostic)) {
			t.Fatal("external credential privacy hid a native failure")
		}
	}
}

func TestExternalCredentialRunnerPreservesCancellationWithoutDiagnostics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := externalCredentialRunner{runner: &captureAdapterRunner{err: errors.New("public-secret-marker")}}
	out, err := runner.RunCapture(ctx, process.Plan{})
	if !errors.Is(err, context.Canceled) || len(out) != 0 {
		t.Fatalf("cancellation=%v, output bytes=%d", err, len(out))
	}
	stdout, stderr, err := runner.RunCaptureStreams(ctx, process.Plan{})
	if !errors.Is(err, context.Canceled) || len(stdout) != 0 || len(stderr) != 0 {
		t.Fatalf("stream cancellation=%v, output bytes=%d/%d", err, len(stdout), len(stderr))
	}
}
