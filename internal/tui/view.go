package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width <= 0 {
		m.width = 80
	}
	var b strings.Builder
	b.WriteString(m.renderTabs())
	b.WriteString("\n")

	if m.browse.active {
		b.WriteString(m.viewBrowse())
		return b.String()
	}
	if m.sshBulkOpen {
		b.WriteString(titleStyle.Render("Несколько SSH (сохранится в ssh_targets)") + "\n")
		b.WriteString(m.sshBulkTA.View() + "\n")
		if m.sshBulkErr != "" {
			b.WriteString(errStyle.Render(m.sshBulkErr) + "\n")
		}
		b.WriteString(mutedStyle.Render("ctrl+s: сохранить  esc: отмена  пароль: TAB после user@host  удалите строку — хост пропадёт") + "\n")
		return b.String()
	}
	if m.cloakBulkOpen {
		b.WriteString(titleStyle.Render("Клоаки: правка списка (как SSH)") + "\n")
		b.WriteString(m.cloakBulkTA.View() + "\n")
		if m.cloakBulkErr != "" {
			b.WriteString(errStyle.Render(m.cloakBulkErr) + "\n")
		}
		b.WriteString(mutedStyle.Render("ctrl+s: удалить строки которых нет в списке  esc: отмена") + "\n")
		return b.String()
	}
	if m.entryNameEdit {
		b.WriteString(titleStyle.Render("Entry filename (relative to DOCUMENT_ROOT)") + "\n")
		b.WriteString(m.entryNameInp.View() + "\n")
		b.WriteString(mutedStyle.Render("enter: save  esc: cancel") + "\n")
		return b.String()
	}
	if m.tagEditSite != "" {
		b.WriteString(titleStyle.Render("Tags for "+m.tagEditSite) + "\n")
		b.WriteString(m.tagInp.View() + "\n")
		b.WriteString(mutedStyle.Render("comma-separated, enter: save  esc: cancel") + "\n")
		return b.String()
	}
	if m.addStep != 0 {
		switch m.addStep {
		case 1:
			b.WriteString(titleStyle.Render("Клоака из файла — путь к .php") + "\n")
			b.WriteString(m.addPathInp.View() + "\n")
			b.WriteString(mutedStyle.Render("enter: далее  f: обзор файла  esc: отмена") + "\n")
		case 4:
			b.WriteString(titleStyle.Render("Быстрый stub — только имя (<?php)") + "\n")
			b.WriteString(m.addLabelInp.View() + "\n")
			b.WriteString(mutedStyle.Render("enter: создать  esc: отмена") + "\n")
		case 5:
			b.WriteString(titleStyle.Render("Новая клоака вручную — имя") + "\n")
			b.WriteString(m.addLabelInp.View() + "\n")
			b.WriteString(mutedStyle.Render("enter: редактор кода  esc: отмена") + "\n")
		case 6:
			b.WriteString(titleStyle.Render("Код PHP (после имени)") + "\n")
			b.WriteString(m.cloakCodeTA.View() + "\n")
			b.WriteString(mutedStyle.Render("ctrl+s: сохранить в библиотеку  esc: отмена") + "\n")
		default:
			b.WriteString(titleStyle.Render("Клоака — метка для файла") + "\n")
			b.WriteString(m.addLabelInp.View() + "\n")
			b.WriteString(mutedStyle.Render("enter: импорт  esc: отмена") + "\n")
		}
		return b.String()
	}

	switch m.scr {
	case scDomains:
		b.WriteString(m.viewDomains())
	case scCloaks:
		b.WriteString(m.viewCloaks())
	case scSettings:
		b.WriteString(m.viewSettings())
	case scRun:
		b.WriteString(m.viewRun())
	}
	return b.String()
}

func (m Model) renderTabs() string {
	names := []string{"Domains", "Cloaks", "Settings", "Run"}
	var parts []string
	for i, n := range names {
		st := mutedStyle
		if screen(i) == m.scr {
			st = titleStyle
		}
		parts = append(parts, st.Render(fmt.Sprintf("[%d] %s", i+1, n)))
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

func (m Model) viewDomains() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Domains (FastPanel)") + "\n")
	if m.loading {
		b.WriteString("Loading sites…\n")
		return b.String()
	}
	if m.sitesErr != "" {
		b.WriteString(errStyle.Render(m.sitesErr) + "\n")
	}
	if m.sitesLoadWarn != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("Часть панелей недоступна:\n"+m.sitesLoadWarn) + "\n")
	}
	if m.tagFilter != "" {
		b.WriteString(mutedStyle.Render("filter tag: "+m.tagFilter) + "\n")
	}
	b.WriteString(fmt.Sprintf("Entry file: %s  ", m.entryName))
	b.WriteString(mutedStyle.Render("(i) edit") + "\n")

	list := m.filteredSites()
	if len(list) == 0 {
		b.WriteString(mutedStyle.Render("No sites — set SSH in Settings, then (r) refresh.") + "\n")
	} else {
		for i, s := range list {
			c := " "
			idx := m.realIndex(list, i)
			if _, ok := m.selected[idx]; ok {
				c = "x"
			}
			cur := " "
			if i == m.cursor {
				cur = ">"
			}
			tags := strings.Join(m.cfg.TagsFor(s.ServerName), ",")
			hostTag := ""
			if s.SSHLabel != "" {
				hostTag = "[" + s.SSHLabel + "] "
			}
			line := fmt.Sprintf("%s [%s] %d  %s%-28s  %s", cur, c, s.ID, hostTag, s.ServerName, tags)
			if i == m.cursor {
				b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Render(line))
			} else {
				b.WriteString(line)
			}
			b.WriteString("\n")
			if i == m.cursor {
				b.WriteString(mutedStyle.Render("  "+s.DocumentRoot) + "\n")
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("↑↓ PgUp/PgDn: move  space: toggle  a: all  A: invert  r: refresh  t: tag  g: tags  i: entry file  ←/→: tab  n: Run") + "\n")
	return b.String()
}

func (m Model) viewCloaks() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Cloak library") + "\n")
	if m.vault == nil {
		b.WriteString(errStyle.Render("vault not configured") + "\n")
		return b.String()
	}
	if len(m.cloakList) == 0 {
		b.WriteString(mutedStyle.Render("No cloaks — press n, path, label") + "\n")
	} else {
		for i, e := range m.cloakList {
			cur := " "
			if i == m.cloakCursor {
				cur = ">"
			}
			sel := " "
			if e.ID == m.cloakSelID {
				sel = "*"
			}
			line := fmt.Sprintf("%s [%s] %-20s  %s", cur, sel, e.Label, e.RelPath)
			if i == m.cloakCursor {
				b.WriteString(lipgloss.NewStyle().Bold(true).Render(line))
			} else {
				b.WriteString(line)
			}
			b.WriteString("\n")
		}
	}
	b.WriteString(mutedStyle.Render("↑↓ выбор  enter/space: активная  n: вручную (имя→код)  f: из файла  t: пустой <?php  b: список/удалить  d: удалить строку  ←/→: tab") + "\n")
	return b.String()
}

func (m Model) viewSettings() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Settings") + "\n")
	b.WriteString("h) " + m.hostInp.View() + "\n")
	b.WriteString("u) " + m.userInp.View() + "\n")
	b.WriteString("w) " + m.passInp.View() + "\n")
	b.WriteString("p) " + m.portInp.View() + "\n")
	b.WriteString("i) SSH key " + m.identityInp.View() + "\n")
	b.WriteString("k) known_hosts " + m.knownHostsInp.View() + "\n")
	b.WriteString("e) default entry " + m.entryDef.View() + "\n")
	b.WriteString("m) mogwai " + m.mogwaiIn.View() + "\n")
	b.WriteString(okStyle.Render("ctrl+s: save to "+m.cfgPath) + "\n")
	b.WriteString(mutedStyle.Render("↑↓/Shift+Tab: fields  h/u/w/p/i/k/e/m: focus  b: пачка SSH  y/o: browse  esc: blur  ctrl+s: save  пароль в YAML открытым текстом; можно ELSABO_SSH_PASSWORD") + "\n")
	return b.String()
}

func (m Model) viewRun() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Replace job") + "\n")
	nSel := len(m.selected)
	b.WriteString(fmt.Sprintf("Selected: %d domains  |  active cloak id: %s\n", nSel, m.cloakSelID))
	b.WriteString(fmt.Sprintf("Progress: %d / %d  failures: %d\n", m.jobIndex, m.jobTotal, m.jobFailCount))
	if m.jobRunning {
		b.WriteString(okStyle.Render("Working…") + "\n")
	}
	if len(m.jobLog) > 0 {
		b.WriteString("\nLast results:\n")
		start := len(m.jobLog) - 8
		if start < 0 {
			start = 0
		}
		for _, r := range m.jobLog[start:] {
			st := okStyle
			if !r.OK {
				st = errStyle
			}
			host := r.Site.ServerName
			if host == "" {
				host = "—"
			}
			extra := r.Verify.Reason
			if extra != "" {
				extra = " [" + extra + "]"
			}
			b.WriteString(st.Render(fmt.Sprintf("- %s: %s%s", host, r.Message, extra)) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("enter: start sequential replace (one SSH session per site)") + "\n")
	return b.String()
}
