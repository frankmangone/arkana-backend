package services

import (
	"regexp"
	"strings"
)

// Ported from arkana-frontend's scripts/indexing/utils/strip-markdown.js.
// The JS version matches bold/italic delimiters with a backreference
// (e.g. `(\*\*|__)(.*?)\1`), which Go's RE2 engine doesn't support -
// each delimiter pair is written out separately here instead, applied in
// the same triple/double/single order so a triple-star run is never
// mistaken for a double-star one.
//
// LaTeX ($...$) is deliberately left untouched: a generic markdown-to-text
// pass doesn't understand it and would risk mangling equations.
var (
	codeBlockRe    = regexp.MustCompile("(?s)```[^\n]*\n(.*?)```")
	inlineCodeRe   = regexp.MustCompile("`([^`]+)`")
	imageRe        = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	linkRe         = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	headingRe      = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	tripleStarRe   = regexp.MustCompile(`(?s)\*\*\*(.*?)\*\*\*`)
	tripleUnderRe  = regexp.MustCompile(`(?s)___(.*?)___`)
	doubleStarRe   = regexp.MustCompile(`(?s)\*\*(.*?)\*\*`)
	doubleUnderRe  = regexp.MustCompile(`(?s)__(.*?)__`)
	singleStarRe   = regexp.MustCompile(`(?s)\*(.*?)\*`)
	singleUnderRe  = regexp.MustCompile(`(?s)_(.*?)_`)
	blockquoteRe   = regexp.MustCompile(`(?m)^>\s?`)
	bulletListRe   = regexp.MustCompile(`(?m)^\s*[-*+]\s+`)
	numberedListRe = regexp.MustCompile(`(?m)^\s*\d+\.\s+`)
	horizontalRule = regexp.MustCompile(`(?m)^(-{3,}|\*{3,}|_{3,})\s*$`)
	htmlTagRe      = regexp.MustCompile(`<[^>]+>`)
	multiNewlineRe = regexp.MustCompile(`\n{2,}`)
)

// stripMarkdownForSearch converts a markdown post body into plain prose
// for the search index - raw ** / [text](url) syntax reads poorly in a
// search result excerpt.
func stripMarkdownForSearch(markdown string) string {
	s := markdown
	s = codeBlockRe.ReplaceAllString(s, "$1")
	s = inlineCodeRe.ReplaceAllString(s, "$1")
	s = imageRe.ReplaceAllString(s, "")
	s = linkRe.ReplaceAllString(s, "$1")
	s = headingRe.ReplaceAllString(s, "")
	s = tripleStarRe.ReplaceAllString(s, "$1")
	s = tripleUnderRe.ReplaceAllString(s, "$1")
	s = doubleStarRe.ReplaceAllString(s, "$1")
	s = doubleUnderRe.ReplaceAllString(s, "$1")
	s = singleStarRe.ReplaceAllString(s, "$1")
	s = singleUnderRe.ReplaceAllString(s, "$1")
	s = blockquoteRe.ReplaceAllString(s, "")
	s = bulletListRe.ReplaceAllString(s, "")
	s = numberedListRe.ReplaceAllString(s, "")
	s = horizontalRule.ReplaceAllString(s, "")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = multiNewlineRe.ReplaceAllString(s, "\n")
	return strings.TrimSpace(s)
}
