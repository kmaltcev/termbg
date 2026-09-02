// Package wizard implements the interactive `termbg init` setup flow:
// an ordered set of prompts (built with charmbracelet/huh) that
// produces a fully populated config.Config, so users never have to
// hand-edit TOML to get started.
package wizard

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/huh/v2"

	"github.com/kmaltcev/termbg/internal/adapter"
	"github.com/kmaltcev/termbg/internal/config"
)

// scheduleCustom is the sentinel value used internally when the user
// picks "custom" in the schedule select, prompting a free-text follow-up.
const scheduleCustom = "__custom__"

// optHeight computes the huh Select/MultiSelect Height needed so all n
// options are visible without scrolling. huh's internal viewport
// height is (Height - title lines - description lines), so we have to
// pad for those to avoid clipping options off-screen.
func optHeight(n int, hasDescription bool) int {
	h := n + 1 // +1 for the title line
	if hasDescription {
		h += 2 // description may wrap to ~2 lines
	}
	return h
}

// Run walks the user through configuring termbg and returns the
// resulting config. existing may be a zero-value *config.Config (fresh
// setup) or a previously loaded one (reconfiguring via `init --force`),
// whose values are used as the form's starting defaults.
//
// Every question is its own huh.Group so the terminal shows exactly
// one question at a time (rather than several stacked questions
// competing for the same screen, which used to truncate option lists).
// Select/MultiSelect fields also set an explicit Height matching their
// option count, so all choices are visible without scrolling.
func Run(existing *config.Config) (*config.Config, error) {
	cfg := *existing
	if cfg.SourceConfig == nil {
		cfg.SourceConfig = map[string]map[string]any{}
	}
	if cfg.TerminalConfig == nil {
		cfg.TerminalConfig = map[string]map[string]any{}
	}

	sourceChoice := cfg.Source
	if sourceChoice == "" {
		sourceChoice = "local"
	}

	terminalChoice := cfg.Terminal
	terminals := adapter.Names()
	if terminalChoice == "" && len(terminals) > 0 {
		terminalChoice = terminals[0]
	}

	ghosttyCfg := cfg.TerminalConfig["ghostty"]
	ghosttyFit, _ := ghosttyCfg["fit"].(string)
	if ghosttyFit == "" {
		ghosttyFit = "contain"
	}
	ghosttyPosition, _ := ghosttyCfg["position"].(string)
	if ghosttyPosition == "" {
		ghosttyPosition = "center"
	}
	ghosttyRepeat, _ := ghosttyCfg["repeat"].(bool)
	ghosttyColor, _ := ghosttyCfg["color"].(string)
	if ghosttyColor == "" {
		ghosttyColor = "#000000"
	}
	ghosttyImageOpacity, ok := ghosttyCfg["image_opacity"].(float64)
	if !ok {
		ghosttyImageOpacity = 1.0
	}
	ghosttyImageOpacityStr := strconv.FormatFloat(ghosttyImageOpacity, 'g', -1, 64)

	localCfg := cfg.SourceConfig["local"]
	localDir, _ := localCfg["dir"].(string)
	localRecursive, _ := localCfg["recursive"].(bool)
	localRotation := "shuffle"
	if shuffle, ok := localCfg["shuffle"].(bool); ok && !shuffle {
		localRotation = "sequential"
	}

	whCfg := cfg.SourceConfig["wallhaven"]
	whAPIKey, _ := whCfg["api_key"].(string)
	whParams, _ := whCfg["params"].(map[string]any)
	if whParams == nil {
		whParams = map[string]any{}
	}
	whTags, _ := whParams["q"].(string)
	whCategories := decodeBitmaskSelection(fmt.Sprintf("%v", whParams["categories"]), []string{"general", "anime", "people"}, []string{"general", "anime", "people"})
	whPurity := decodeBitmaskSelection(fmt.Sprintf("%v", whParams["purity"]), []string{"sfw", "sketchy", "nsfw"}, []string{"sfw"})
	whSorting, _ := whParams["sorting"].(string)
	if whSorting == "" {
		whSorting = "random"
	}
	whResolution, _ := whParams["atleast"].(string)
	whRatio, _ := whParams["ratios"].(string)

	scheduleChoice, scheduleCustomValue := splitSchedule(cfg.Schedule)

	var groups []*huh.Group

	groups = append(groups,
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Where should background images come from?").
				Options(
					huh.NewOption("Local directory", "local"),
					huh.NewOption("wallhaven.cc (online, filterable)", "wallhaven"),
				).
				Height(optHeight(2, false)).
				Value(&sourceChoice),
		),
	)

	if len(terminals) > 1 {
		opts := make([]huh.Option[string], 0, len(terminals))
		for _, t := range terminals {
			opts = append(opts, huh.NewOption(t, t))
		}
		groups = append(groups,
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Which terminal emulator should termbg manage?").
					Options(opts...).
					Height(optHeight(len(opts), false)).
					Value(&terminalChoice),
			),
		)
	}

	// --- local source ---
	isLocal := func() bool { return sourceChoice != "local" }
	groups = append(groups,
		huh.NewGroup(
			huh.NewInput().
				Title("Path to the directory containing your wallpaper images").
				Value(&localDir).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("a directory path is required")
					}
					return nil
				}),
		).WithHideFunc(isLocal),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Also look inside subdirectories?").
				Value(&localRecursive),
		).WithHideFunc(isLocal),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Rotation order").
				Options(
					huh.NewOption("Shuffle (random order)", "shuffle"),
					huh.NewOption("Sequential (alphabetical, wraps around)", "sequential"),
				).
				Height(optHeight(2, false)).
				Value(&localRotation),
		).WithHideFunc(isLocal),
	)

	// --- wallhaven source ---
	isWallhaven := func() bool { return sourceChoice != "wallhaven" }
	groups = append(groups,
		huh.NewGroup(
			huh.NewInput().
				Title("wallhaven.cc API key (leave empty to use $TERMBG_WALLHAVEN_API_KEY, or for SFW-only anonymous access)").
				Description("An API key is only required for NSFW content or to reuse your account's saved browsing settings. It is stored in your local config file, never committed to a repo.").
				Value(&whAPIKey).
				EchoMode(huh.EchoModePassword),
		).WithHideFunc(isWallhaven),
		huh.NewGroup(
			huh.NewInput().
				Title("Tags / keywords to search for (wallhaven `q` filter)").
				Description(`Examples: "nature", "+cyberpunk +city", "-people", "id:123". Leave empty to browse latest wallpapers unfiltered.`).
				Value(&whTags),
		).WithHideFunc(isWallhaven),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Categories").
				Options(
					huh.NewOption("General", "general"),
					huh.NewOption("Anime", "anime"),
					huh.NewOption("People", "people"),
				).
				Height(optHeight(3, false)).
				Value(&whCategories),
		).WithHideFunc(isWallhaven),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Purity").
				Description("NSFW requires an API key.").
				Options(
					huh.NewOption("SFW", "sfw"),
					huh.NewOption("Sketchy", "sketchy"),
					huh.NewOption("NSFW", "nsfw"),
				).
				Height(optHeight(3, true)).
				Value(&whPurity),
		).WithHideFunc(isWallhaven),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Sort results by").
				Options(
					huh.NewOption("Random", "random"),
					huh.NewOption("Date added", "date_added"),
					huh.NewOption("Relevance", "relevance"),
					huh.NewOption("Views", "views"),
					huh.NewOption("Favorites", "favorites"),
					huh.NewOption("Toplist", "toplist"),
				).
				Height(optHeight(6, false)).
				Value(&whSorting),
		).WithHideFunc(isWallhaven),
		huh.NewGroup(
			huh.NewInput().
				Title("Minimum resolution, e.g. 1920x1080 (optional)").
				Value(&whResolution),
		).WithHideFunc(isWallhaven),
		huh.NewGroup(
			huh.NewInput().
				Title("Aspect ratio, e.g. 16x9 (optional)").
				Value(&whRatio),
		).WithHideFunc(isWallhaven),
	)

	// --- schedule (applies regardless of source) ---
	groups = append(groups,
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("How often should the background rotate automatically?").
				Description("You can always trigger a change on request with `termbg next`, regardless of this setting.").
				Options(
					huh.NewOption("Manual only (no automatic rotation)", ""),
					huh.NewOption("Every 15 minutes", "@every 15m"),
					huh.NewOption("Every 30 minutes", "@every 30m"),
					huh.NewOption("Every hour", "@every 1h"),
					huh.NewOption("Every 6 hours", "@every 6h"),
					huh.NewOption("Once a day", "@every 24h"),
					huh.NewOption("Custom cron/interval expression", scheduleCustom),
				).
				Height(optHeight(7, true)).
				Value(&scheduleChoice),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Custom schedule expression").
				Description(`Either a cron expression (e.g. "0 9,21 * * *") or a Go duration via "@every", e.g. "@every 45m".`).
				Value(&scheduleCustomValue).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("required when \"Custom\" is selected")
					}
					return nil
				}),
		).WithHideFunc(func() bool { return scheduleChoice != scheduleCustom }),
	)

	// --- ghostty display options ---
	isGhostty := func() bool { return terminalChoice != "ghostty" }
	groups = append(groups,
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Ghostty: how should the image fit the window? (background-image-fit)").
				Options(
					huh.NewOption("Contain (show whole image, may letterbox)", "contain"),
					huh.NewOption("Cover (fill window, may crop image)", "cover"),
					huh.NewOption("Stretch (fill window, ignores aspect ratio)", "stretch"),
					huh.NewOption("None (no scaling)", "none"),
				).
				Height(optHeight(4, false)).
				Value(&ghosttyFit),
		).WithHideFunc(isGhostty),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Ghostty: where should the image be anchored? (background-image-position)").
				Options(
					huh.NewOption("Center", "center"),
					huh.NewOption("Top left", "top-left"),
					huh.NewOption("Top center", "top-center"),
					huh.NewOption("Top right", "top-right"),
					huh.NewOption("Center left", "center-left"),
					huh.NewOption("Center right", "center-right"),
					huh.NewOption("Bottom left", "bottom-left"),
					huh.NewOption("Bottom center", "bottom-center"),
					huh.NewOption("Bottom right", "bottom-right"),
				).
				Height(optHeight(9, false)).
				Value(&ghosttyPosition),
		).WithHideFunc(isGhostty),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Ghostty: repeat/tile the image to fill blank space? (background-image-repeat)").
				Value(&ghosttyRepeat),
		).WithHideFunc(isGhostty),
		huh.NewGroup(
			huh.NewInput().
				Title("Ghostty: solid background color (background), shown behind/around the image").
				Description(`Hex color, e.g. "#000000" for black. This stays fully opaque; use the next question to make the image itself see-through over it.`).
				Value(&ghosttyColor).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("a color is required, e.g. #000000")
					}
					return nil
				}),
		).WithHideFunc(isGhostty),
		huh.NewGroup(
			huh.NewInput().
				Title("Ghostty: image opacity over the background color (background-image-opacity)").
				Description("0.0 = image invisible (solid color only), 1.0 = fully opaque image (default). Lower this to let the solid background color show through the image.").
				Value(&ghosttyImageOpacityStr).
				Validate(func(s string) error {
					v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
					if err != nil {
						return fmt.Errorf("must be a number, e.g. 0.5")
					}
					if v < 0 {
						return fmt.Errorf("must be 0.0 or greater")
					}
					return nil
				}),
		).WithHideFunc(isGhostty),
	)

	form := huh.NewForm(groups...)

	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("wizard: %w", err)
	}

	cfg.Source = sourceChoice
	cfg.Terminal = terminalChoice
	if scheduleChoice == scheduleCustom {
		cfg.Schedule = scheduleCustomValue
	} else {
		cfg.Schedule = scheduleChoice
	}

	switch sourceChoice {
	case "local":
		cfg.SourceConfig["local"] = map[string]any{
			"dir":       localDir,
			"recursive": localRecursive,
			"shuffle":   localRotation == "shuffle",
		}
	case "wallhaven":
		params := map[string]any{
			"sorting": whSorting,
		}
		if whTags != "" {
			params["q"] = whTags
		}
		if len(whCategories) > 0 {
			params["categories"] = encodeBitmask(whCategories, []string{"general", "anime", "people"})
		}
		if len(whPurity) > 0 {
			params["purity"] = encodeBitmask(whPurity, []string{"sfw", "sketchy", "nsfw"})
		}
		if whResolution != "" {
			params["atleast"] = whResolution
		}
		if whRatio != "" {
			params["ratios"] = whRatio
		}
		cfg.SourceConfig["wallhaven"] = map[string]any{
			"api_key": whAPIKey,
			"params":  params,
		}
	}

	if _, ok := cfg.TerminalConfig[terminalChoice]; !ok {
		cfg.TerminalConfig[terminalChoice] = map[string]any{}
	}
	if terminalChoice == "ghostty" {
		cfg.TerminalConfig["ghostty"]["fit"] = ghosttyFit
		cfg.TerminalConfig["ghostty"]["position"] = ghosttyPosition
		cfg.TerminalConfig["ghostty"]["repeat"] = ghosttyRepeat
		cfg.TerminalConfig["ghostty"]["color"] = ghosttyColor
		if v, err := strconv.ParseFloat(strings.TrimSpace(ghosttyImageOpacityStr), 64); err == nil {
			cfg.TerminalConfig["ghostty"]["image_opacity"] = v
		}
	}

	return &cfg, nil
}

// splitSchedule maps a stored schedule string back to a select value
// plus a custom-value fallback, so re-running the wizard prefills
// sensibly whether the previous value was one of the presets or a
// hand-written expression.
func splitSchedule(schedule string) (choice, custom string) {
	presets := []string{"", "@every 15m", "@every 30m", "@every 1h", "@every 6h", "@every 24h"}
	for _, p := range presets {
		if schedule == p {
			return p, ""
		}
	}
	return scheduleCustom, schedule
}

// decodeBitmaskSelection turns a wallhaven bitmask string (e.g. "110")
// back into the set of selected option names, given the option order
// and a fallback default if mask is empty/invalid.
func decodeBitmaskSelection(mask string, names []string, fallback []string) []string {
	if len(mask) != len(names) {
		return fallback
	}
	var selected []string
	for i, ch := range mask {
		if ch == '1' {
			selected = append(selected, names[i])
		}
	}
	if len(selected) == 0 {
		return fallback
	}
	return selected
}

// encodeBitmask turns a set of selected option names into a wallhaven
// bitmask string in the given option order, e.g. selected=[general,
// people], names=[general,anime,people] -> "101".
func encodeBitmask(selected, names []string) string {
	set := make(map[string]bool, len(selected))
	for _, s := range selected {
		set[s] = true
	}
	var b strings.Builder
	for _, n := range names {
		if set[n] {
			b.WriteString("1")
		} else {
			b.WriteString("0")
		}
	}
	return b.String()
}
