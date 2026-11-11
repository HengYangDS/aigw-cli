package store

import (
	"aigw-cli/internal/domain"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBackupRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aigw.toml")
	store := NewStore(path, testAdmission())
	if err := store.Save(convergenceConfig("old")); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	backup, err := store.LoadBackup()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := backup.Providers["old"]; !ok {
		t.Fatalf("backup config = %#v, want the previous version", backup)
	}
}

func TestLoadBackupSurfacesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aigw.toml")
	if _, err := NewStore(path, testAdmission()).LoadBackup(); err == nil || !strings.Contains(err.Error(), "read previous config backup") {
		t.Fatalf("missing backup error = %v", err)
	}
}

func convergenceConfig(id string) domain.Config {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers[id] = domain.Provider{Label: strings.ToUpper(id), Endpoints: domain.Endpoints{Responses: "https://" + id + ".test/v1"}}
	cfg.Models[domain.NewModelRef(id, id+"-model")] = testModel(strings.ToUpper(id), id, id+"-model", domain.ProtocolResponses)
	cfg.SetSelectedModel(ClientCodex, domain.NewModelRef(id, id+"-model"), "")
	return cfg
}
