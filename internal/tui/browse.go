package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type browseKind string

const browseKindPickFile browseKind = "file"

type browseState struct {
	active  bool
	kind    browseKind
	root    string
	dir     string
	sel     int
	entries []browseEntry
	filter  string
	onPick  func(path string)
	onClose func()
	err     string
}

func (m *Model) openBrowse(kind browseKind, start string, onPick func(string), onClose func()) {
	if start == "" {
		if wd, err := os.Getwd(); err == nil {
			start = wd
		} else {
			start = "/"
		}
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		abs = start
	}
	st, statErr := os.Stat(abs)
	if statErr != nil || !st.IsDir() {
		abs = filepath.Dir(abs)
		if abs == "" {
			abs = "/"
		}
	}
	m.browse = browseState{
		active: true,
		kind:   kind,
		root:   abs,
		dir:    abs,
		sel:    0,
		filter: "",
		onPick: onPick,
		onClose: func() {
			if onClose != nil {
				onClose()
			}
		},
	}
	m.browse.refresh()
}

func (b *browseState) refresh() {
	b.err = ""
	ents, err := os.ReadDir(b.dir)
	if err != nil {
		b.err = err.Error()
		b.entries = nil
		return
	}
	var names []string
	for _, e := range ents {
		n := e.Name()
		if n == "." || n == ".." {
			continue
		}
		if strings.HasPrefix(n, ".") {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	var rows []browseEntry
	for _, n := range names {
		p := filepath.Join(b.dir, n)
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if st.IsDir() {
			rows = append(rows, browseEntry{name: n + string(filepath.Separator), path: p, isDir: true})
		} else {
			rows = append(rows, browseEntry{name: n, path: p, isDir: false})
		}
	}
	b.entries = rows
	if b.sel >= len(b.entries) {
		b.sel = 0
	}
	if b.sel < 0 {
		b.sel = 0
	}
}

type browseEntry struct {
	name  string
	path  string
	isDir bool
}

func (m *Model) updateBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.browse.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			close := m.browse.onClose
			m.browse = browseState{}
			if close != nil {
				close()
			}
			return m, nil
		case "up", "k":
			if m.browse.sel > 0 {
				m.browse.sel--
			}
			return m, nil
		case "down", "j":
			if m.browse.sel < len(m.browse.entries)-1 {
				m.browse.sel++
			}
			return m, nil
		case "enter", " ":
			if len(m.browse.entries) == 0 {
				return m, nil
			}
			e := m.browse.entries[m.browse.sel]
			if e.isDir {
				m.browse.dir = e.path
				m.browse.sel = 0
				m.browse.refresh()
				return m, nil
			}
			if m.browse.kind == browseKindPickFile {
				pick := m.browse.onPick
				closeFn := m.browse.onClose
				m.browse = browseState{}
				if pick != nil {
					pick(e.path)
				}
				if closeFn != nil {
					closeFn()
				}
				return m, textinput.Blink
			}
			return m, nil
		case "backspace", "h":
			parent := filepath.Dir(m.browse.dir)
			if parent != m.browse.dir && parent != "" {
				m.browse.dir = parent
				m.browse.sel = 0
				m.browse.refresh()
			}
			return m, nil
		case "r":
			m.browse.refresh()
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewBrowse() string {
	if !m.browse.active {
		return ""
	}
	b := m.browse
	var out strings.Builder
	out.WriteString(titleStyle.Render("Pick file") + "\n")
	out.WriteString(mutedStyle.Render(b.dir) + "\n")
	if b.err != "" {
		out.WriteString(errStyle.Render(b.err) + "\n")
	}
	if len(b.entries) == 0 {
		out.WriteString(mutedStyle.Render("(empty dir)") + "\n")
	} else {
		for i, e := range b.entries {
			cur := " "
			if i == b.sel {
				cur = ">"
			}
			line := fmt.Sprintf("%s %s", cur, e.name)
			if i == b.sel {
				out.WriteString(lipgloss.NewStyle().Bold(true).Render(line))
			} else {
				out.WriteString(line)
			}
			out.WriteString("\n")
		}
	}
	out.WriteString(mutedStyle.Render("↑↓ j/k: move  enter: open/select  backspace/h: parent  esc: cancel") + "\n")
	return out.String()
}
