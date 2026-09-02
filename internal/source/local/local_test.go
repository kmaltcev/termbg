package local

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("fake"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
}

func TestSourceSequentialRotation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.png")
	writeFile(t, dir, "b.png")
	writeFile(t, dir, "ignore.txt")

	src, err := New(dir, false, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if src.Name() != "local" {
		t.Fatalf("Name() = %q, want %q", src.Name(), "local")
	}

	ctx := context.Background()
	first, err := src.Next(ctx)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	second, err := src.Next(ctx)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	third, err := src.Next(ctx)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}

	if first == second {
		t.Fatalf("expected sequential rotation to differ, got %q twice", first)
	}
	if third != first {
		t.Fatalf("expected rotation to wrap around to %q, got %q", first, third)
	}
}

func TestSourceRejectsEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if _, err := New(dir, false, false); err == nil {
		t.Fatal("expected error for directory with no images, got nil")
	}
}
