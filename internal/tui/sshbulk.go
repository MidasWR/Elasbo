package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textarea"

	"github.com/MidasWR/Elasbo/internal/config"
)

func (m *Model) openSSHBulkEditor() {
	m.blurAllSettings()
	m.sshBulkErr = ""
	m.sshBulkTA.SetValue(config.FormatSSHBulk(m.cfg.SSHTargets))
	if m.width > 0 {
		m.sshBulkTA.SetWidth(max(20, m.width-8))
	}
	m.sshBulkTA.Focus()
	m.sshBulkOpen = true
}

func (m *Model) updateSSHBulk(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.sshBulkOpen = false
		m.sshBulkErr = ""
		m.sshBulkTA.Blur()
		return m, nil
	case "ctrl+s":
		targets, errs := config.ParseSSHBulk(m.sshBulkTA.Value())
		if len(errs) > 0 {
			var b strings.Builder
			for i, e := range errs {
				if i > 0 {
					b.WriteString("; ")
				}
				b.WriteString(e.Error())
			}
			m.sshBulkErr = b.String()
			return m, nil
		}
		if m.cfg != nil {
			m.cfg.SSHTargets = targets
			_ = config.Save(m.cfgPath, m.cfg)
		}
		m.sshBulkOpen = false
		m.sshBulkErr = ""
		m.sshBulkTA.Blur()
		m.loading = true
		m.sitesErr = ""
		return m, m.loadSitesCmd()
	}
	var cmd tea.Cmd
	m.sshBulkTA, cmd = m.sshBulkTA.Update(msg)
	return m, cmd
}

func newSSHBulkTextArea() textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "# user@host[:port] optional TAB password\n# root@10.0.0.1\n# deploy@panel:2222\tsecret"
	ta.SetHeight(12)
	ta.SetWidth(60)
	ta.ShowLineNumbers = false
	ta.Blur()
	return ta
}
