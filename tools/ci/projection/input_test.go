package projection

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestNativePackageProjectionUsesOnePortableReleaseInputOwner(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	data, err := os.ReadFile(filepath.Join(root, ".gitlab-ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	type nativeJob struct {
		Before []string `yaml:"before_script"`
		Script []string `yaml:"script"`
		After  []string `yaml:"after_script"`
		Rules  []struct {
			If string `yaml:"if"`
		} `yaml:"rules"`
	}
	var pipeline struct {
		Darwin  nativeJob `yaml:"native-darwin"`
		Linux   nativeJob `yaml:"native-linux"`
		Windows nativeJob `yaml:"native-windows"`
	}
	if err := yaml.Unmarshal(data, &pipeline); err != nil {
		t.Fatal(err)
	}
	for platform, job := range map[string]nativeJob{"darwin": pipeline.Darwin, "linux": pipeline.Linux, "windows": pipeline.Windows} {
		commands := strings.Join(append(slices.Clone(job.Before), job.Script...), "\n")
		for _, required := range []string{"--input-package", "--input-sha256", "--candidate-source", "AIGW_NATIVE_INPUT_PACKAGE"} {
			if !strings.Contains(commands, required) {
				t.Errorf("%s omitted the release-owned native package input %s", platform, required)
			}
		}
		if len(job.After) == 0 {
			t.Errorf("%s has no exact cleanup after a failed native journey", platform)
		}
	}
}
