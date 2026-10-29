package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderConfiguration(t *testing.T) {
	for _, provider := range []string{"deepseek", "openai", "gemini", "compatible"} {
		t.Run(provider, func(t *testing.T) {
			env := map[string]string{"VIGIL_PROVIDER": provider, "VIGIL_API_KEY": "test-key", "VIGIL_MODEL": "test-model", "VIGIL_BASE_URL": "https://example.com/v1"}
			c := configureAI(func(k string) string { return env[k] })
			if !c.ready() || c.model != "test-model" || c.key != "test-key" {
				t.Fatal("configuration failed")
			}
			if provider == "compatible" && c.endpoint != "https://example.com/v1/chat/completions" {
				t.Fatal("wrong compatible endpoint")
			}
		})
	}
	env := map[string]string{"DEEPSEEK_API_KEY": "legacy", "DEEPSEEK_MODEL": "legacy-model"}
	c := configureAI(func(k string) string { return env[k] })
	if c.provider != "deepseek" || c.key != "legacy" || c.model != "legacy-model" {
		t.Fatal("legacy settings broke")
	}
	env = map[string]string{"VIGIL_PROVIDER": "openai", "DEEPSEEK_API_KEY": "must-not-be-used"}
	c = configureAI(func(k string) string { return env[k] })
	if c.ready() || c.key != "" {
		t.Fatal("keys must not cross providers")
	}
}
func TestCompatibleURLValidation(t *testing.T) {
	for _, tc := range []struct {
		url   string
		valid bool
	}{{"https://example.com/v1", true}, {"http://localhost:11434/v1", true}, {"http://127.0.0.1:8080/v1", true}, {"http://[::1]:8000/v1", true}, {"http://example.com/v1", false}, {"https://user:pass@example.com/v1", false}, {"https://example.com/v1?key=secret", false}, {"", false}} {
		if validBaseURL(tc.url) != tc.valid {
			t.Errorf("URL validation failed for %q", tc.url)
		}
	}
	env := map[string]string{"VIGIL_PROVIDER": "compatible", "VIGIL_MODEL": "local-model", "VIGIL_BASE_URL": "http://localhost:11434/v1"}
	if !configureAI(func(k string) string { return env[k] }).ready() {
		t.Fatal("keyless local server should work")
	}
	env["VIGIL_PROVIDER"] = "unknown"
	if configureAI(func(k string) string { return env[k] }).ready() {
		t.Fatal("unknown provider must be rejected")
	}
}
func TestNativeProviderRequests(t *testing.T) {
	for _, provider := range []string{"openai", "gemini", "compatible"} {
		t.Run(provider, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if _, ok := body["thinking"]; ok {
					t.Error("DeepSeek-only option leaked")
				}
				switch provider {
				case "openai":
					if r.Header.Get("Authorization") != "Bearer fixture" || string(body["store"]) != "false" || !strings.Contains(string(body["input"]), "sample") || len(body["instructions"]) == 0 {
						t.Error("invalid Responses request")
					}
					w.Write([]byte(`{"status":"completed","output":[{"type":"reasoning"},{"type":"message","content":[{"type":"output_text","text":"Observed evidence."}]}]}`))
				case "gemini":
					if r.Header.Get("x-goog-api-key") != "fixture" || r.Header.Get("Authorization") != "" || r.URL.RawQuery != "" || len(body["systemInstruction"]) == 0 || !strings.Contains(string(body["contents"]), "sample") {
						t.Error("invalid Gemini request")
					}
					w.Write([]byte(`{"candidates":[{"finishReason":"STOP","content":{"parts":[{"thought":true,"text":"hidden reasoning"},{"text":"Observed evidence."}]}}]}`))
				default:
					if len(body["messages"]) == 0 {
						t.Error("missing chat messages")
					}
					w.Write([]byte(`{"choices":[{"message":{"content":"Observed evidence."},"finish_reason":"stop"}]}`))
				}
			}))
			defer server.Close()
			c := aiClient{provider: provider, key: "fixture", model: "test-model", endpoint: server.URL, http: server.Client()}
			answer, err := c.ask(context.Background(), "explain", "sample")
			if err != nil || answer != "Observed evidence." {
				t.Fatalf("wrong result: %q %v", answer, err)
			}
		})
	}
}
func TestNativeResponseFailures(t *testing.T) {
	for _, tc := range []struct{ provider, body string }{{"openai", `{"status":"failed","output":[]}`}, {"openai", `{"status":"completed","output":[]}`}, {"gemini", `{"candidates":[{"finishReason":"SAFETY","content":{"parts":[{"text":"unsafe"}]}}]}`}, {"gemini", `{"candidates":[]}`}} {
		if _, err := decodeAnswer(tc.provider, []byte(tc.body)); err == nil {
			t.Fatal("expected unavailable response error")
		}
	}
}

func TestProviderRedirectDoesNotForwardEvidence(t *testing.T) {
	reached := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	c := configureAI(func(k string) string {
		if k == "VIGIL_API_KEY" {
			return "fixture-key"
		}
		return ""
	})
	c.endpoint = redirect.URL
	if _, err := c.ask(context.Background(), "explain", "private fixture"); err == nil {
		t.Fatal("redirect should fail")
	}
	if reached {
		t.Fatal("provider redirect forwarded the request")
	}
}
