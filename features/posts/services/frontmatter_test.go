package services

import "testing"

func TestParseFrontmatter(t *testing.T) {
	t.Run("splits YAML frontmatter from the body", func(t *testing.T) {
		raw := "---\ntitle: Hashing 101\nthumbnail: /img/thumb.png\ndescription: A post about hashing\ntags:\n  - crypto\n  - hashing\n---\n\n# Hello\n\nBody text.\n"

		fm, body, err := parseFrontmatter(raw)
		if err != nil {
			t.Fatal(err)
		}

		if frontmatterString(fm, "title") != "Hashing 101" {
			t.Errorf("title = %q, want Hashing 101", frontmatterString(fm, "title"))
		}
		if frontmatterString(fm, "thumbnail") != "/img/thumb.png" {
			t.Errorf("thumbnail = %q, want /img/thumb.png", frontmatterString(fm, "thumbnail"))
		}
		if frontmatterString(fm, "description") != "A post about hashing" {
			t.Errorf("description = %q, want %q", frontmatterString(fm, "description"), "A post about hashing")
		}
		tags := frontmatterStringSlice(fm, "tags")
		if len(tags) != 2 || tags[0] != "crypto" || tags[1] != "hashing" {
			t.Errorf("tags = %v, want [crypto hashing]", tags)
		}
		if body != "# Hello\n\nBody text.\n" {
			t.Errorf("body = %q, want %q", body, "# Hello\n\nBody text.\n")
		}
	})

	t.Run("handles folded YAML scalars", func(t *testing.T) {
		raw := "---\ndescription: >-\n  Line one\n  line two\n---\nBody\n"

		fm, body, err := parseFrontmatter(raw)
		if err != nil {
			t.Fatal(err)
		}
		if frontmatterString(fm, "description") != "Line one line two" {
			t.Errorf("description = %q, want %q", frontmatterString(fm, "description"), "Line one line two")
		}
		if body != "Body\n" {
			t.Errorf("body = %q, want %q", body, "Body\n")
		}
	})

	t.Run("treats content with no frontmatter delimiter as pure body", func(t *testing.T) {
		raw := "# No frontmatter here\n\nJust body text.\n"

		fm, body, err := parseFrontmatter(raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(fm) != 0 {
			t.Errorf("frontmatter = %v, want empty", fm)
		}
		if body != raw {
			t.Errorf("body = %q, want the original content unchanged", body)
		}
	})

	t.Run("returns an error for malformed YAML", func(t *testing.T) {
		raw := "---\ntitle: [unterminated\n---\nBody\n"

		_, _, err := parseFrontmatter(raw)
		if err == nil {
			t.Fatal("expected an error for malformed YAML frontmatter")
		}
	})
}
