package idgen

import (
	"regexp"
	"testing"
)

var v4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewV4(t *testing.T) {
	id, err := NewV4()
	if err != nil {
		t.Fatalf("NewV4 returned error: %v", err)
	}
	if !v4Pattern.MatchString(id) {
		t.Fatalf("NewV4() = %q, does not match v4 UUID pattern", id)
	}
}

func TestNewV4Unique(t *testing.T) {
	a, err := NewV4()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewV4()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("NewV4() produced the same id twice: %q", a)
	}
}
