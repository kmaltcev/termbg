// Package tray implements termbg's system tray / menu bar frontend.
// It shows a tray icon with a small menu (apply the next background
// on demand, pause/resume the schedule, open the config file, quit)
// and — since the tray app itself is the long-running process — also
// drives the scheduled rotation loop described by the config's
// Schedule field.
//
// Built on github.com/gogpu/systray, a pure-Go (no cgo) tray library,
// so it cross-compiles cleanly alongside the rest of termbg's
// CGO_ENABLED=0 release builds. Tested primarily on macOS; Linux
// (via D-Bus StatusNotifierItem) and Windows are supported by the
// underlying library but not yet a focus of this project.
package tray

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"

	"github.com/gogpu/systray"

	"github.com/kmaltcev/termbg/internal/rotator"
	"github.com/kmaltcev/termbg/internal/scheduler"
)

// App holds everything the tray needs to run.
type App struct {
	// ConfigPath is the config file to open via the "Open config
	// file" menu item.
	ConfigPath string
	// Rotator applies the next background image on request or on
	// schedule.
	Rotator *rotator.Rotator
	// Schedule is the optional cron/"@every" expression controlling
	// automatic rotation; empty disables the schedule (manual only).
	Schedule string
}

// Run shows the tray icon and blocks until the user quits. Returns an
// error if the tray/menu loop or an invalid schedule prevents
// startup.
func Run(a App) error {
	ctx, cancel := context.WithCancel(context.Background())

	var paused atomic.Bool
	var pauseItem *systray.MenuItem

	tr := systray.New()

	next := func() {
		img, err := a.Rotator.Next(ctx)
		if err != nil {
			tr.ShowNotification("termbg", "failed to apply background: "+err.Error())
			return
		}
		tr.ShowNotification("termbg", "applied background: "+filepath.Base(img))
	}

	menu := systray.NewMenu()
	menu.Add("Next background", next)

	if a.Schedule != "" {
		pauseItem = menu.AddCheckbox("Pause schedule", false, func() {
			newState := !paused.Load()
			paused.Store(newState)
			pauseItem.SetChecked(newState)
		})
	}

	menu.Add("Open config file", func() {
		if err := openPath(a.ConfigPath); err != nil {
			tr.ShowNotification("termbg", "failed to open config file: "+err.Error())
		}
	})
	menu.AddSeparator()
	menu.Add("Quit", func() {
		cancel()
		tr.Remove()
		os.Exit(0)
	})

	iconPNG := icon()
	tr.SetIcon(iconPNG).SetTemplateIcon(iconPNG).SetTooltip("termbg").SetMenu(menu).Show()

	if a.Schedule != "" {
		cronSched, err := scheduler.Parse(a.Schedule)
		if err != nil {
			cancel()
			return fmt.Errorf("tray: parsing schedule %q: %w", a.Schedule, err)
		}
		go scheduler.Loop(ctx, cronSched, paused.Load, next)
	}

	defer cancel()
	return tr.Run()
}

// openPath opens path in the OS's default handler for it (e.g.
// Finder's default app on macOS, xdg-open on Linux) rather than an
// $EDITOR, since the tray runs without an attached terminal.
func openPath(path string) error {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "start"
	default:
		cmd = "xdg-open"
	}
	return exec.Command(cmd, path).Start()
}
