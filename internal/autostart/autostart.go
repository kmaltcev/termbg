// Package autostart installs/removes an optional macOS LaunchAgent so
// `termbg tray` starts automatically at login. It is opt-in: nothing
// in termbg configures autostart automatically, users must explicitly
// run `termbg tray autostart enable`.
package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"
)

const label = "dev.kmaltcev.termbg"

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.Exe}}</string>
		<string>tray</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
	<key>StandardOutPath</key>
	<string>{{.LogPath}}</string>
	<key>StandardErrorPath</key>
	<string>{{.LogPath}}</string>
</dict>
</plist>
`

// plistPath returns ~/Library/LaunchAgents/dev.kmaltcev.termbg.plist.
func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("autostart: determining home dir: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
}

// Enable installs and loads a LaunchAgent that runs `<termbg
// executable> tray` at every login. macOS only.
func Enable() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("autostart: not supported on %s yet (macOS only)", runtime.GOOS)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("autostart: locating termbg executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("autostart: resolving termbg executable path: %w", err)
	}

	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("autostart: creating LaunchAgents dir: %w", err)
	}

	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, "Library", "Logs", "termbg-tray.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return fmt.Errorf("autostart: creating log dir: %w", err)
	}

	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return fmt.Errorf("autostart: internal template error: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("autostart: creating %s: %w", path, err)
	}
	defer f.Close()
	if err := tmpl.Execute(f, struct{ Label, Exe, LogPath string }{label, exe, logPath}); err != nil {
		return fmt.Errorf("autostart: writing %s: %w", path, err)
	}

	// Unload first in case an older version is already loaded, then
	// load; launchctl returns a non-zero exit if nothing was loaded,
	// which is fine to ignore here.
	_ = exec.Command("launchctl", "unload", path).Run()
	if out, err := exec.Command("launchctl", "load", "-w", path).CombinedOutput(); err != nil {
		return fmt.Errorf("autostart: launchctl load failed: %w (%s)", err, string(out))
	}
	return nil
}

// Disable unloads and removes the LaunchAgent installed by Enable.
func Disable() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("autostart: not supported on %s yet (macOS only)", runtime.GOOS)
	}

	path, err := plistPath()
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return fmt.Errorf("autostart: not currently enabled (%s not found)", path)
	}

	_ = exec.Command("launchctl", "unload", "-w", path).Run()
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("autostart: removing %s: %w", path, err)
	}
	return nil
}

// Status reports whether the LaunchAgent plist is currently installed.
func Status() (installed bool, path string, err error) {
	path, err = plistPath()
	if err != nil {
		return false, "", err
	}
	_, statErr := os.Stat(path)
	return statErr == nil, path, nil
}
