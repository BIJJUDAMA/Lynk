package sanitizer

import "testing"

func TestSanitizeText_StripsScriptAndDangerousTags(t *testing.T) {
	input := "Clean text <script>alert('xss')</script><a href='javascript:evil()'>Click</a>"
	expected := "Clean text Click"

	clean := SanitizeStrict(input)
	if clean != expected {
		t.Fatalf("expected %q, got %q", expected, clean)
	}
}

func TestSanitizeText_PlainTextPassThrough(t *testing.T) {
	input := "Full-Stack Developer with React and Python skills."
	clean := SanitizeStrict(input)
	if clean != input {
		t.Fatalf("expected plain text unchanged, got %q", clean)
	}
}

func TestSanitizeText_EmptyInput(t *testing.T) {
	if got := SanitizeStrict(""); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestSanitizeSingleLine_RemovesNewlines(t *testing.T) {
	input := "Line one\nLine two"
	got := SanitizeSingleLine(input)
	if got != "Line one Line two" {
		t.Fatalf("expected 'Line one Line two', got %q", got)
	}
}

func TestSanitizeStrict_UnclosedDangerousTag(t *testing.T) {
	input := "Hello <script>alert('xss')"
	expected := "Hello"
	clean := SanitizeStrict(input)
	if clean != expected {
		t.Fatalf("expected %q, got %q", expected, clean)
	}

	onlyScript := "<script>alert('evil')"
	if got := SanitizeStrict(onlyScript); got != "" {
		t.Fatalf("expected empty string for unclosed script, got %q", got)
	}

	iframe := "Safe text <iframe src='http://evil.com'>some nested content"
	if got := SanitizeStrict(iframe); got != "Safe text" {
		t.Fatalf("expected 'Safe text', got %q", got)
	}
}
