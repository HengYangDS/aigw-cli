package upgrade

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"testing"

	"aigw-cli/internal/process"
	"aigw-cli/internal/upgrade/artifact"

	"github.com/hiddeco/sshsig"
	"golang.org/x/crypto/ssh"
)

var releaseTestSigner ssh.Signer

// Private transport tests must never inherit a developer's release credentials.
func TestMain(m *testing.M) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	releaseTestSigner, err = ssh.NewSignerFromKey(privateKey)
	if err != nil {
		panic(err)
	}
	artifact.BuildReleasePublicKey = string(ssh.MarshalAuthorizedKey(releaseTestSigner.PublicKey()))
	for _, name := range []string{"AIGW_GITHUB_TOKEN", "GITHUB_TOKEN", "GH_TOKEN", "GITLAB_TOKEN"} {
		if err := os.Unsetenv(name); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}

func signReleaseForTest(t *testing.T, manifest string) []byte {
	t.Helper()
	signature, err := sshsig.Sign(bytes.NewBufferString(manifest), releaseTestSigner, sshsig.HashSHA512, artifact.SignatureNamespace)
	if err != nil {
		t.Fatal(err)
	}
	return sshsig.Armor(signature)
}

// recordingRunner exposes capture only; file-capability admission stays observable.
type recordingRunner struct {
	output  []byte
	err     error
	plans   []process.Plan
	inspect func(process.Plan) ([]byte, error)
}

func (runner *recordingRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	runner.plans = append(runner.plans, plan)
	if runner.inspect != nil {
		return runner.inspect(plan)
	}
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
