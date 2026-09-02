package wizard

import (
	"reflect"
	"testing"
)

func TestEncodeDecodeBitmask(t *testing.T) {
	names := []string{"general", "anime", "people"}

	mask := encodeBitmask([]string{"general", "people"}, names)
	if mask != "101" {
		t.Fatalf("encodeBitmask = %q, want %q", mask, "101")
	}

	got := decodeBitmaskSelection(mask, names, nil)
	want := []string{"general", "people"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decodeBitmaskSelection = %v, want %v", got, want)
	}
}

func TestDecodeBitmaskSelectionFallback(t *testing.T) {
	names := []string{"sfw", "sketchy", "nsfw"}
	fallback := []string{"sfw"}

	if got := decodeBitmaskSelection("", names, fallback); !reflect.DeepEqual(got, fallback) {
		t.Fatalf("empty mask: got %v, want fallback %v", got, fallback)
	}
	if got := decodeBitmaskSelection("000", names, fallback); !reflect.DeepEqual(got, fallback) {
		t.Fatalf("all-zero mask: got %v, want fallback %v", got, fallback)
	}
}

func TestSplitSchedule(t *testing.T) {
	if choice, custom := splitSchedule("@every 30m"); choice != "@every 30m" || custom != "" {
		t.Fatalf("preset: got (%q, %q)", choice, custom)
	}
	if choice, custom := splitSchedule("0 9,21 * * *"); choice != scheduleCustom || custom != "0 9,21 * * *" {
		t.Fatalf("custom: got (%q, %q)", choice, custom)
	}
	if choice, custom := splitSchedule(""); choice != "" || custom != "" {
		t.Fatalf("manual only: got (%q, %q)", choice, custom)
	}
}
