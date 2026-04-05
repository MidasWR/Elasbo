package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func keyIsUp(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp, tea.KeyPgUp:
		return true
	}
	return msg.String() == "k"
}

func keyIsDown(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyDown, tea.KeyPgDown:
		return true
	}
	return msg.String() == "j"
}

func keyArrowLeft(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyLeft
}

func keyShiftTab(msg tea.KeyMsg) bool {
	return msg.String() == "shift+tab"
}

func keyArrowRight(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyRight
}

func (m *Model) settingsAnyFocused() bool {
	return m.hostInp.Focused() ||
		m.userInp.Focused() ||
		m.passInp.Focused() ||
		m.portInp.Focused() ||
		m.identityInp.Focused() ||
		m.knownHostsInp.Focused() ||
		m.entryDef.Focused() ||
		m.mogwaiIn.Focused()
}

func (m *Model) settingsCurrentField() int {
	switch {
	case m.hostInp.Focused():
		return 0
	case m.userInp.Focused():
		return 1
	case m.passInp.Focused():
		return 2
	case m.portInp.Focused():
		return 3
	case m.identityInp.Focused():
		return 4
	case m.knownHostsInp.Focused():
		return 5
	case m.entryDef.Focused():
		return 6
	case m.mogwaiIn.Focused():
		return 7
	default:
		return m.settingsLooseIdx
	}
}

func (m *Model) blurAllSettings() {
	m.hostInp.Blur()
	m.userInp.Blur()
	m.passInp.Blur()
	m.portInp.Blur()
	m.identityInp.Blur()
	m.knownHostsInp.Blur()
	m.entryDef.Blur()
	m.mogwaiIn.Blur()
}

const settingsFieldCount = 8

func (m *Model) focusSettingsField(i int) tea.Cmd {
	i = (i%settingsFieldCount + settingsFieldCount) % settingsFieldCount
	m.settingsLooseIdx = i
	m.blurAllSettings()
	switch i {
	case 0:
		m.hostInp.Focus()
	case 1:
		m.userInp.Focus()
	case 2:
		m.passInp.Focus()
	case 3:
		m.portInp.Focus()
	case 4:
		m.identityInp.Focus()
	case 5:
		m.knownHostsInp.Focus()
	case 6:
		m.entryDef.Focus()
	case 7:
		m.mogwaiIn.Focus()
	}
	return textinput.Blink
}
