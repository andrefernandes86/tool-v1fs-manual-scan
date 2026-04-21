package scanner

import (
	"strings"
	"unicode"
)

const (
	maxUserScanTags  = 32
	maxUserScanTagLen = 128
)

// ParseUserScanTags trims, deduplicates, enforces limits, and drops empty entries.
// Tags are passed to the File Security SDK as-is (after trimming); avoid control characters.
func ParseUserScanTags(in []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, raw := range in {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		if strings.ContainsFunc(t, unicode.IsControl) {
			continue
		}
		if len(t) > maxUserScanTagLen {
			t = t[:maxUserScanTagLen]
			t = strings.TrimRight(t, " ")
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
		if len(out) >= maxUserScanTags {
			break
		}
	}
	return out
}
