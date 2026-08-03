package db

import "testing"

func TestPlaceholders(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{n: 0, want: ""},
		{n: -1, want: ""},
		{n: 1, want: "?"},
		{n: 3, want: "?,?,?"},
	}
	for _, tt := range tests {
		if got := Placeholders(tt.n); got != tt.want {
			t.Errorf("Placeholders(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestToAny(t *testing.T) {
	t.Run("strings", func(t *testing.T) {
		got := ToAny([]string{"a", "b", "c"})
		want := []any{"a", "b", "c"}
		if len(got) != len(want) {
			t.Fatalf("len = %d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
			}
		}
	})

	t.Run("ints", func(t *testing.T) {
		got := ToAny([]int{4, 8, 15})
		want := []any{4, 8, 15}
		if len(got) != len(want) {
			t.Fatalf("len = %d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
			}
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		got := ToAny([]string{})
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})
}
