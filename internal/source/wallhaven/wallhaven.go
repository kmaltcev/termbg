// Package wallhaven implements a termbg source.Source backed by the
// wallhaven.cc public API (https://wallhaven.cc/help/api). It supports
// passing through arbitrary search filters supported by that API (q,
// categories, purity, sorting, order, topRange, atleast, resolutions,
// ratios, colors, seed, ...) via a raw query-parameter map, and caches
// downloaded wallpapers locally so repeated Next() calls (e.g. after a
// restart) don't always require a network round trip.
package wallhaven

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kmaltcev/termbg/internal/source"
)

func init() {
	source.Register("wallhaven", newFromConfig)
}

const (
	apiBase      = "https://wallhaven.cc/api/v1/search"
	envAPIKeyVar = "TERMBG_WALLHAVEN_API_KEY"
)

// Source fetches wallpapers from wallhaven.cc matching a set of search
// parameters, downloads them, and serves them as local file paths.
type Source struct {
	params   url.Values
	apiKey   string
	cacheDir string
	client   *http.Client

	mu      sync.Mutex
	results []wallpaper
	idx     int
}

type wallpaper struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type searchResponse struct {
	Data []wallpaper `json:"data"`
}

func newFromConfig(cfg map[string]any) (source.Source, error) {
	params := url.Values{}
	if raw, ok := cfg["params"].(map[string]any); ok {
		for k, v := range raw {
			params.Set(k, fmt.Sprintf("%v", v))
		}
	}
	// Backward/alternate config shape: map[string]string.
	if raw, ok := cfg["params"].(map[string]string); ok {
		for k, v := range raw {
			params.Set(k, v)
		}
	}

	apiKey := os.Getenv(envAPIKeyVar)
	if apiKey == "" {
		apiKey, _ = cfg["api_key"].(string)
	}

	cacheDir, _ := cfg["cache_dir"].(string)
	if cacheDir == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("wallhaven source: determining cache dir: %w", err)
		}
		cacheDir = filepath.Join(userCache, "termbg", "wallhaven")
	}

	return New(params, apiKey, cacheDir)
}

// New creates a wallhaven source with the given raw search params (see
// https://wallhaven.cc/help/api), an optional API key (required for
// NSFW purity and to use the account's own default filters), and a
// local directory used to cache downloaded images.
func New(params url.Values, apiKey, cacheDir string) (*Source, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("wallhaven source: creating cache dir %s: %w", cacheDir, err)
	}
	return &Source{
		params:   params,
		apiKey:   apiKey,
		cacheDir: cacheDir,
		client:   &http.Client{Timeout: 20 * time.Second},
	}, nil
}

func (s *Source) Name() string { return "wallhaven" }

// Next fetches (if needed) a page of search results matching the
// configured filters and returns the local path to the next wallpaper
// in that page, downloading it first if not already cached.
func (s *Source) Next(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.idx >= len(s.results) {
		if err := s.fetchPage(ctx); err != nil {
			return "", err
		}
		s.idx = 0
	}
	if len(s.results) == 0 {
		return "", fmt.Errorf("wallhaven source: no results for the configured search filters")
	}

	w := s.results[s.idx]
	s.idx++
	return s.download(ctx, w)
}

func (s *Source) fetchPage(ctx context.Context) error {
	q := url.Values{}
	for k, v := range s.params {
		q[k] = v
	}
	if s.apiKey != "" {
		q.Set("apikey", s.apiKey)
	}
	// A random seed keeps sorting=random pagination stable across
	// pages if the caller ever wants to page forward; harmless
	// otherwise since wallhaven ignores unknown/irrelevant params.
	if q.Get("sorting") == "random" && q.Get("seed") == "" {
		q.Set("seed", randomSeed())
	}

	reqURL := apiBase + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("wallhaven source: building request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("wallhaven source: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("wallhaven source: unexpected status %s: %s", resp.Status, body)
	}

	var parsed searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return fmt.Errorf("wallhaven source: decoding response: %w", err)
	}
	s.results = parsed.Data
	return nil
}

func (s *Source) download(ctx context.Context, w wallpaper) (string, error) {
	dest := filepath.Join(s.cacheDir, w.ID+filepath.Ext(w.Path))
	if _, err := os.Stat(dest); err == nil {
		return dest, nil // already cached
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.Path, nil)
	if err != nil {
		return "", fmt.Errorf("wallhaven source: building download request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("wallhaven source: downloading %s: %w", w.Path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wallhaven source: unexpected status downloading %s: %s", w.Path, resp.Status)
	}

	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", fmt.Errorf("wallhaven source: creating %s: %w", tmp, err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", fmt.Errorf("wallhaven source: writing %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("wallhaven source: closing %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", fmt.Errorf("wallhaven source: renaming %s: %w", tmp, err)
	}
	return dest, nil
}

func randomSeed() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
