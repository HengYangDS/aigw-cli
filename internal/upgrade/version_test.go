package upgrade

import "testing"

func TestCompareVersionsIgnoresBuildMetadata(t *testing.T) {
	t.Parallel()
	for _, pair := range [][2]string{
		{"v1.2.3+build.1", "1.2.3+build.2"},
		{"1.2.3-rc.1+build.1", "1.2.3-rc.1"},
	} {
		comparison, err := compareVersions(pair[0], pair[1])
		if err != nil || comparison != 0 {
			t.Errorf("compareVersions(%q, %q) = %d, %v; want equal precedence", pair[0], pair[1], comparison, err)
		}
	}
}

func TestParseVersionRequiresCanonicalNumericIdentifiers(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"01.2.3", "1.02.3", "1.2.03", "1.2.3-01", "1.2.3-"} {
		if _, err := parseVersion(value); err == nil {
			t.Errorf("parseVersion(%q) accepted a noncanonical release version", value)
		}
	}
}

func TestCompareVersionsFollowsSemanticPrecedence(t *testing.T) {
	t.Parallel()
	versions := []string{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0"}
	for index := 1; index < len(versions); index++ {
		comparison, err := compareVersions(versions[index-1], versions[index])
		if err != nil || comparison != -1 {
			t.Errorf("compareVersions(%q, %q) = %d, %v; want older", versions[index-1], versions[index], comparison, err)
		}
	}
}

func TestCompareVersionsPropagatesLeftParseError(t *testing.T) {
	if _, err := compareVersions("not-a-version", "1.0.0"); err == nil {
		t.Fatal("compareVersions accepted a malformed left version")
	}
}

func TestCompareVersionsPropagatesRightParseError(t *testing.T) {
	if _, err := compareVersions("1.0.0", "not-a-version"); err == nil {
		t.Fatal("compareVersions accepted a malformed right version")
	}
}

func TestCompareVersionsTreatsPrereleaseAsOlderThanRelease(t *testing.T) {
	got, err := compareVersions("1.0.0", "1.0.0-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("compareVersions(release, prerelease) = %d, want 1", got)
	}
}

func TestCompareVersionsTreatsReleaseAsNewerThanPrerelease(t *testing.T) {
	got, err := compareVersions("1.0.0-alpha", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if got != -1 {
		t.Fatalf("compareVersions(prerelease, release) = %d, want -1", got)
	}
}

func TestCompareVersionsComparesCoreComponents(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.2.0", "1.1.0", 1},
		{"1.1.2", "1.1.1", 1},
		{"1.0.0", "1.0.0", 0},
	}
	for _, tc := range cases {
		got, err := compareVersions(tc.left, tc.right)
		if err != nil {
			t.Fatalf("compareVersions(%q,%q) error = %v", tc.left, tc.right, err)
		}
		if got != tc.want {
			t.Fatalf("compareVersions(%q,%q) = %d, want %d", tc.left, tc.right, got, tc.want)
		}
	}
}

func TestParseVersionRejectsEmptyValue(t *testing.T) {
	if _, err := parseVersion(""); err == nil {
		t.Fatal("parseVersion accepted an empty value")
	}
	if _, err := parseVersion("v"); err == nil {
		t.Fatal("parseVersion accepted a bare v prefix")
	}
}

func TestParseVersionRejectsWrongNumberOfCoreComponents(t *testing.T) {
	if _, err := parseVersion("1.2"); err == nil {
		t.Fatal("parseVersion accepted a two-component version")
	}
	if _, err := parseVersion("1.2.3.4"); err == nil {
		t.Fatal("parseVersion accepted a four-component version")
	}
}

func TestParseVersionRejectsNonNumericCoreComponent(t *testing.T) {
	if _, err := parseVersion("1.a.3"); err == nil {
		t.Fatal("parseVersion accepted a non-numeric core component")
	}
}

func TestParseVersionRejectsInvalidPrerelease(t *testing.T) {
	if _, err := parseVersion("1.2.3-alpha!"); err == nil {
		t.Fatal("parseVersion accepted a prerelease with an invalid character")
	}
}

func TestParseVersionAcceptsValidPrerelease(t *testing.T) {
	parsed, err := parseVersion("v1.2.3-alpha.1")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Major() != 1 || parsed.Minor() != 2 || parsed.Patch() != 3 || parsed.Prerelease() != "alpha.1" {
		t.Fatalf("parsed = %+v", parsed)
	}
}
