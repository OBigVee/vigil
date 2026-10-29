package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPrivateConfigTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vigil", "config.env")
	if err := initConfig(path); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "VIGIL_API_KEY=\n") {
		t.Fatal("template must have a blank key")
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0600 {
			t.Fatal("configuration must be private")
		}
	}
	if err := initConfig(path); err == nil {
		t.Fatal("must not overwrite existing configuration")
	}
}
func TestConfigEnvironmentPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.env")
	if err := os.WriteFile(path, []byte("VIGIL_PROVIDER=gemini\nVIGIL_API_KEY=file-fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIGIL_PROVIDER", "openai")
	t.Setenv("VIGIL_API_KEY", "environment-fixture")
	if err := loadConfig(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("VIGIL_PROVIDER") != "openai" || os.Getenv("VIGIL_API_KEY") != "environment-fixture" {
		t.Fatal("environment must take precedence")
	}
	if err := loadConfig(path + ".missing"); err == nil {
		t.Fatal("explicit missing file must be reported")
	}
}
func TestConfigErrorsDoNotExposeContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.env")
	os.WriteFile(path, []byte("VIGIL_API_KEY=\"fixture-private-value\n"), 0600)
	err := loadConfig(path)
	if err == nil || strings.Contains(err.Error(), "fixture-private-value") {
		t.Fatal("invalid config must return a sanitized error")
	}
}
