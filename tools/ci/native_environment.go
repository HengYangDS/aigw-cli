package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// secretServiceCollection is the private session collection that becomes the
// default alias for the system credential journey.
const secretServiceCollection = "/org/freedesktop/secrets/collection/session"

// nativeMode is the job behavior that GitLab inputs select besides the release
// acceptance arguments.
type nativeMode struct {
	fullQuality   bool
	protectedFile string
	secretService bool
	refreshLocks  bool
	windowsInput  bool
}

// nativeJob is one GitLab native job as its declared inputs select it.
type nativeJob struct {
	nativeMode
	arguments []string
}

// nativeHost carries the process facts and runners a GitLab native job uses.
type nativeHost struct {
	getenv   func(string) string
	platform string
	workdir  string
	// session is set inside the job's private Secret Service session.
	session bool
	inquiry outputRunner
	attach  commandRunner
}

// nativeJobFromEnvironment maps the GitLab job inputs to native acceptance. It
// owns the logic the pipeline once wrote as bash and PowerShell, so every
// platform reads the same inputs the same way.
func nativeJobFromEnvironment(getenv func(string) string, platform, workdir string) (nativeJob, error) {
	manual := getenv("CI_PIPELINE_SOURCE") == "web" || getenv("CI_PIPELINE_SOURCE") == "api"
	diagnostic := getenv("AIGW_NATIVE_DIAGNOSTIC_CLIENT")
	if (diagnostic != "" || getenv("AIGW_NATIVE_PERFORMANCE") == "true" || getenv("AIGW_NATIVE_PERFORMANCE_ATTRIBUTION") == "true") && !manual {
		return nativeJob{}, errors.New("native diagnostics and performance require a manual pipeline")
	}
	pack := getenv("AIGW_NATIVE_INPUT_PACKAGE")
	job := nativeJob{
		fullQuality:  getenv("AIGW_FULL_NATIVE_QUALITY") == "true",
		refreshLocks: getenv("AIGW_REFRESH_LOCKS") == "true",
		// Windows supplies the official clients from the public input itself.
		windowsInput: platform == "windows" && pack != "" && (getenv("AIGW_NATIVE_CLIENTS") == "true" || diagnostic != ""),
	}
	if platform == "darwin" && getenv("CI_COMMIT_REF_PROTECTED") == "false" {
		job.protectedFile = getenv("AIGW_REVIEW_PROTECTED_FILE")
	}
	if platform == "linux" {
		// Linux release inputs reach the system credential store.
		for _, name := range []string{"AIGW_NATIVE_INPUT_PACKAGE", "AIGW_CANDIDATE_ARTIFACTS", "AIGW_CANDIDATE_TAG", "AIGW_BASELINE_TAG", "AIGW_BASELINE_ARTIFACTS"} {
			job.secretService = job.secretService || getenv(name) != ""
		}
	}
	archive := ""
	if job.windowsInput {
		archive = windowsJobPath(getenv, "native-input.tar")
	}
	arguments, err := nativeAcceptanceArguments(getenv, archive, workdir)
	job.arguments = arguments
	return job, err
}

// nativeAcceptanceArguments maps the job inputs to accept-native flags. A
// caller-owned archive replaces the input package download.
func nativeAcceptanceArguments(getenv func(string) string, archive, workdir string) ([]string, error) {
	arguments := []string{"--peer", "gitlab", "--repository", getenv("CI_PROJECT_URL")}
	for _, option := range []struct{ variable, flag string }{
		{"AIGW_BASELINE_TAG", "--baseline-tag"},
		{"AIGW_BASELINE_ARTIFACTS", "--baseline-artifacts"},
		{"AIGW_CANDIDATE_TAG", "--tag"},
	} {
		if value := getenv(option.variable); value != "" {
			arguments = append(arguments, option.flag, value)
		}
	}
	if value := getenv("AIGW_CANDIDATE_ARTIFACTS"); value != "" {
		arguments = append(arguments, "--artifacts", value, "--candidate")
	}
	if value := getenv("AIGW_CANDIDATE_SOURCE"); value != "" {
		arguments = append(arguments, "--candidate-source", value)
	}
	if value := getenv("AIGW_NATIVE_INPUT_PACKAGE"); value != "" {
		input := []string{"--input-package", value}
		if archive != "" {
			input = []string{"--input-archive", archive}
		}
		arguments = append(append(arguments, input...), "--input-sha256", getenv("AIGW_NATIVE_INPUT_SHA256"), "--candidate")
	}
	if getenv("AIGW_NATIVE_PERFORMANCE") == "true" {
		arguments = append(arguments, "--performance", filepath.Join(workdir, "build", "verification", "performance"))
	}
	if getenv("AIGW_NATIVE_PERFORMANCE_ATTRIBUTION") == "true" {
		arguments = append(arguments, "--performance-attribution")
	}
	if diagnostic := getenv("AIGW_NATIVE_DIAGNOSTIC_CLIENT"); diagnostic != "" {
		return append(arguments, "--diagnostic-client", diagnostic), nil
	}
	if getenv("AIGW_NATIVE_CLIENTS") == "true" {
		if getenv("AIGW_BASELINE_TAG") == "" {
			return nil, errors.New("real-client succession requires AIGW_BASELINE_TAG")
		}
		arguments = append(arguments, "--clients")
	}
	return arguments, nil
}

// nativeTrustDefaults returns the trust-file variables that a job without
// explicit files takes from its GitLab file variables.
func nativeTrustDefaults(getenv func(string) string) []string {
	var defaults []string
	for _, trust := range [][2]string{
		{"AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "AIGW_RELEASE_ALLOWED_SIGNERS"},
		{"AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE", "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS"},
	} {
		if getenv(trust[0]) == "" && getenv(trust[1]) != "" {
			defaults = append(defaults, trust[0]+"="+getenv(trust[1]))
		}
	}
	return defaults
}

// windowsJobPath names an entry in the job-owned directory that the Windows
// job prepares beside its checkout.
func windowsJobPath(getenv func(string) string, name string) string {
	return filepath.Join(filepath.Dir(getenv("CI_PROJECT_DIR")), "aigw-ci-mise-"+getenv("CI_JOB_ID"), name)
}

// runNativeJob runs one GitLab native job from its declared inputs: the source
// bootstrap and any requested lock refresh, the Windows client supply and
// native acceptance. Linux release inputs run inside a private Secret Service
// session that this command starts around itself.
func runNativeJob(host nativeHost, stdout io.Writer, runner commandRunner) error {
	job, err := nativeJobFromEnvironment(host.getenv, host.platform, host.workdir)
	if err != nil {
		return err
	}
	if job.secretService && !host.session {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		return host.attach(command{Name: "dbus-run-session", Args: []string{"--", self, "native", "--platform", host.platform, "--from-env", "--secret-service-session"}})
	}
	environment := nativeTrustDefaults(host.getenv)
	if host.session {
		if err := openSecretServiceSession(host.inquiry); err != nil {
			return err
		}
		environment = append(environment, "AIGW_VERIFY_SYSTEM_KEYRING=1")
	}
	if job.windowsInput {
		clients, err := supplyWindowsInput(withEnvironment(host.getenv, environment), runner)
		if err != nil {
			return err
		}
		environment = append(environment, clients...)
	}
	setup := []command{{Name: "mise", Args: []string{"run", "bootstrap"}}}
	if job.refreshLocks {
		setup = append(setup, command{Name: "mise", Args: []string{"run", "dependencies:resolve"}})
	}
	return runNativeRequest(nativeRequest{
		platform: host.platform, fullQuality: job.fullQuality, protectedFile: job.protectedFile,
		release: job.arguments, environment: environment, setup: setup,
	}, stdout, runner)
}

// openSecretServiceSession makes the private session collection the default
// alias that the system credential journey writes to.
func openSecretServiceSession(inquiry outputRunner) error {
	call := func(method string, arguments ...string) ([]byte, error) {
		return inquiry(command{
			Name:    "gdbus",
			Args:    append([]string{"call", "--session", "--dest", "org.freedesktop.secrets", "--object-path", "/org/freedesktop/secrets", "--method", method}, arguments...),
			Timeout: time.Minute,
		})
	}
	alias, err := call("org.freedesktop.Secret.Service.ReadAlias", "session")
	if err != nil || !bytes.Contains(alias, []byte(secretServiceCollection)) {
		return errors.Join(errors.New("the private Secret Service session collection is unavailable"), err)
	}
	if output, err := call("org.freedesktop.Secret.Service.SetAlias", "default", secretServiceCollection); err != nil {
		return fmt.Errorf("alias the private Secret Service collection: %w: %s", err, bytes.TrimSpace(output))
	}
	return nil
}

// withEnvironment returns a lookup that reads entries before getenv.
func withEnvironment(getenv func(string) string, entries []string) func(string) string {
	values := map[string]string{}
	for _, entry := range entries {
		key, value, _ := strings.Cut(entry, "=")
		values[key] = value
	}
	return func(key string) string {
		if value, ok := values[key]; ok {
			return value
		}
		return getenv(key)
	}
}

// physicalWorkdir returns the checkout's physical path, so retained output
// belongs to the native checkout rather than a runner manager path.
func physicalWorkdir() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(directory)
}

// runAttached runs a command whose own steps qualify their diagnostics, with
// its output attached to this process.
func runAttached(call command) error {
	process := exec.Command(call.Name, call.Args...)
	process.Env = append(process.Environ(), call.Env...)
	process.Stdout, process.Stderr = os.Stdout, os.Stderr
	return process.Run()
}
