package cloaks

import (
	"regexp"
	"strings"
)

var lineUUID = regexp.MustCompile(`(?i)^\s*([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`)

// FormatBulkEdit renders manifest entries for the bulk editor (delete lines to remove).
func FormatBulkEdit(entries []Entry) string {
	var b strings.Builder
	b.WriteString("# Удалите строку — клоак пропадёт после ctrl+s\n")
	for _, e := range entries {
		b.WriteString(e.ID)
		if e.Label != "" {
			b.WriteString("\t")
			b.WriteString(e.Label)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// ParseBulkIDs extracts entry IDs from editor lines (# and blank skipped).
func ParseBulkIDs(text string) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := lineUUID.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		id := m[1]
		low := strings.ToLower(id)
		if _, ok := seen[low]; ok {
			continue
		}
		seen[low] = struct{}{}
		out = append(out, id)
	}
	return out
}
