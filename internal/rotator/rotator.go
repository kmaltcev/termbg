// Package rotator ties a background source and a terminal adapter
// together: Next() fetches the next image from the configured source
// and applies it via the configured terminal adapter.
package rotator

import (
	"context"
	"fmt"

	"github.com/kmaltcev/termbg/internal/adapter"
	"github.com/kmaltcev/termbg/internal/source"
)

// Rotator applies the next image from src to term.
type Rotator struct {
	Source   source.Source
	Terminal adapter.Adapter
}

// New creates a Rotator from an already-constructed source and
// terminal adapter.
func New(src source.Source, term adapter.Adapter) *Rotator {
	return &Rotator{Source: src, Terminal: term}
}

// Next fetches the next image from the source and applies it to the
// terminal, returning the image path that was applied.
func (r *Rotator) Next(ctx context.Context) (string, error) {
	path, err := r.Source.Next(ctx)
	if err != nil {
		return "", fmt.Errorf("rotator: getting next image from %s source: %w", r.Source.Name(), err)
	}
	if err := r.Terminal.SetBackground(path); err != nil {
		return "", fmt.Errorf("rotator: applying background to %s: %w", r.Terminal.Name(), err)
	}
	return path, nil
}
