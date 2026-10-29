package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

const configTemplate = `# Vigil Intelligence: choose deepseek, openai, gemini, or compatible.
VIGIL_PROVIDER=deepseek
# Paste your provider's API key here. Do not share this file.
VIGIL_API_KEY=
# Optional model override (provider defaults are used when blank).
VIGIL_MODEL=
# For compatible APIs only: full API base URL, e.g. http://localhost:11434/v1
VIGIL_BASE_URL=
`

func defaultConfigPath() (string, error) {
	dir, e := os.UserConfigDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(dir, "vigil", "config.env"), nil
}
func initConfig(path string) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return fmt.Errorf("could not create configuration directory")
	}
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		if os.IsExist(e) {
			return fmt.Errorf("configuration already exists; edit it instead of overwriting it")
		}
		return fmt.Errorf("could not create configuration file")
	}
	defer f.Close()
	if _, e = f.WriteString(configTemplate); e != nil {
		return fmt.Errorf("could not write configuration file")
	}
	return nil
}
func loadConfig(explicit string) error {
	load := func(path string, required bool) error {
		e := godotenv.Load(path)
		if e != nil && (!os.IsNotExist(e) || required) {
			return fmt.Errorf("could not load configuration; check file permissions and KEY=value syntax")
		}
		return nil
	}
	if explicit != "" {
		return load(explicit, true)
	}
	if e := load(".env", false); e != nil {
		return e
	}
	path, e := defaultConfigPath()
	if e != nil {
		return nil
	}
	return load(path, false)
}
