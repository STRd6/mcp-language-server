package glob_test

import (
	"testing"

	"github.com/STRd6/mcp-language-server/internal/glob"
)

// Patterns should print back as themselves after parsing — this exercises
// every element type's String() (slash, literal, star, anyChar, starStar,
// group, charRange) and Glob.String().
func TestGlob_StringRoundTrip(t *testing.T) {
	patterns := []string{
		"abc",
		"*",
		"**",
		"?",
		"/",
		"a/b/c",
		"foo*bar",
		"foo?bar",
		"**/*.go",
		"**/*.{ts,js}",
		"[a-z]",
		"prefix-[0-9]-suffix",
		"{foo,bar}",
		"a{b,c}d",
	}
	for _, p := range patterns {
		g, err := glob.Parse(p)
		if err != nil {
			t.Fatalf("Parse(%q): %v", p, err)
		}
		if got := g.String(); got != p {
			t.Errorf("Parse(%q).String() = %q, want %q", p, got, p)
		}
	}
}
