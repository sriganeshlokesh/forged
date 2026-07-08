package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrUnsafe is wrapped by every guardrail failure.
var ErrUnsafe = errors.New("unsafe content")

// numericRe matches numeric tokens including optional currency prefix and suffix.
var numericRe = regexp.MustCompile(`[$€£]?\d[\d,]*(?:\.\d+)?\s?(?:%|[kKmMbBxX])?`)

// normToken normalizes a raw numeric token for comparison:
// lowercase, strip commas, strip all whitespace.
func normToken(tok string) string {
	tok = strings.ToLower(tok)
	tok = strings.ReplaceAll(tok, ",", "")
	tok = strings.Join(strings.Fields(tok), "")
	return tok
}

// stripTags removes HTML tags from s, inserting a space where each tag was.
// This ensures adjacent-element text like <li>100</li><li>200</li>
// produces "100 200" (two separate tokens) not "100200".
func stripTags(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
			b.WriteRune(' ')
		case r == '>' && inTag:
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// NumericFacts extracts the set of normalized numeric tokens from s
// (after HTML tags are stripped).
func NumericFacts(s string) map[string]struct{} {
	plain := stripTags(s)
	matches := numericRe.FindAllString(plain, -1)
	out := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		out[normToken(m)] = struct{}{}
	}
	return out
}

// CheckNoNewNumbers reports an error (wrapping ErrUnsafe) if after contains
// numeric tokens not present in before or extra.
// extra may be nil; it is treated as an empty set.
func CheckNoNewNumbers(before, after string, extra map[string]struct{}) error {
	allowed := NumericFacts(before)
	for k, v := range extra {
		allowed[k] = v
	}
	afterFacts := NumericFacts(after)
	for tok := range afterFacts {
		if _, ok := allowed[tok]; !ok {
			return fmt.Errorf("%w: new numeric token %q introduced", ErrUnsafe, tok)
		}
	}
	return nil
}

// allowedTags is the set of HTML tags SanitizeHTML keeps.
var allowedTags = map[string]bool{
	"p": true, "ul": true, "ol": true, "li": true,
	"strong": true, "em": true, "br": true,
}

// SanitizeHTML removes dangerous content and allows only safe structural tags.
// Order of operations:
//  1. Remove <script>…</script> and <style>…</style> with their content
//     (case-insensitive; unclosed → truncate at opening tag).
//  2. Single pass: keep allowed tags stripped of attributes; normalise <br/> → <br>;
//     unknown tags are dropped but their inner text is kept.
func SanitizeHTML(s string) string {
	// Step 1: strip script/style blocks with content.
	s = removeBlock(s, "script")
	s = removeBlock(s, "style")

	// Step 2: single-pass tag rewriting.
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] != '<' {
			b.WriteByte(s[i])
			i++
			continue
		}
		// Find closing '>'.
		end := strings.IndexByte(s[i:], '>')
		if end < 0 {
			// Unclosed tag: drop rest.
			break
		}
		raw := s[i+1 : i+end] // content between < and >
		i += end + 1

		selfClose := strings.HasSuffix(raw, "/")
		inner := strings.TrimSuffix(raw, "/")
		inner = strings.TrimSpace(inner)

		closing := strings.HasPrefix(inner, "/")
		name := strings.TrimPrefix(inner, "/")
		if idx := strings.IndexAny(name, " \t\n\r/"); idx >= 0 {
			name = name[:idx]
		}
		name = strings.ToLower(strings.TrimSpace(name))

		if name == "" {
			continue
		}
		if !allowedTags[name] {
			// Drop tag, keep text content (handled by outer loop).
			continue
		}
		// Normalise br: always emit <br> (no slash, no attributes).
		if name == "br" {
			b.WriteString("<br>")
			_ = selfClose
			continue
		}
		if closing {
			b.WriteString("</")
			b.WriteString(name)
			b.WriteString(">")
		} else {
			b.WriteString("<")
			b.WriteString(name)
			b.WriteString(">")
		}
	}
	return b.String()
}

// removeBlock removes <tag>…</tag> blocks (case-insensitive) including their
// content. If the closing tag is missing, everything from the opening tag to
// end-of-string is removed.
func removeBlock(s, tag string) string {
	openLower := "<" + tag
	openUpper := "<" + strings.ToUpper(tag)
	closeTag := "</" + tag + ">"
	closeTagUpper := "</" + strings.ToUpper(tag) + ">"

	var b strings.Builder
	b.Grow(len(s))
	for {
		// Find next opening tag (case-insensitive search).
		startLow := indexFold(s, openLower)
		startUp := indexFold(s, openUpper)
		start := -1
		if startLow >= 0 && (startUp < 0 || startLow <= startUp) {
			start = startLow
		} else if startUp >= 0 {
			start = startUp
		}
		if start < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:start])
		s = s[start:]

		// Find end of opening tag.
		gtIdx := strings.IndexByte(s, '>')
		if gtIdx < 0 {
			// Malformed: drop rest.
			break
		}
		s = s[gtIdx+1:]

		// Find closing tag (case-insensitive).
		endIdx := indexFold(s, closeTag)
		endIdxUp := indexFold(s, closeTagUpper)
		end := -1
		if endIdx >= 0 && (endIdxUp < 0 || endIdx <= endIdxUp) {
			end = endIdx
			s = s[end+len(closeTag):]
		} else if endIdxUp >= 0 {
			end = endIdxUp
			s = s[end+len(closeTagUpper):]
		} else {
			// No closing tag: truncate.
			break
		}
	}
	return b.String()
}

// indexFold is a simple case-folded substring search for ASCII tag names.
func indexFold(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	sLow := strings.ToLower(s)
	subLow := strings.ToLower(sub)
	return strings.Index(sLow, subLow)
}

// ValidateShape checks that after is structurally valid for the given field.
func ValidateShape(field, before, after string) error {
	limit := 4000
	if len(after) > limit {
		return fmt.Errorf("%w: output exceeds %d bytes", ErrUnsafe, limit)
	}
	if len(after) > 3*len(before)+500 {
		return fmt.Errorf("%w: output is more than 3× original length", ErrUnsafe)
	}
	sanitized := SanitizeHTML(after)
	switch field {
	case "bullets":
		if !strings.Contains(sanitized, "<ul>") && !strings.Contains(sanitized, "<ol>") {
			return fmt.Errorf("%w: bullets field must contain <ul> or <ol>", ErrUnsafe)
		}
		if !strings.Contains(sanitized, "<li>") {
			return fmt.Errorf("%w: bullets field must contain at least one <li>", ErrUnsafe)
		}
	default: // summary, description
		plain := strings.TrimSpace(stripTags(sanitized))
		if plain == "" {
			return fmt.Errorf("%w: field must contain non-empty text", ErrUnsafe)
		}
	}
	return nil
}
