// Package quote decides whether a quoted sentence still appears in a page.
//
// Matching is done on a folded form: lower case, letters and digits only.
// That ignores curly vs straight quotes, dashes, non-breaking spaces and
// the spacing a page's markup leaves between inline tags, while still
// requiring every word and number to be there in the same order.
package quote

import (
	"strings"
	"unicode"
)

// Fold reduces s to its lower-case letters and digits.
func Fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// Index holds a folded page so many quotes can be checked against it.
type Index struct{ folded string }

func NewIndex(pageText string) Index { return Index{Fold(pageText)} }

// Contains reports whether q appears in the page. A quote may skip text
// with "..." or "…"; the parts must then appear in order.
func (ix Index) Contains(q string) bool {
	parts := splitEllipsis(q)
	pos := 0
	for _, p := range parts {
		f := Fold(p)
		if f == "" {
			continue
		}
		i := strings.Index(ix.folded[pos:], f)
		if i < 0 {
			return false
		}
		pos += i + len(f)
	}
	return true
}

// Closest returns how much of q (in folded characters, from the start) can
// be found in the page. It helps tell "the page changed one word" from "the
// page is gone".
func (ix Index) Closest(q string) (matched, total int) {
	f := Fold(q)
	total = len(f)
	lo, hi := 0, len(f)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if strings.Contains(ix.folded, f[:mid]) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo, total
}

func splitEllipsis(q string) []string {
	q = strings.ReplaceAll(q, "…", "...")
	return strings.Split(q, "...")
}
