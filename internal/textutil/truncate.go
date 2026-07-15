// Package textutil provides shared, UTF-8-safe text bounding helpers used by
// plugins, the downstream MCP client, and result/prompt shaping. Slicing a
// string at a raw byte offset can split a multi-byte rune and hand invalid
// UTF-8 to json.Marshal (which rewrites it to U+FFFD); every truncation in
// Prism should go through CutBytes instead.
package textutil

import "unicode/utf8"

// CutBytes returns s truncated to at most limit bytes without splitting a
// UTF-8 rune, plus whether truncation occurred. limit <= 0 disables
// truncation.
func CutBytes(s string, limit int) (string, bool) {
	if limit <= 0 || len(s) <= limit {
		return s, false
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut], true
}

// Truncate bounds s to limit bytes, appending suffix when content was
// removed. The suffix does not count against the limit so callers keep their
// existing size contracts for the payload portion. For a hard byte budget
// where the total output must not exceed limit, use TruncateWithin.
func Truncate(s string, limit int, suffix string) string {
	out, cut := CutBytes(s, limit)
	if !cut {
		return out
	}
	return out + suffix
}

// TruncateWithin bounds the total output — payload plus suffix — to limit
// bytes. Use it to enforce hard budgets (e.g. ToolSpec.MaxBytes) where even
// the truncation marker must not overflow the limit. When limit is too small
// to fit the suffix, the payload is hard-cut without a marker.
func TruncateWithin(s string, limit int, suffix string) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	if limit <= len(suffix) {
		out, _ := CutBytes(s, limit)
		return out
	}
	out, _ := CutBytes(s, limit-len(suffix))
	return out + suffix
}
