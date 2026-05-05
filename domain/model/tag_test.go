package model

import (
	"errors"
	"reflect"
	"testing"
)

func TestNormalizeTagKey(t *testing.T) {
	t.Parallel()

	got := NormalizeTagKey("  Team API  ")
	if got != "team-api" {
		t.Fatalf("NormalizeTagKey() = %q, want team-api", got)
	}
}

func TestNormalizeTagKeysSortsAndDeduplicates(t *testing.T) {
	t.Parallel()

	got := NormalizeTagKeys([]string{"Security", "team-api", "security", "  "})
	want := []string{"security", "team-api"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeTagKeys() = %#v, want %#v", got, want)
	}
}

func TestValidateTagKeyRejectsReservedAndInvalidValues(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"", "all", "bad/key", "bad key!"} {
		if err := ValidateTagKey(key); !errors.Is(err, ErrInvalidTagKey) {
			t.Fatalf("ValidateTagKey(%q) error = %v, want ErrInvalidTagKey", key, err)
		}
	}
}

func TestValidateTagKeyAcceptsKnownTaxonomyForms(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"team-api", "owasp-a01", "cwe-79", "go:security", "mutation.testing"} {
		if err := ValidateTagKey(key); err != nil {
			t.Fatalf("ValidateTagKey(%q) error = %v, want nil", key, err)
		}
	}
}

func TestValidateTagColor(t *testing.T) {
	t.Parallel()

	for _, color := range []string{"", "#0ea5e9", "#fff"} {
		if err := ValidateTagColor(color); err != nil {
			t.Fatalf("ValidateTagColor(%q) error = %v, want nil", color, err)
		}
	}
	if err := ValidateTagColor("blue"); !errors.Is(err, ErrInvalidTagColor) {
		t.Fatalf("ValidateTagColor(blue) error = %v, want ErrInvalidTagColor", err)
	}
}
