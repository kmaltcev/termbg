// Package local implements a termbg source.Source that rotates through
// image files found in a local directory.
package local

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/kmaltcev/termbg/internal/source"
)

func init() {
	source.Register("local", newFromConfig)
}

var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".bmp": true,
}

// Source rotates through image files in Dir.
type Source struct {
	dir       string
	recursive bool
	shuffle   bool

	mu    sync.Mutex
	files []string
	idx   int
}

func newFromConfig(cfg map[string]any) (source.Source, error) {
	dir, _ := cfg["dir"].(string)
	if dir == "" {
		return nil, fmt.Errorf("local source: %q config key is required", "dir")
	}
	recursive, _ := cfg["recursive"].(bool)
	shuffle, _ := cfg["shuffle"].(bool)
	return New(dir, recursive, shuffle)
}

// New creates a local directory source. If recursive is true,
// subdirectories are scanned too. If shuffle is true, images are served
// in random order; otherwise alphabetically, wrapping around.
func New(dir string, recursive, shuffle bool) (*Source, error) {
	s := &Source{dir: dir, recursive: recursive, shuffle: shuffle}
	if err := s.scan(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Source) Name() string { return "local" }

func (s *Source) scan() error {
	var files []string
	walk := func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if !s.recursive && path != s.dir {
				return filepath.SkipDir
			}
			return nil
		}
		if imageExts[strings.ToLower(filepath.Ext(path))] {
			files = append(files, path)
		}
		return nil
	}
	if err := filepath.WalkDir(s.dir, walk); err != nil {
		return fmt.Errorf("local source: scanning %s: %w", s.dir, err)
	}
	if len(files) == 0 {
		return fmt.Errorf("local source: no images found in %s", s.dir)
	}
	sort.Strings(files)
	s.files = files
	return nil
}

// Next returns the next image path, advancing the rotation.
func (s *Source) Next(_ context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.files) == 0 {
		if err := s.scan(); err != nil {
			return "", err
		}
	}

	if s.shuffle {
		return s.files[rand.Intn(len(s.files))], nil
	}

	f := s.files[s.idx%len(s.files)]
	s.idx++
	return f, nil
}
