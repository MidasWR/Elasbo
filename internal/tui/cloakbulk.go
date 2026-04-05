package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/midaswr/elsabo/internal/cloaks"
)

func newCloakCodeTextArea() textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "<?php\n// код клоаки"
	ta.SetHeight(14)
	ta.SetWidth(60)
	ta.ShowLineNumbers = true
	ta.Blur()
	return ta
}

func newCloakBulkTextArea() textarea.Model {
	ta := textarea.New()
	ta.SetHeight(12)
	ta.SetWidth(60)
	ta.ShowLineNumbers = false
	ta.Blur()
	return ta
}

func (m *Model) openCloakBulkEditor() {
	m.cloakBulkErr = ""
	if m.vault == nil {
		m.cloakBulkErr = "vault not configured"
		m.cloakBulkTA.SetValue("")
		m.cloakBulkOpen = true
		return
	}
	list, err := m.vault.List()
	if err != nil {
		m.cloakBulkErr = err.Error()
		m.cloakBulkTA.SetValue("")
		m.cloakBulkOpen = true
		return
	}
	sort.Slice(list, func(i, j int) bool {
		return strings.ToLower(list[i].Label) < strings.ToLower(list[j].Label)
	})
	m.cloakBulkTA.SetValue(cloaks.FormatBulkEdit(list))
	if m.width > 0 {
		m.cloakBulkTA.SetWidth(max(20, m.width-8))
	}
	m.cloakBulkTA.Focus()
	m.cloakBulkOpen = true
}

func (m *Model) updateCloakBulk(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.vault == nil {
		if msg.String() == "esc" {
			m.cloakBulkOpen = false
			m.cloakBulkErr = ""
			m.cloakBulkTA.Blur()
		}
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.cloakBulkOpen = false
		m.cloakBulkErr = ""
		m.cloakBulkTA.Blur()
		return m, nil
	case "ctrl+s":
		ids := cloaks.ParseBulkIDs(m.cloakBulkTA.Value())
		if err := m.vault.KeepOnlyIDs(ids); err != nil {
			m.cloakBulkErr = err.Error()
			return m, nil
		}
		m.cloakBulkOpen = false
		m.cloakBulkErr = ""
		m.cloakBulkTA.Blur()
		return m, m.loadCloaksCmd()
	}
	var cmd tea.Cmd
	m.cloakBulkTA, cmd = m.cloakBulkTA.Update(msg)
	return m, cmd
}
