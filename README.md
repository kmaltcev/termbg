# termbg

A tiny cross-platform (macOS + Linux) utility that rotates terminal
background images/wallpapers on a schedule or on demand — a "wallpaper
changer" for your terminal emulator.

## The problem

Modern terminal emulators (Ghostty, Kitty, iTerm2, WezTerm, ...) support
setting a background image, but none of them ship a way to automatically
rotate that image over time or swap it on request, the way desktop
wallpaper managers do. Today that means manually editing config files.

## The idea

`termbg` is a single small binary that:

- Runs as a lightweight background process with a **menu bar / tray icon**
  (buttons: Next background, Pause/Resume, Choose folder, Quit).
- Rotates the terminal background image **on a schedule** (interval or
  cron-style, e.g. "every 30 minutes" or "at 9am/9pm").
- Lets you trigger a change **on request**, via the tray menu or the CLI.
- Ships a **CLI** (`termbg next`, `termbg set <path>`, `termbg pause`,
  `termbg status`, ...) that talks to the running instance, so it can be
  scripted, bound to a hotkey, or run from automation.
- Is pluggable per terminal emulator: each supported terminal gets a small
  "adapter" that knows how to write its config format and trigger a reload.
  First target: **Ghostty** (`background-image` config key). Kitty and
  iTerm2 adapters may follow.
- Works identically on **macOS** and **Linux**, packaged as a single
  static binary with optional `launchd`/`systemd --user` units for
  autostart.

## Planned stack

- **Language:** Go — single static binary, easy cross-compilation, no
  runtime dependencies to install.
- **Tray icon:** `fyne.io/systray`
- **CLI:** `spf13/cobra` (or `urfave/cli`)
- **Scheduling:** `robfig/cron/v3` (cron-style) or a simple ticker
- **Config:** TOML file at `~/.config/termbg/config.toml`
- **IPC:** CLI talks to the running tray/daemon process over a local
  Unix domain socket.

## Status

Early prototype / idea stage. Nothing is implemented yet — this README
captures the initial design so implementation can start incrementally.

## License

TBD.
