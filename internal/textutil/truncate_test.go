package textutil

import (
	"testing"
	"unicode/utf8"
)

func TestCutBytesRuneSafe(t *testing.T) {
	// "héllo" — é is 2 bytes; cutting at byte 2 would split it.
	s := "héllo"
	out, cut := CutBytes(s, 2)
	if !cut {
		t.Fatal("expected truncation")
	}
	if out != "h" {
		t.Fatalf("got %q, want %q", out, "h")
	}
	if !utf8.ValidString(out) {
		t.Fatalf("output is not valid UTF-8: %q", out)
	}
}

func TestCutBytesNoTruncation(t *testing.T) {
	if out, cut := CutBytes("abc", 10); cut || out != "abc" {
		t.Fatalf("got %q cut=%v", out, cut)
	}
	if out, cut := CutBytes("abc", 0); cut || out != "abc" {
		t.Fatalf("limit 0 should disable truncation, got %q cut=%v", out, cut)
	}
}

func TestCutBytesMultiByteHeavy(t *testing.T) {
	s := "日本語テキスト" // 3 bytes per rune
	for limit := 1; limit <= len(s); limit++ {
		out, _ := CutBytes(s, limit)
		if !utf8.ValidString(out) {
			t.Fatalf("limit %d produced invalid UTF-8: %q", limit, out)
		}
		if len(out) > limit {
			t.Fatalf("limit %d produced %d bytes", limit, len(out))
		}
	}
}

func TestTruncateSuffix(t *testing.T) {
	if got := Truncate("abcdef", 3, "..."); got != "abc..." {
		t.Fatalf("got %q", got)
	}
	if got := Truncate("ab", 3, "..."); got != "ab" {
		t.Fatalf("got %q", got)
	}
}
