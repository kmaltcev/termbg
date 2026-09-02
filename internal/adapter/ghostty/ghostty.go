// Package ghostty implements a termbg adapter.Adapter for the Ghostty
// terminal (https://ghostty.org), which reads background-image and
// related keys from its config file. Ghostty does NOT auto-reload
// config changes: the user must reload manually (Ctrl+Shift+, or the
// "Reload Configuration" menu item) or fully restart the app.
package ghostty

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kmaltcev/termbg/internal/adapter"
)

func init() {
	adapter.Register("ghostty", newFromConfig)
}

const (
	keyImage        = "background-image"
	keyFit          = "background-image-fit"
	keyPosition     = "background-image-position"
	keyRepeat       = "background-image-repeat"
	keyColor        = "background"
	keyImageOpacity = "background-image-opacity"
)

// ValidFits are the fit values Ghostty accepts for background-image-fit
// (available since Ghostty 1.2.0). Default is "contain".
var ValidFits = []string{"contain", "cover", "stretch", "none"}

// ValidPositions are the position values Ghostty accepts for
// background-image-position (available since Ghostty 1.2.0). Default
// is "center".
var ValidPositions = []string{
	"top-left", "top-center", "top-right",
	"center-left", "center", "center-right",
	"bottom-left", "bottom-center", "bottom-right",
}

// Adapter writes the background-image key (and optional fit/position/
// repeat/color/image-opacity keys) into a Ghostty config file.
type Adapter struct {
	configPath   string
	fit          string
	position     string
	repeat       *bool
	color        string
	imageOpacity *float64
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
	a := New(path)
	a.fit, _ = cfg["fit"].(string)
	a.position, _ = cfg["position"].(string)
	if repeat, ok := cfg["repeat"].(bool); ok {
		a.repeat = &repeat
	}
	a.color, _ = cfg["color"].(string)
	if imageOpacity, ok := cfg["image_opacity"].(float64); ok {
		a.imageOpacity = &imageOpacity
	}
	return a, nil
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

// SetBackground rewrites the background-image line (and, if
// configured, background-image-fit/-position/-repeat,
// background/background-image-opacity) in the Ghostty config file,
// adding keys that are absent and leaving all other lines/config keys
// untouched. Ghostty does not reload config changes automatically: the
// user must reload manually (Ctrl+Shift+, or the "Reload
// Configuration" menu item) for the new background to appear.
func (a *Adapter) SetBackground(imagePath string) error {
	lines, err := readLines(a.configPath)
	if err != nil {
		return fmt.Errorf("ghostty adapter: reading config %s: %w", a.configPath, err)
	}

	lines = setKeyValue(lines, keyImage, imagePath)
	if a.fit != "" {
		lines = setKeyValue(lines, keyFit, a.fit)
	}
	if a.position != "" {
		lines = setKeyValue(lines, keyPosition, a.position)
	}
	if a.repeat != nil {
		lines = setKeyValue(lines, keyRepeat, fmt.Sprintf("%t", *a.repeat))
	}
	if a.color != "" {
		lines = setKeyValue(lines, keyColor, a.color)
	}
	if a.imageOpacity != nil {
		lines = setKeyValue(lines, keyImageOpacity, strconv.FormatFloat(*a.imageOpacity, 'g', -1, 64))
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

// setKeyValue replaces the first "key = ..." / "key=..." line for key
// in lines with a freshly formatted one, or appends it if absent.
func setKeyValue(lines []string, key, value string) []string {
	newLine := fmt.Sprintf("%s = %s", key, value)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=") {
			lines[i] = newLine
			return lines
		}
	}
	return append(lines, newLine)
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
