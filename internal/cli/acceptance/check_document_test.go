package cli_test

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"
)

// checkReport decodes the fields of `aigw check`'s document that these tests
// read.
type checkReport struct {
	SchemaVersion  int             `json:"schema_version"`
	OK             bool            `json:"ok"`
	Depth          string          `json:"depth"`
	Clients        []checkedClient `json:"clients"`
	EnabledClients int             `json:"enabled_clients"`
	State          string          `json:"state"`
	Problem        string          `json:"problem"`
	NextAction     string          `json:"next_action"`
	NextDetail     string          `json:"next_detail"`
}

// checkedClient is one checked client's climb.
type checkedClient struct {
	ID        string         `json:"id"`
	Enabled   bool           `json:"enabled"`
	Model     string         `json:"model"`
	Provider  string         `json:"provider"`
	Transport string         `json:"transport"`
	State     string         `json:"state"`
	Depth     string         `json:"depth"`
	Skipped   []checkSkip    `json:"skipped"`
	Failure   *checkFailure  `json:"failure"`
	Checks    []checkedLocal `json:"checks"`
	Native    bool           `json:"native_model_preference"`
	// NextAction and NextDetail are the client's one next step.
	NextAction string `json:"next_action"`
	NextDetail string `json:"next_detail"`
}

type checkSkip struct {
	Depth  string `json:"depth"`
	Reason string `json:"reason"`
}

type checkFailure struct {
	Depth      string `json:"depth"`
	Problem    string `json:"problem"`
	Kind       string `json:"kind"`
	HTTPStatus int    `json:"http_status"`
	Retryable  bool   `json:"retryable"`
}

type checkedLocal struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Detail string `json:"detail"`
}

func decodeCheck(t *testing.T, out *bytes.Buffer) checkReport {
	t.Helper()
	var report checkReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("decode check's document: %v\n%s", err, out)
	}
	return report
}

// client returns the report's entry of id, and whether it has one.
func (report checkReport) client(id string) (checkedClient, bool) {
	index := slices.IndexFunc(report.Clients, func(client checkedClient) bool { return client.ID == id })
	if index < 0 {
		return checkedClient{}, false
	}
	return report.Clients[index], true
}

// clientState is one client's state in a document.
type clientState struct {
	ID                 string `json:"id"`
	State              string `json:"state"`
	Detail             string `json:"detail"`
	NextAction         string `json:"next_action"`
	NextDetail         string `json:"next_detail"`
	ProjectionDeferred bool   `json:"projection_deferred"`
	// FilesDeferred is how status reports ProjectionDeferred.
	FilesDeferred bool `json:"client_files_deferred"`
}

// clientStates decodes a document's clients by ID: a map, as status and
// doctor write them, or a list of entries with their id, as check writes them.
type clientStates map[string]clientState

func (states *clientStates) UnmarshalJSON(data []byte) error {
	var mapped map[string]clientState
	if err := json.Unmarshal(data, &mapped); err == nil {
		*states = mapped
		return nil
	}
	var listed []clientState
	if err := json.Unmarshal(data, &listed); err != nil {
		return err
	}
	*states = clientStates{}
	for _, entry := range listed {
		(*states)[entry.ID] = entry
	}
	return nil
}
