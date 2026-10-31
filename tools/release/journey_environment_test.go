package main

import (
	"aigw-cli/internal/secret"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
)

func (j *journeyFixture) setEnvironment(key, value string) {
	j.testing.Helper()
	if strings.HasPrefix(key, "AIGW_TOKEN_") {
		j.sensitiveInputs = append(j.sensitiveInputs, value)
	}
	j.environment = append(environmentWithout(j.environment, key), key+"="+value)
}

func TestJourneyEnvironmentUpdatesPreserveOwnedCredentials(t *testing.T) {
	key := secret.EnvironmentKey("native-system-keyring-probe")
	initial := environmentWith([]string{"PATH=host", key + "=ambient"}, map[string]string{"PATH": "isolated"})
	if !slices.Equal(initial, []string{"PATH=isolated"}) {
		t.Fatalf("initial environment retained ambient credentials: %v", initial)
	}
	journey := &journeyFixture{testing: t, environment: initial}
	journey.setEnvironment(key, "synthetic")
	journey.setEnvironment("PATH", "native-tools")
	journey.setEnvironment("PATH", "native-tools")
	if !slices.Equal(journey.environment, []string{key + "=synthetic", "PATH=native-tools"}) {
		t.Fatalf("environment update changed owned credentials: %v", journey.environment)
	}
}

func TestJourneyEnvironmentValuesAreAssignable(t *testing.T) {
	environment := []string{"=C:=C:\\native work", "=D:=D:\\", "AIGW_TEST_VALUE=a=b", "AIGW_TEST_EMPTY=", "unassigned"}
	values := environmentValues(environment)
	want := map[string]string{"AIGW_TEST_VALUE": "a=b", "AIGW_TEST_EMPTY": ""}
	if !maps.Equal(values, want) {
		t.Fatalf("assignable environment includes native process metadata: got=%v want=%v", values, want)
	}
	for key, value := range values {
		t.Setenv(key, value)
		if actual, present := os.LookupEnv(key); !present || actual != value {
			t.Fatalf("native environment replay changed %q", key)
		}
	}
	if got := environmentWith(environment, map[string]string{"AIGW_TEST_VALUE": "replacement"}); !slices.Contains(got, environment[0]) || !slices.Contains(got, environment[1]) {
		t.Fatalf("child environment lost native drive metadata: %v", got)
	}
}

func environmentValues(environment []string) map[string]string {
	values := make(map[string]string, len(environment))
	for _, entry := range environment {
		key, value, present := strings.Cut(entry, "=")
		if present && key != "" {
			values[key] = value
		}
	}
	return values
}

func environmentWith(current []string, replacements map[string]string) []string {
	result := make([]string, 0, len(current)+len(replacements))
	for _, item := range current {
		key, _, found := strings.Cut(item, "=")
		if _, replaced := replacements[key]; found && (replaced || strings.HasPrefix(key, "AIGW_TOKEN_")) {
			continue
		}
		result = append(result, item)
	}
	for key, value := range replacements {
		result = append(result, key+"="+value)
	}
	return result
}

func environmentWithout(current []string, removed ...string) []string {
	keys := make(map[string]struct{}, len(removed))
	for _, key := range removed {
		keys[key] = struct{}{}
	}
	result := make([]string, 0, len(current))
	for _, item := range current {
		key, _, found := strings.Cut(item, "=")
		if _, remove := keys[key]; found && remove {
			continue
		}
		result = append(result, item)
	}
	return result
}
