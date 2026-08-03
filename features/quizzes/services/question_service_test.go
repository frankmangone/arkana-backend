package services

import "testing"

func TestDedupe(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"no duplicates", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"removes duplicates, preserving first-occurrence order", []string{"b", "a", "b", "c", "a"}, []string{"b", "a", "c"}},
		{"all duplicates collapse to one", []string{"a", "a", "a"}, []string{"a"}},
		{"empty input", nil, nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := dedupe(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("dedupe(%v) = %v, want %v", c.in, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("dedupe(%v) = %v, want %v", c.in, got, c.want)
				}
			}
		})
	}
}
