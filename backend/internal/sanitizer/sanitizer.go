package sanitizer

import (
	"strings"
	"unicode/utf8"
)

// dangerousBlockTags are tags whose inner content must also be stripped.
var dangerousBlockTags = map[string]bool{
	"script": true,
	"style":  true,
	"iframe": true,
	"object": true,
	"embed":  true,
}

// SanitizeStrict strips all HTML tags and dangerous content from input,
// returning plain text safe for storage and display.
func SanitizeStrict(input string) string {
	if !utf8.ValidString(input) {
		return ""
	}

	var b strings.Builder
	b.Grow(len(input))

	i := 0
	for i < len(input) {
		if input[i] != '<' {
			r, size := utf8.DecodeRuneInString(input[i:])
			if r == utf8.RuneError && size == 1 {
				i++
				continue
			}
			b.WriteRune(r)
			i += size
			continue
		}

		// We are at '<' - extract the tag name.
		end := strings.IndexByte(input[i:], '>')
		if end == -1 {
			// Unclosed tag: skip the '<' and continue.
			i++
			continue
		}
		tagContent := input[i+1 : i+end] // content between < and >
		i += end + 1                      // advance past '>'

		// Determine tag name (strip leading slash for closing tags, attributes etc.)
		tagName := strings.ToLower(strings.TrimSpace(tagContent))
		tagName = strings.TrimPrefix(tagName, "/")
		if spaceIdx := strings.IndexAny(tagName, " \t\r\n/"); spaceIdx >= 0 {
			tagName = tagName[:spaceIdx]
		}

		if dangerousBlockTags[tagName] {
			// Also consume everything up to the matching closing tag.
			closeTag := "</" + tagName
			closeIdx := strings.Index(strings.ToLower(input[i:]), closeTag)
			if closeIdx >= 0 {
				// Skip past the closing tag entirely.
				rest := input[i+closeIdx:]
				closeEnd := strings.IndexByte(rest, '>')
				if closeEnd >= 0 {
					i += closeIdx + closeEnd + 1
				} else {
					i += closeIdx + len(closeTag)
				}
			} else {
				i = len(input)
			}
		}
		// For all other tags: tag brackets stripped, inner content preserved (already past '>').
	}

	// Collapse multiple spaces into single space and trim.
	result := strings.TrimSpace(b.String())
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	return result
}


// SanitizeSingleLine strips HTML and normalizes to a single line (no newlines).
func SanitizeSingleLine(input string) string {
	clean := SanitizeStrict(input)
	clean = strings.ReplaceAll(clean, "\n", " ")
	clean = strings.ReplaceAll(clean, "\r", " ")
	for strings.Contains(clean, "  ") {
		clean = strings.ReplaceAll(clean, "  ", " ")
	}
	return strings.TrimSpace(clean)
}
