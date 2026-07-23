package services

import "testing"

func TestStripMarkdownForSearch(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "strips a fenced code block, keeping its contents",
			in:   "before\n```go\nfmt.Println(1)\n```\nafter",
			want: "before\nfmt.Println(1)\nafter",
		},
		{
			name: "strips inline code ticks",
			in:   "use `foo()` here",
			want: "use foo() here",
		},
		{
			name: "drops images entirely",
			in:   "before ![alt text](img.png) after",
			want: "before  after",
		},
		{
			name: "keeps link text, drops the URL",
			in:   "see [our docs](https://example.com) for more",
			want: "see our docs for more",
		},
		{
			name: "strips heading markers",
			in:   "## A Heading\nbody",
			want: "A Heading\nbody",
		},
		{
			name: "strips triple-star bold-italic, keeping text",
			in:   "***important***",
			want: "important",
		},
		{
			name: "strips double-star bold, keeping text",
			in:   "**bold**",
			want: "bold",
		},
		{
			name: "strips single-star italic, keeping text",
			in:   "*italic*",
			want: "italic",
		},
		{
			name: "strips double-underscore bold, keeping text",
			in:   "__bold__",
			want: "bold",
		},
		{
			name: "strips single-underscore italic, keeping text",
			in:   "_italic_",
			want: "italic",
		},
		{
			name: "strips blockquote markers",
			in:   "> quoted text",
			want: "quoted text",
		},
		{
			name: "strips bullet list markers",
			in:   "- item one\n- item two",
			want: "item one\nitem two",
		},
		{
			name: "strips numbered list markers",
			in:   "1. first\n2. second",
			want: "first\nsecond",
		},
		{
			name: "strips horizontal rules",
			in:   "above\n---\nbelow",
			want: "above\nbelow",
		},
		{
			name: "strips raw HTML tags",
			in:   "text <br/> more text",
			want: "text  more text",
		},
		{
			name: "collapses multiple blank lines",
			in:   "one\n\n\n\ntwo",
			want: "one\ntwo",
		},
		{
			name: "leaves LaTeX untouched",
			in:   "the formula $x^2 + y^2 = z^2$ holds",
			want: "the formula $x^2 + y^2 = z^2$ holds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdownForSearch(tt.in)
			if got != tt.want {
				t.Errorf("stripMarkdownForSearch(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
