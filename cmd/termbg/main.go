// Command termbg is a tiny CLI (with a tray-icon frontend planned) that
// rotates a terminal emulator's background image, on request or on a
// schedule, from a pluggable set of image sources.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kmaltcev/termbg/internal/adapter"
	"github.com/kmaltcev/termbg/internal/config"
	"github.com/kmaltcev/termbg/internal/rotator"
	"github.com/kmaltcev/termbg/internal/source"

	// Blank imports register each built-in source/adapter with the
	// registries in internal/source and internal/adapter. Adding a
	// new source or adapter package here (or via a plugin build tag)
	// is the only change needed to make it available by name in the
	// config file — nothing else in this file changes.
	_ "github.com/kmaltcev/termbg/internal/adapter/ghostty"
	_ "github.com/kmaltcev/termbg/internal/source/local"
	_ "github.com/kmaltcev/termbg/internal/source/wallhaven"
)

var configPathFlag string

// version is set at build time via -ldflags "-X main.version=...";
// defaults to "dev" for local/unreleased builds.
var version = "dev"

func main() {
	root := &cobra.Command{
		Use:     "termbg",
		Short:   "Rotate terminal emulator backgrounds on schedule or on request",
		Version: version,
	}
	root.PersistentFlags().StringVar(&configPathFlag, "config", "", "path to config.toml (default: $XDG_CONFIG_HOME/termbg/config.toml)")

	root.AddCommand(nextCmd(), statusCmd(), sourcesCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "termbg:", err)
		os.Exit(1)
	}
}

func resolveConfigPath() (string, error) {
	if configPathFlag != "" {
		return configPathFlag, nil
	}
	return config.DefaultPath()
}

func buildRotator(cfg *config.Config) (*rotator.Rotator, error) {
	if cfg.Source == "" {
		return nil, fmt.Errorf("config: %q is not set", "source")
	}
	if cfg.Terminal == "" {
		return nil, fmt.Errorf("config: %q is not set", "terminal")
	}

	src, err := source.New(cfg.Source, cfg.SourceConfig[cfg.Source])
	if err != nil {
		return nil, err
	}
	term, err := adapter.New(cfg.Terminal, cfg.TerminalConfig[cfg.Terminal])
	if err != nil {
		return nil, err
	}
	return rotator.New(src, term), nil
}

func nextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "Apply the next background image immediately",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			rot, err := buildRotator(cfg)
			if err != nil {
				return err
			}
			img, err := rot.Next(context.Background())
			if err != nil {
				return err
			}
			fmt.Printf("applied %s background: %s\n", cfg.Terminal, img)
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the resolved config and registered sources/adapters",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			fmt.Printf("config file:    %s\n", path)
			fmt.Printf("active source:  %s\n", cfg.Source)
			fmt.Printf("active terminal: %s\n", cfg.Terminal)
			fmt.Printf("schedule:       %s\n", cfg.Schedule)
			return nil
		},
	}
}

func sourcesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sources",
		Short: "List registered source and terminal adapter plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("sources:  ", source.Names())
			fmt.Println("terminals:", adapter.Names())
			return nil
		},
	}
}
