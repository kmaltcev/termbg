package wallhaven

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

// TestNextIsRandomizedAcrossFreshSources guards against a regression
// where termbg (a short-lived CLI process) always fetched a fresh
// page and always returned page[0], which is deterministic for
// non-random sort orders (e.g. sorting=views) — meaning every
// invocation of `termbg next` returned the exact same wallpaper. Next()
// must shuffle results so repeated calls, even from brand new Source
// instances (simulating separate CLI invocations), pick varied images.
func TestNextIsRandomizedAcrossFreshSources(t *testing.T) {
	const numResults = 8

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/search":
			w.Header().Set("Content-Type", "application/json")
			data := `{"data":[`
			for i := 0; i < numResults; i++ {
				if i > 0 {
					data += ","
				}
				data += fmt.Sprintf(`{"id":"id%d","path":"http://%s/img/%d.png"}`, i, r.Host, i)
			}
			data += `]}`
			w.Write([]byte(data))
		default:
			// Serve a tiny fake image for any /img/ download request.
			w.Write([]byte("fake-image-bytes"))
		}
	}))
	defer srv.Close()

	oldBase := apiBase
	apiBase = srv.URL + "/api/v1/search"
	defer func() { apiBase = oldBase }()

	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		dir := t.TempDir()
		src, err := New(url.Values{"sorting": {"views"}}, "", dir)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		// Redirect downloads through our test server too.
		src.client = srv.Client()

		path, err := src.Next(context.Background())
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		seen[path] = true
	}

	if len(seen) <= 1 {
		t.Fatalf("expected varied results across fresh Source instances, got only %v", seen)
	}
}

// TestFetchPageExploresMultiplePages guards against a regression where
// fetchPage always requested wallhaven's page 1 and never anything
// else. With a non-random sort order, page 1 is a small, completely
// deterministic set of results — reshuffling it locally only ever
// recycles the same handful of wallpapers, which looks like the
// configured tags/filters "aren't working" even though they're being
// applied correctly server-side. Once meta.last_page > 1, fetchPage
// must pick a random page to explore the full matching result set.
func TestFetchPageExploresMultiplePages(t *testing.T) {
	const lastPage = 50

	var mu sync.Mutex
	seenPages := map[string]int{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/search":
			page := r.URL.Query().Get("page")
			if page == "" {
				page = "1"
			}
			mu.Lock()
			seenPages[page]++
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"data":[{"id":"page-%s","path":"http://%s/img/%s.png"}],"meta":{"last_page":%d}}`,
				page, r.Host, page, lastPage)
		default:
			w.Write([]byte("fake-image-bytes"))
		}
	}))
	defer srv.Close()

	oldBase := apiBase
	apiBase = srv.URL + "/api/v1/search"
	defer func() { apiBase = oldBase }()

	dir := t.TempDir()
	src, err := New(url.Values{"sorting": {"views"}}, "", dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	src.client = srv.Client()

	// Force many fresh fetches (idx always resets past len(results)==1)
	// so we get a good sample of requested pages.
	for i := 0; i < 30; i++ {
		if _, err := src.Next(context.Background()); err != nil {
			t.Fatalf("Next: %v", err)
		}
		src.mu.Lock()
		src.idx = len(src.results) // force a fresh fetchPage next call
		src.mu.Unlock()
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seenPages) <= 1 {
		t.Fatalf("expected requests across multiple pages, got only %v", seenPages)
	}
}
