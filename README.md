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

## Background sources

Backgrounds are provided by pluggable **source providers**. Built-in
sources planned for the first version:

1. **Local directory** — rotate through image files in a configured
   folder (optionally recursive, shuffled or sequential).
2. **wallhaven.cc API** — fetch wallpapers from
   [wallhaven.cc](https://wallhaven.cc/help/api) using its public `/search`
   endpoint, with full support for its filter query params, e.g.:
   - `q` — tag/keyword search (`+tag`, `-tag`, `@username`, `id:123`,
     `type:png|jpg`, `like:<id>`)
   - `categories` / `purity` — bitmasks (general/anime/people,
     sfw/sketchy/nsfw); NSFW requires an API key
   - `sorting` (`date_added`, `relevance`, `random`, `views`,
     `favorites`, `toplist`) + `order` (`desc`/`asc`) + `topRange`
   - `atleast`, `resolutions`, `ratios`, `colors`
   - `apikey` for the user's own account settings/NSFW access
   - `seed` for stable pagination through `random` results

   Config exposes these as a raw query-param map so any filter Wallhaven
   supports can be used without code changes; the source downloads a
   matching wallpaper, caches it locally, and hands the file path to the
   rotator like any other source.

### Source plugin interface

Each source implements a small common interface, roughly:

```go
type Source interface {
    Name() string
    // Next returns a local file path to an image, downloading/caching
    // it if necessary.
    Next(ctx context.Context) (path string, err error)
}
```

New sources (e.g. Unsplash, Pexels, a custom API, an RSS feed of images)
are added by implementing this interface and registering themselves in a
source registry — no changes needed to the core rotator, scheduler, tray,
or CLI. Each source has its own config section
(`[source.local]`, `[source.wallhaven]`, ...) validated independently.

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
