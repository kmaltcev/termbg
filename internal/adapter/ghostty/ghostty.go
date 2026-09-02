// Package ghostty implements a termbg adapter.Adapter for the Ghostty
// terminal (https://ghostty.org), which reads background-image and
// related keys from its config file and (in recent versions) reloads
// them live when the file changes on disk.
package ghostty

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kmaltcev/termbg/internal/adapter"
)

func init() {
	adapter.Register("ghostty", newFromConfig)
}

const configKey = "background-image"

// Adapter writes the background-image key into a Ghostty config file.
type Adapter struct {
	configPath string
}

func newFromConfig(cfg map[string]any) (adapter.Adapter, error) {
	path, _ := cfg["config_path"].(string)
	if path == "" {
		var err error
		path, err = defaultConfigPath()
		if err != nil {
			return nil, err
		}
	}
	return New(path), nil
}

// New creates a Ghostty adapter that manages the given config file.
func New(configPath string) *Adapter {
	return &Adapter{configPath: configPath}
}

func defaultConfigPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("ghostty adapter: determining home dir: %w", err)
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "ghostty", "config"), nil
}

func (a *Adapter) Name() string { return "ghostty" }

// SetBackground rewrites the background-image line in the Ghostty
// config file (adding it if absent) and leaves all other lines/config
// keys untouched. Ghostty watches its config file and reloads
// automatically; no extra signal/reload step is required on recent
// versions.
func (a *Adapter) SetBackground(imagePath string) error {
	lines, err := readLines(a.configPath)
	if err != nil {
		return fmt.Errorf("ghostty adapter: reading config %s: %w", a.configPath, err)
	}

	newLine := fmt.Sprintf("%s = %s", configKey, imagePath)
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, configKey+" ") || strings.HasPrefix(trimmed, configKey+"=") {
			lines[i] = newLine
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, newLine)
	}

	if err := os.MkdirAll(filepath.Dir(a.configPath), 0o755); err != nil {
		return fmt.Errorf("ghostty adapter: creating config dir: %w", err)
	}
	tmp := a.configPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("ghostty adapter: writing config: %w", err)
	}
	if err := os.Rename(tmp, a.configPath); err != nil {
		return fmt.Errorf("ghostty adapter: replacing config: %w", err)
	}
	return nil
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}
