package fastpanel

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

var rootRx = regexp.MustCompile(`\s+(/[^\s]+)\s*$`)

// Site is one row from mogwai sites list.
type Site struct {
	ID           int
	ServerName   string
	DocumentRoot string
	RawLine      string
	// SSH* are set when aggregating several panels (which host this site came from).
	SSHLabel    string
	SSHHost     string
	SSHUser     string
	SSHPort     int
	SSHPassword string // per-panel password from ssh_targets (optional)
}

// ParseSitesList parses CLI output of `mogwai sites list`.
func ParseSitesList(output string) ([]Site, error) {
	lines := strings.Split(output, "\n")
	var colIndex map[string]int
	for _, line := range lines {
		line = strings.TrimRight(line, "\r\t ")
		if line == "" {
			continue
		}
		fields := splitColumns(line)
		if isHeader(fields) {
			colIndex = map[string]int{}
			for i, h := range fields {
				colIndex[strings.ToUpper(strings.TrimSpace(h))] = i
			}
			break
		}
	}

	var sites []Site
	if colIndex != nil {
		pastHeader := false
		for _, line := range lines {
			line = strings.TrimRight(line, "\r\t ")
			if line == "" {
				continue
			}
			fields := splitColumns(line)
			if !pastHeader {
				if isHeader(fields) {
					pastHeader = true
				}
				continue
			}
			s, err := siteFromColumns(fields, colIndex)
			if err != nil {
				continue
			}
			s.RawLine = line
			sites = append(sites, s)
		}
	}
	if len(sites) == 0 {
		for _, line := range lines {
			line = strings.TrimRight(line, "\r\t ")
			if line == "" {
				continue
			}
			if strings.Contains(strings.ToUpper(line), "DOCUMENT_ROOT") {
				continue
			}
			if s, ok := parseHeuristicLine(line); ok {
				s.RawLine = line
				sites = append(sites, s)
			}
		}
	}
	if len(sites) == 0 {
		return nil, fmt.Errorf("no sites parsed from mogwai output")
	}
	return sites, nil
}

func splitColumns(line string) []string {
	parts := strings.Split(line, "\t")
	if len(parts) > 1 {
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			out = append(out, strings.TrimSpace(p))
		}
		return out
	}
	rx := regexp.MustCompile(` {2,}`)
	raw := rx.Split(strings.TrimSpace(line), -1)
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}

func isHeader(fields []string) bool {
	for _, f := range fields {
		u := strings.ToUpper(f)
		if strings.Contains(u, "DOCUMENT_ROOT") || strings.Contains(u, "SERVER_NAME") {
			return true
		}
	}
	return false
}

func siteFromColumns(fields []string, colIndex map[string]int) (Site, error) {
	idIdx := pickCol(colIndex, "ID")
	nameIdx := pickCol(colIndex, "SERVER_NAME")
	rootIdx := pickCol(colIndex, "DOCUMENT_ROOT")
	if idIdx < 0 || nameIdx < 0 || rootIdx < 0 {
		return Site{}, fmt.Errorf("missing columns")
	}
	if idIdx >= len(fields) || nameIdx >= len(fields) || rootIdx >= len(fields) {
		return Site{}, fmt.Errorf("short line")
	}
	var id int
	_, err := fmt.Sscanf(fields[idIdx], "%d", &id)
	if err != nil {
		return Site{}, err
	}
	root := fields[rootIdx]
	if !strings.HasPrefix(root, "/") {
		return Site{}, fmt.Errorf("bad document root")
	}
	root = path.Clean(root)
	return Site{
		ID:           id,
		ServerName:   strings.TrimSpace(fields[nameIdx]),
		DocumentRoot: root,
	}, nil
}

func pickCol(m map[string]int, name string) int {
	if i, ok := m[name]; ok {
		return i
	}
	for k, v := range m {
		if strings.HasPrefix(k, name) {
			return v
		}
	}
	return -1
}

func parseHeuristicLine(line string) (Site, bool) {
	m := rootRx.FindStringSubmatch(line)
	if len(m) < 2 {
		return Site{}, false
	}
	root := path.Clean(strings.TrimSpace(m[1]))
	i := strings.LastIndex(strings.TrimRight(line, " \t\r"), m[1])
	if i < 0 {
		return Site{}, false
	}
	rest := strings.TrimSpace(line[:i])
	fields := strings.Fields(rest)
	if len(fields) < 2 {
		return Site{}, false
	}
	var id int
	if _, err := fmt.Sscanf(fields[0], "%d", &id); err != nil {
		return Site{}, false
	}
	return Site{
		ID:           id,
		ServerName:   fields[1],
		DocumentRoot: root,
	}, true
}
