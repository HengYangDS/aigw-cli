package construction

import (
	"aigw-cli/internal/program/artifact"
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestPublisherSigningIdentityIsExplicitAndNative(t *testing.T) {
	identity := strings.Repeat("a", 40)
	request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "0", MacOSSigningIdentity: identity}
	err := validateRequest(request)
	if (err == nil) != (runtime.GOOS == "darwin") {
		t.Fatalf("native identity admission: %v", err)
	}

	for _, value := range []string{"publisher name", `a" --anything`, strings.Repeat("g", 40)} {
		request.MacOSSigningIdentity = value
		if err := validateRequest(request); err == nil {
			t.Fatalf("invalid fingerprint accepted: %q", value)
		}
	}
	request.MacOSSigningIdentity = identity
	request.TargetOS = "windows"
	if err := validateRequest(request); err == nil {
		t.Fatal("irrelevant signing identity accepted for Windows-only build")
	}
}

func TestSignedArchiveVerificationPreservesExactIdentityAndFailure(t *testing.T) {
	for _, failure := range []string{"", "amd64", "arm64", "missing"} {
		t.Run(failure, func(t *testing.T) {
			stage := t.TempDir()
			writeMacOSArchiveFixtures(t, stage, failure != "missing")
			sentinel := errors.New("native signature rejected")
			var paths []string
			identity := strings.Repeat("a", 40)
			err := verifySignedArchives(buildRequest{Version: "1.2.3", MacOSSigningIdentity: identity}, stage, func(call toolCall) error {
				if call.Name != "/usr/bin/codesign" || !slices.Contains(call.Args, `-R=anchor apple generic and certificate leaf = H"`+identity+`"`) {
					t.Fatalf("unbound signature verifier: %#v", call)
				}
				path := call.Args[len(call.Args)-1]
				paths = append(paths, path)
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(data) == failure {
					return sentinel
				}
				return nil
			})
			if (err == nil) != (failure == "") {
				t.Fatalf("verification failure=%s: %v", failure, err)
			}
			if failure == "amd64" || failure == "arm64" {
				if !errors.Is(err, sentinel) {
					t.Fatalf("lost verifier cause: %v", err)
				}
			}
			if failure == "" && len(paths) != 2 {
				t.Fatalf("verified paths: %v", paths)
			}
			for _, path := range paths {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("verification residue: %v", err)
				}
			}
		})
	}
}

func TestSignedConstructionRequiresArchiveVerification(t *testing.T) {
	if runtime.GOOS != "darwin" {
		return
	}
	request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "0", MacOSSigningIdentity: strings.Repeat("a", 40)}
	stage, err := buildArchives(request, t.TempDir(), func(toolCall) error { return nil })
	if err == nil || stage != "" {
		t.Fatalf("missing signed archives accepted: %q, %v", stage, err)
	}
}

func writeMacOSArchiveFixtures(t *testing.T, stage string, includeChecksums bool) {
	t.Helper()
	var checksums strings.Builder
	for _, arch := range []string{"amd64", "arm64"} {
		name := (artifact.Target{OS: "darwin", Arch: arch}).ArchiveName("1.2.3")
		path := filepath.Join(stage, name)
		var data bytes.Buffer
		gz := gzip.NewWriter(&data)
		tarWriter := tar.NewWriter(gz)
		payload := []byte(arch)
		if err := tarWriter.WriteHeader(&tar.Header{Name: strings.TrimSuffix(name, ".tar.gz") + "/aigw", Mode: 0o700, Size: int64(len(payload))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(payload); err != nil {
			t.Fatal(err)
		}
		if err := tarWriter.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&checksums, "%x  %s\n", sha256.Sum256(data.Bytes()), name)
	}
	if includeChecksums {
		if err := os.WriteFile(filepath.Join(stage, "checksums.txt"), []byte(checksums.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNativeArchiveConstructionUsesFrozenSigner(t *testing.T) {
	root := releaseRoot(t)
	request := buildRequest{Root: root, Version: "1.2.3", Epoch: "1784246400"}
	var err error
	request.ReleasePublicKey, err = releasePublicKey(signingKey(t))
	if err != nil {
		t.Fatal(err)
	}
	request.SigningKey = filepath.Join(root, "missing.pub")
	environment, err := goReleaserEnvironment(request)
	if err != nil || !slices.Contains(environment, "AIGW_RELEASE_PUBLIC_KEY="+request.ReleasePublicKey) {
		t.Fatalf("frozen signer was reread or changed: %v, %v", environment, err)
	}
	request.ReleasePublicKey = ""
	noExecution := func(call toolCall) error {
		t.Fatalf("unadmitted native build reached %s", call.Name)
		return nil
	}
	if stage, err := buildArchives(request, t.TempDir(), noExecution); err == nil || stage != "" || !strings.Contains(err.Error(), "read release signing public key") {
		t.Fatalf("unreadable signer entered native construction: %q, %v", stage, err)
	}
	request.Epoch = "invalid"
	if _, err := goReleaserEnvironment(request); err == nil {
		t.Fatal("invalid epoch entered release environment")
	}
	if stage, err := buildArchives(request, t.TempDir(), noExecution); err == nil || stage != "" {
		t.Fatalf("invalid epoch entered native construction: %q, %v", stage, err)
	}
}
