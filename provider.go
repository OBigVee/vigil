package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func newAIClient() aiClient { return configureAI(os.Getenv) }
func configureAI(get func(string) string) aiClient {
	c := aiClient{provider: strings.ToLower(strings.TrimSpace(get("VIGIL_PROVIDER"))), http: &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}}
	if c.provider == "" {
		c.provider = "deepseek"
	}
	keyVar, modelVar, defaultModel := "", "", ""
	switch c.provider {
	case "deepseek":
		keyVar = "DEEPSEEK_API_KEY"
		modelVar = "DEEPSEEK_MODEL"
		defaultModel = "deepseek-flash"
		c.endpoint = "https://api.deepseek.com/chat/completions"
	case "openai":
		keyVar = "OPENAI_API_KEY"
		modelVar = "OPENAI_MODEL"
		defaultModel = "gpt-5.6-terra"
		c.endpoint = "https://api.openai.com/v1/responses"
	case "gemini":
		keyVar = "GEMINI_API_KEY"
		modelVar = "GEMINI_MODEL"
		defaultModel = "gemini-3.8-flash"
	case "compatible":
		base := strings.TrimRight(strings.TrimSpace(get("VIGIL_BASE_URL")), "/")
		if !validBaseURL(base) {
			c.configErr = fmt.Errorf("set VIGIL_BASE_URL to an HTTPS API base URL, or HTTP on localhost, without credentials or query parameters")
		}
		c.endpoint = base + "/chat/completions"
	default:
		c.configErr = fmt.Errorf("VIGIL_PROVIDER must be deepseek, openai, gemini, or compatible")
	}
	c.key = get("VIGIL_API_KEY")
	if c.key == "" && keyVar != "" {
		c.key = get(keyVar)
	}
	c.model = strings.TrimSpace(get("VIGIL_MODEL"))
	if c.model == "" && modelVar != "" {
		c.model = strings.TrimSpace(get(modelVar))
	}
	if c.model == "" {
		c.model = defaultModel
	}
	if c.provider == "compatible" && c.model == "" {
		c.configErr = fmt.Errorf("set VIGIL_MODEL for the compatible endpoint")
	}
	if c.provider == "gemini" {
		c.model = strings.TrimPrefix(c.model, "models/")
		c.endpoint = "https://generativelanguage.googleapis.com/v1beta/models/" + url.PathEscape(c.model) + ":generateContent"
	}
	return c
}
func localURL(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil {
		return false
	}
	return u.Hostname() == "localhost" || net.ParseIP(u.Hostname()).IsLoopback()
}
func validBaseURL(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && (u.Scheme == "https" || u.Scheme == "http" && localURL(raw))
}
func (c aiClient) accessError() error {
	if c.configErr != nil {
		return c.configErr
	}
	if c.key == "" && !(c.provider == "compatible" && localURL(c.endpoint)) {
		return fmt.Errorf("configure intelligence access using vigil --init-config, then edit the generated file and restart Vigil; local inspection works without a key")
	}
	return nil
}
func (c aiClient) ready() bool { return c.accessError() == nil }
