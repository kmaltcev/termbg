// Package source defines the pluggable background-image source
// interface and a registry that source implementations register
// themselves into. Adding a new source (e.g. Unsplash, Pexels, an RSS
// feed) only requires implementing Source and calling Register in an
// init() function; no changes are needed anywhere else in the app.
package source

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Source produces image file paths that can be used as a terminal
// background. Implementations are responsible for downloading/caching
// any remote images and returning a local file path.
type Source interface {
	// Name returns the unique, config-facing name of the source, e.g.
	// "local" or "wallhaven".
	Name() string

	// Next returns the local path to the next image to use.
	Next(ctx context.Context) (path string, err error)
}

// Factory builds a Source from a raw config section for that source.
// The map contains whatever keys were present under
// [source.<name>] in the config file.
type Factory func(cfg map[string]any) (Source, error)

var (
	mu        sync.RWMutex
	factories = map[string]Factory{}
)

// Register registers a source factory under the given name. It is
// meant to be called from an init() function of the source's package.
// Registering the same name twice panics, since that indicates a
// programming error rather than a runtime condition.
func Register(name string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := factories[name]; exists {
		panic(fmt.Sprintf("source: factory already registered for %q", name))
	}
	factories[name] = f
}

// New builds a Source by name using its registered factory.
func New(name string, cfg map[string]any) (Source, error) {
	mu.RLock()
	f, ok := factories[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("source: no such source %q (known: %v)", name, Names())
	}
	return f(cfg)
}

// Names returns the sorted list of currently registered source names.
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
