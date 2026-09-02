// Package adapter defines the interface used to apply a background
// image to a specific terminal emulator's configuration, plus a
// registry so new terminal adapters (Kitty, iTerm2, WezTerm, ...) can
// be added without touching the rotator or CLI.
package adapter

import (
	"fmt"
	"sort"
	"sync"
)

// Adapter knows how to make a given terminal emulator use imagePath as
// its background image.
type Adapter interface {
	// Name is the config-facing identifier, e.g. "ghostty".
	Name() string

	// SetBackground applies imagePath as the terminal background,
	// writing whatever config the terminal needs and triggering a
	// reload if the terminal doesn't pick up config changes live.
	SetBackground(imagePath string) error
}

// Factory builds an Adapter from a raw [terminal.<name>] config section.
type Factory func(cfg map[string]any) (Adapter, error)

var (
	mu        sync.RWMutex
	factories = map[string]Factory{}
)

// Register registers an adapter factory under name. Meant to be called
// from an init() function of the adapter's package.
func Register(name string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := factories[name]; exists {
		panic(fmt.Sprintf("adapter: factory already registered for %q", name))
	}
	factories[name] = f
}

// New builds an Adapter by name using its registered factory.
func New(name string, cfg map[string]any) (Adapter, error) {
	mu.RLock()
	f, ok := factories[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("adapter: no such terminal adapter %q (known: %v)", name, Names())
	}
	return f(cfg)
}

// Names returns the sorted list of currently registered adapter names.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(factories))
	for n := range factories {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
