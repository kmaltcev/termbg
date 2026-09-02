package ghostty

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetBackgroundWritesImageOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")

	a := New(path)
	if err := a.SetBackground("/tmp/foo.png"); err != nil {
		t.Fatalf("SetBackground: %v", err)
	}

	got := readFile(t, path)
	if !strings.Contains(got, "background-image = /tmp/foo.png") {
		t.Fatalf("missing background-image line, got:\n%s", got)
	}
	for _, key := range []string{keyFit, keyPosition, keyRepeat, keyColor, keyImageOpacity} {
		if strings.Contains(got, key+" =") {
			t.Fatalf("unexpected %s line present when unconfigured, got:\n%s", key, got)
		}
	}
}

func TestSetBackgroundWritesFitPositionRepeatColorOpacity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")

	a := New(path)
	a.fit = "cover"
	a.position = "top-right"
	repeat := true
	a.repeat = &repeat
	a.color = "#000000"
	imageOpacity := 0.6
	a.imageOpacity = &imageOpacity

	if err := a.SetBackground("/tmp/foo.png"); err != nil {
		t.Fatalf("SetBackground: %v", err)
	}

	got := readFile(t, path)
	for _, want := range []string{
		"background-image = /tmp/foo.png",
		"background-image-fit = cover",
		"background-image-position = top-right",
		"background-image-repeat = true",
		"background = #000000",
		"background-image-opacity = 0.6",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q, got:\n%s", want, got)
		}
	}
}

func TestSetBackgroundReplacesExistingKeysAndPreservesOthers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	initial := "font-size = 14\n" +
		"background-image = /old.png\n" +
		"background-image-fit = stretch\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatalf("seeding config: %v", err)
	}

	a := New(path)
	a.fit = "contain"

	if err := a.SetBackground("/new.png"); err != nil {
		t.Fatalf("SetBackground: %v", err)
	}

	got := readFile(t, path)
	if !strings.Contains(got, "font-size = 14") {
		t.Fatalf("unrelated line lost, got:\n%s", got)
	}
	if !strings.Contains(got, "background-image = /new.png") {
		t.Fatalf("image not replaced, got:\n%s", got)
	}
	if !strings.Contains(got, "background-image-fit = contain") {
		t.Fatalf("fit not replaced, got:\n%s", got)
	}
	if strings.Count(got, "background-image-fit") != 1 {
		t.Fatalf("expected exactly one background-image-fit line, got:\n%s", got)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}
