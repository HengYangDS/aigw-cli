package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// rejectedToken is the Account Token every scripted provider refuses.
const rejectedToken = "sk-rejected-token"

// upstream answers the requests AIGW and the scripted clients send: model
// catalogues, one capped inference request per protocol, the DMXAPI account
// diagnostics and the GitHub release metadata of repository team/aigw. It
// refuses rejectedToken with HTTP 401, and an inference request that asks
// for AIGW_OK receives it.
func upstream() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") == rejectedToken || r.Header.Get("Authorization") == "Bearer "+rejectedToken {
			http.Error(w, `{"error":{"message":"invalid token"}}`, http.StatusUnauthorized)
			return
		}
		request, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		answer := "pong"
		if strings.Contains(string(request), "AIGW_OK") {
			answer = "AIGW_OK"
		}
		var body string
		path := r.URL.Path
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/models"):
			body = `{"data":[{"id":"team-model"},{"id":"spare-model"}]}`
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/messages"):
			body = fmt.Sprintf(`{"type":"message","role":"assistant","content":[{"type":"text","text":%q}]}`, answer)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/responses"):
			body = fmt.Sprintf(`{"status":"completed","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":%q}]}]}`, answer)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/chat/completions"):
			body = fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":%q}}]}`, answer)
		case r.Method == http.MethodGet && path == "/api/user/self":
			body = `{"success":true,"data":{"quota":5000000}}`
		case r.Method == http.MethodGet && path == "/api/token/search":
			body = `{"success":true,"data":{"page_size":100,"items":[{"name":"team","key":"team**********0001","status":1,"used_quota":500000,"remain_quota":2500000,"remain_count":42}]}}`
		case r.Method == http.MethodGet && path == "/repos/team/aigw/releases/latest":
			body = `{"tag_name":"v0.1.0-dev","assets":[]}`
		default:
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	})
}

// scriptedClaude stands in for Claude Code. It reports a version, and a
// stream-json print reads the given settings, runs their credential helper
// and sends the prompt to their Anthropic endpoint, as Claude Code does.
func scriptedClaude() {
	args := os.Args[1:]
	if slices.Equal(args, []string{"--version"}) {
		fmt.Println("2.1.312 (Claude Code)")
		return
	}
	index := slices.Index(args, "--settings")
	if index < 0 || index+1 >= len(args) || !slices.Contains(args, "stream-json") {
		scriptedFailure("scripted Claude Code answers only stream-json prints with --settings")
	}
	data, err := os.ReadFile(args[index+1])
	if err != nil {
		scriptedFailure(err.Error())
	}
	var settings struct {
		APIKeyHelper string            `json:"apiKeyHelper"`
		Env          map[string]string `json:"env"`
		Model        string            `json:"model"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		scriptedFailure(err.Error())
	}
	words := commandWords(settings.APIKeyHelper)
	if len(words) == 0 {
		scriptedFailure("settings have no credential helper")
	}
	token, err := exec.Command(words[0], words[1:]...).Output()
	if err != nil {
		scriptedFailure("credential helper: " + err.Error())
	}
	prompt, _ := json.Marshal(map[string]any{
		"model": settings.Model, "max_tokens": 16,
		"messages": []map[string]string{{"role": "user", "content": args[len(args)-1]}},
	})
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(settings.Env["ANTHROPIC_BASE_URL"], "/")+"/v1/messages", strings.NewReader(string(prompt)))
	if err != nil {
		scriptedFailure(err.Error())
	}
	request.Header.Set("X-Api-Key", strings.TrimSpace(string(token)))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		scriptedFailure(err.Error())
	}
	defer func() { _ = response.Body.Close() }()
	var message struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&message) != nil || len(message.Content) == 0 {
		scriptedFailure(fmt.Sprintf("endpoint answered HTTP %d", response.StatusCode))
	}
	result, _ := json.Marshal(map[string]any{"type": "result", "is_error": false, "result": message.Content[0].Text})
	fmt.Println(string(result))
}

func scriptedFailure(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}

// commandWords splits a projected helper command line into its words. Single
// or double quotes group a word, which is how AIGW quotes the executable.
func commandWords(line string) []string {
	var words []string
	var word strings.Builder
	quote, inWord := rune(0), false
	for _, r := range line {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote == 0 && (r == '\'' || r == '"'):
			quote, inWord = r, true
		case quote == 0 && r == ' ':
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		words = append(words, word.String())
	}
	return words
}
