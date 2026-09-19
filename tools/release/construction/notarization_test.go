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
	"strings"
	"testing"
)

func TestNotarizationBindsAcceptedUploadToBothFinalPrograms(t *testing.T) {
	for _, scenario := range []string{"accepted", "pending", "invalid", "wrong-job", "wrong-upload", "wrong-program", "query-failed"} {
		t.Run(scenario, func(t *testing.T) {
			stage := t.TempDir()
			writeMacOSArchiveFixtures(t, stage, true)
			var payload bytes.Buffer
			archive := zip.NewWriter(&payload)
			for _, arch := range []string{"amd64", "arm64"} {
				entry, err := archive.Create(arch + "/aigw")
				if err != nil {
					t.Fatal(err)
				}
				program := []byte(arch)
				if scenario == "wrong-program" && arch == "arm64" {
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
			if err := os.WriteFile(path, payload.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			submission := Notarization{Archive: path, SubmissionID: "12345678-1234-1234-1234-123456789abc", KeychainProfile: "test-profile"}
			status, job, digest := "Accepted", submission.SubmissionID, fmt.Sprintf("%x", sha256.Sum256(payload.Bytes()))
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
