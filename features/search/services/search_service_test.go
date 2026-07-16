package services

import "testing"

func TestBuildTagFilter(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		matchAll bool
		want     string
	}{
		{
			name:     "single tag matchAll",
			tags:     []string{"plonk"},
			matchAll: true,
			want:     `tags = "plonk"`,
		},
		{
			name:     "multiple tags matchAll",
			tags:     []string{"plonk", "zeroKnowledgeProofs"},
			matchAll: true,
			want:     `tags = "plonk" AND tags = "zeroKnowledgeProofs"`,
		},
		{
			name:     "multiple tags matchAny",
			tags:     []string{"plonk", "entropy"},
			matchAll: false,
			want:     `tags IN ["plonk", "entropy"]`,
		},
		{
			name:     "quotes in a tag cannot break out of the expression",
			tags:     []string{`x" OR id = "1`},
			matchAll: true,
			want:     `tags = "x\" OR id = \"1"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTagFilter(tt.tags, tt.matchAll)
			if got != tt.want {
				t.Errorf("buildTagFilter(%v, %t) = %q, want %q", tt.tags, tt.matchAll, got, tt.want)
			}
		})
	}
}
