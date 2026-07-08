package service

import (
	"errors"
	"strings"
	"testing"
)

func TestNumericFacts_Normalization(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string // normalized tokens that must be present
	}{
		{"comma stripped", "1,200,000", []string{"1200000"}},
		{"percent kept", "50%", []string{"50%"}},
		{"currency lowercase", "$5M", []string{"$5m"}},
		{"k suffix", "10k", []string{"10k"}},
		{"K suffix normalized", "10K", []string{"10k"}},
		{"M suffix", "3M", []string{"3m"}},
		{"B suffix", "1B", []string{"1b"}},
		{"x suffix", "2x", []string{"2x"}},
		{"decimal", "1.5", []string{"1.5"}},
		{"HTML stripped input", "<li>100</li><li>200</li>", []string{"100", "200"}},
		{"no numbers", "grew the team significantly", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NumericFacts(tt.input)
			for _, w := range tt.want {
				if _, ok := got[w]; !ok {
					t.Errorf("NumericFacts(%q) missing %q; got %v", tt.input, w, got)
				}
			}
		})
	}
}

func TestCheckNoNewNumbers(t *testing.T) {
	t.Run("year added is rejected", func(t *testing.T) {
		before := "Grew the team"
		after := "Grew the team by 2021"
		err := CheckNoNewNumbers(before, after, nil)
		if err == nil {
			t.Fatal("expected error for new year token")
		}
		if !errors.Is(err, ErrUnsafe) {
			t.Errorf("expected ErrUnsafe, got %v", err)
		}
	})

	t.Run("same numbers reordered pass", func(t *testing.T) {
		before := "<li>200 req/s</li><li>100ms latency</li>"
		after := "<li>100ms latency</li><li>200 req/s</li>"
		if err := CheckNoNewNumbers(before, after, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("adjacent li elements facts separated", func(t *testing.T) {
		before := "<ul><li>100</li><li>200</li></ul>"
		after := "<ul><li>200</li><li>100</li></ul>"
		if err := CheckNoNewNumbers(before, after, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestSanitizeHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantOut  string
		mustHave []string
		mustNot  []string
	}{
		{
			name:     "script removed with content",
			input:    `<p>Hello</p><script>alert(1)</script><p>World</p>`,
			mustNot:  []string{"script", "alert"},
			mustHave: []string{"<p>Hello</p>", "<p>World</p>"},
		},
		{
			name:     "style removed with content",
			input:    `<style>body{color:red}</style><p>text</p>`,
			mustNot:  []string{"style", "color:red"},
			mustHave: []string{"<p>text</p>"},
		},
		{
			name:     "li set preserved",
			input:    `<ul><li>item1</li><li>item2</li></ul>`,
			mustHave: []string{"<ul>", "<li>item1</li>", "<li>item2</li>", "</ul>"},
		},
		{
			name:     "attributes stripped",
			input:    `<p class="foo" style="color:red">text</p>`,
			mustNot:  []string{"class", "style", "color"},
			mustHave: []string{"<p>text</p>"},
		},
		{
			name:     "unknown tag drops keeping inner text",
			input:    `<div>inner text</div>`,
			mustHave: []string{"inner text"},
			mustNot:  []string{"<div>", "</div>"},
		},
		{
			name:     "br normalized",
			input:    `line1<br/>line2`,
			mustHave: []string{"<br>"},
			mustNot:  []string{"<br/>"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeHTML(tt.input)
			for _, want := range tt.mustHave {
				if !strings.Contains(got, want) {
					t.Errorf("SanitizeHTML(%q) = %q; missing %q", tt.input, got, want)
				}
			}
			for _, notWant := range tt.mustNot {
				if strings.Contains(got, notWant) {
					t.Errorf("SanitizeHTML(%q) = %q; should not contain %q", tt.input, got, notWant)
				}
			}
		})
	}
}

func TestValidateShape(t *testing.T) {
	t.Run("oversize >4000 rejected", func(t *testing.T) {
		after := strings.Repeat("a", 4001)
		err := ValidateShape("summary", "original", after)
		if err == nil || !errors.Is(err, ErrUnsafe) {
			t.Errorf("expected ErrUnsafe for oversize, got %v", err)
		}
	})

	t.Run(">3x before+500 rejected", func(t *testing.T) {
		before := "short"
		after := strings.Repeat("a", 3*len(before)+501)
		err := ValidateShape("summary", before, after)
		if err == nil || !errors.Is(err, ErrUnsafe) {
			t.Errorf("expected ErrUnsafe for >3x, got %v", err)
		}
	})

	t.Run("bullets returned as p tag fails", func(t *testing.T) {
		err := ValidateShape("bullets", "<ul><li>original</li></ul>", "<p>rewritten</p>")
		if err == nil || !errors.Is(err, ErrUnsafe) {
			t.Errorf("expected ErrUnsafe for bullets without list, got %v", err)
		}
	})

	t.Run("summary empty text fails", func(t *testing.T) {
		err := ValidateShape("summary", "original text here", "<p></p>")
		if err == nil || !errors.Is(err, ErrUnsafe) {
			t.Errorf("expected ErrUnsafe for empty summary text, got %v", err)
		}
	})

	t.Run("valid bullets passes", func(t *testing.T) {
		before := "<ul><li>built X</li></ul>"
		after := "<ul><li>built X with 10 engineers</li></ul>"
		// after has "10" which is new, but ValidateShape doesn't check numbers
		err := ValidateShape("bullets", before, after)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("valid summary passes", func(t *testing.T) {
		err := ValidateShape("summary", "original summary text", "<p>rewritten summary text</p>")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
