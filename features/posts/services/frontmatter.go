package services

import (
	"strings"

	"gopkg.in/yaml.v3"
)

const frontmatterDelim = "---"

// parseFrontmatter splits a gray-matter-style markdown file (YAML
// frontmatter between "---" delimiters, followed by the body) into the
// parsed frontmatter and the remaining body. Content with no frontmatter
// delimiter is returned unchanged as the body, with an empty frontmatter
// map.
func parseFrontmatter(raw string) (map[string]any, string, error) {
	trimmed := strings.TrimLeft(raw, "\uFEFF \t\r\n")
	if !strings.HasPrefix(trimmed, frontmatterDelim) {
		return map[string]any{}, raw, nil
	}

	rest := strings.TrimPrefix(trimmed[len(frontmatterDelim):], "\r\n")
	rest = strings.TrimPrefix(rest, "\n")

	closeIdx := strings.Index(rest, "\n"+frontmatterDelim)
	if closeIdx == -1 {
		return map[string]any{}, raw, nil
	}

	yamlPart := rest[:closeIdx]
	body := strings.TrimLeft(rest[closeIdx+len("\n"+frontmatterDelim):], "\r\n")

	var frontmatter map[string]any
	if err := yaml.Unmarshal([]byte(yamlPart), &frontmatter); err != nil {
		return nil, "", err
	}
	if frontmatter == nil {
		frontmatter = map[string]any{}
	}

	return frontmatter, body, nil
}

func frontmatterString(fm map[string]any, key string) string {
	v, ok := fm[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func frontmatterStringSlice(fm map[string]any, key string) []string {
	v, ok := fm[key]
	if !ok || v == nil {
		return nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
