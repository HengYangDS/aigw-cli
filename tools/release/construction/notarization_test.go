package construction

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNotarizationRequiresOneExplicitNativeAuthenticationMode(t *testing.T) {
	for _, test := range []struct {
		name    string
		input   Notarization
		want    []string
		wantErr bool
	}{
		{name: "keychain", input: Notarization{KeychainProfile: "release-profile"}, want: []string{"--keychain-profile", "release-profile"}},
		{name: "team API key", input: Notarization{APIKeyFile: "AuthKey.p8", APIKeyID: "KEYID", APIIssuerID: "issuer-id"}, want: []string{"--key", "AuthKey.p8", "--key-id", "KEYID", "--issuer", "issuer-id"}},
		{name: "individual API key", input: Notarization{APIKeyFile: "AuthKey.p8", APIKeyID: "KEYID"}, want: []string{"--key", "AuthKey.p8", "--key-id", "KEYID"}},
		{name: "missing", wantErr: true},
		{name: "mixed", input: Notarization{KeychainProfile: "release-profile", APIKeyFile: "AuthKey.p8", APIKeyID: "KEYID"}, wantErr: true},
		{name: "partial API key", input: Notarization{APIKeyFile: "AuthKey.p8"}, wantErr: true},
		{name: "issuer without API key", input: Notarization{APIIssuerID: "issuer-id"}, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual, err := test.input.authArguments()
			if (err != nil) != test.wantErr || !slices.Equal(actual, test.want) {
				t.Fatalf("authentication arguments = %q, %v; want %q, error=%t", actual, err, test.want, test.wantErr)
			}
		})
	}
	for _, submission := range []Notarization{
		{KeychainProfile: "release-profile"},
		{Archive: "upload.zip", SubmissionID: "submission"},
	} {
		if err := VerifyMacOSDistribution(t.Context(), t.TempDir(), "1.2.3", strings.Repeat("a", 40), submission); err == nil {
			t.Fatalf("incomplete notarization input accepted: %#v", submission)
		}
	}
}

func TestNotarizationBindsAcceptedUploadToBothFinalPrograms(t *testing.T) {
	for _, scenario := range []string{"accepted", "pending", "invalid", "wrong-job", "wrong-upload", "wrong-program", "query-failed"} {
		t.Run(scenario, func(t *testing.T) {
			stage := t.TempDir()
			submission, upload := writeNotarizationUpload(t, stage, scenario == "wrong-program")
			status, job, digest := "Accepted", submission.SubmissionID, fmt.Sprintf("%x", sha256.Sum256(upload))
			switch scenario {
			case "pending":
				status = "In Progress"
			case "invalid":
				status = "Invalid"
			case "wrong-job":
				job = "other"
			case "wrong-upload":
				digest = strings.Repeat("0", 64)
			}
			queries := 0
			err := verifyNotarizedArchives(buildRequest{Version: "1.2.3", MacOSSigningIdentity: strings.Repeat("a", 40)}, stage, submission, func(call toolCall) error {
				if call.Name == "/usr/bin/codesign" {
					return nil
				}
				if call.Name != "xcrun" || call.Args[0] != "notarytool" || call.Args[1] != "log" || call.Args[2] != submission.SubmissionID {
					t.Fatalf("unexpected command: %#v", call)
				}
				queries++
				if scenario == "query-failed" {
					return errors.New("query unavailable")
				}
				data, _ := json.Marshal(map[string]any{"jobId": job, "status": status, "sha256": digest})
				return os.WriteFile(call.Args[len(call.Args)-1], data, 0600)
			})
			if (err == nil) != (scenario == "accepted") {
				t.Fatalf("%s: %v", scenario, err)
			}
			if queries != 1 {
				t.Fatalf("queries=%d", queries)
			}
		})
	}
}

func TestNotarizationRejectsMissingOrMalformedAppleEvidence(t *testing.T) {
	for _, test := range []struct {
		name     string
		wantText string
		wantErr  error
	}{
		{name: "missing-log", wantErr: os.ErrNotExist},
		{name: "malformed-log", wantText: "decode Apple notarization log"},
		{name: "missing-upload", wantErr: os.ErrNotExist},
		{name: "invalid-zip", wantText: "not a valid zip file"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stage := t.TempDir()
			submission, upload := writeNotarizationUpload(t, stage, false)
			if test.name == "missing-upload" {
				submission.Archive = filepath.Join(stage, "absent.zip")
			}
			if test.name == "invalid-zip" {
				upload = []byte("not a ZIP")
				if err := os.WriteFile(submission.Archive, upload, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			digest := fmt.Sprintf("%x", sha256.Sum256(upload))
			queries := 0
			err := verifyNotarizedArchives(buildRequest{Version: "1.2.3", MacOSSigningIdentity: strings.Repeat("a", 40)}, stage, submission, func(call toolCall) error {
				if call.Name == "/usr/bin/codesign" {
					return nil
				}
				queries++
				if test.name == "missing-log" {
					return nil
				}
				if test.name == "malformed-log" {
					return os.WriteFile(call.Args[len(call.Args)-1], []byte("{"), 0o600)
				}
				data, _ := json.Marshal(map[string]any{"jobId": submission.SubmissionID, "status": "Accepted", "sha256": digest})
				return os.WriteFile(call.Args[len(call.Args)-1], data, 0o600)
			})
			if queries != 1 || err == nil || test.wantErr != nil && !errors.Is(err, test.wantErr) || test.wantText != "" && !strings.Contains(err.Error(), test.wantText) {
				t.Fatalf("%s: queries=%d error=%v", test.name, queries, err)
			}
		})
	}
}

func writeNotarizationUpload(t *testing.T, stage string, wrongProgram bool) (Notarization, []byte) {
	t.Helper()
	writeMacOSArchiveFixtures(t, stage, true)
	var payload bytes.Buffer
	archive := zip.NewWriter(&payload)
	for _, arch := range []string{"amd64", "arm64"} {
		entry, err := archive.Create(arch + "/aigw")
		if err != nil {
			t.Fatal(err)
		}
		program := []byte(arch)
		if wrongProgram && arch == "arm64" {
			program = []byte("unrelated")
		}
		if _, err := entry.Write(program); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stage, "submission.zip")
	if err := os.WriteFile(path, payload.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return Notarization{Archive: path, SubmissionID: "12345678-1234-1234-1234-123456789abc", APIKeyFile: "AuthKey.p8", APIKeyID: "KEYID", APIIssuerID: "issuer-id"}, payload.Bytes()
}
