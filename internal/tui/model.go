package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/midaswr/elsabo/internal/cloaks"
	"github.com/midaswr/elsabo/internal/config"
	"github.com/midaswr/elsabo/internal/fastpanel"
	"github.com/midaswr/elsabo/internal/replace"
	"github.com/midaswr/elsabo/internal/sshutil"
)

type screen int

const (
	scDomains screen = iota
	scCloaks
	scSettings
	scRun
)

type sitesMsg struct {
	sites []fastpanel.Site
	err   error
	warn  string // partial failures when loading from several panels
}

// progressMsg reports one finished site replacement; Next is 1-based count completed.
type progressMsg struct {
	next   int // sites completed so far
	total  int
	result replace.SiteResult
}

// Model is root TUI state.
type Model struct {
	width  int
	height int
	scr    screen

	cfg     *config.Config
	cfgPath string

	sites         []fastpanel.Site
	sitesErr      string
	sitesLoadWarn string
	loading       bool

	sshBulkOpen bool
	sshBulkTA   textarea.Model
	sshBulkErr  string

	cloakBulkOpen bool
	cloakBulkTA   textarea.Model
	cloakBulkErr  string
	cloakCodeTA   textarea.Model

	cursor      int
	selected    map[int]struct{}
	tagFilter   string
	entryName   string
	cloakCursor int
	cloakSelID  string

	vault     *cloaks.Vault
	cloakList []cloaks.Entry

	// settings fields
	hostInp       textinput.Model
	userInp       textinput.Model
	passInp       textinput.Model // ssh password (masked)
	portInp       textinput.Model
	identityInp   textinput.Model
	knownHostsInp textinput.Model
	entryDef      textinput.Model
	mogwaiIn      textinput.Model

	browse browseState

	// add-cloak path
	addPathInp  textinput.Model
	addLabelInp textinput.Model
	addStep     int // 0 idle; 1 path 2 label (file); 4 touch stub; 5 label 6 code (manual)

	// run
	jobRunning   bool
	jobIndex     int
	jobTotal     int
	jobLog       []replace.SiteResult
	jobFailCount int

	jobSites   []fastpanel.Site
	jobContent []byte
	jobEntry   string
	jobCfg     *config.Config

	tagEditSite string // non-empty → comma-tags input for this ServerName
	tagInp      textinput.Model

	entryNameEdit bool
	entryNameInp  textinput.Model

	settingsLooseIdx int // field 0..settingsFieldCount-1 when navigating Settings with arrows
}

// selectedSites returns chosen sites in stable order.
func (m *Model) selectedSites() []fastpanel.Site {
	if len(m.selected) == 0 {
		return nil
	}
	idxs := make([]int, 0, len(m.selected))
	for i := range m.sites {
		if _, ok := m.selected[i]; ok {
			idxs = append(idxs, i)
		}
	}
	sort.Ints(idxs)
	out := make([]fastpanel.Site, 0, len(idxs))
	for _, i := range idxs {
		out = append(out, m.sites[i])
	}
	return out
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// New builds initial model.
func New(cfg *config.Config, cfgPath string, vault *cloaks.Vault) *Model {
	if cfg == nil {
		d := config.Defaults()
		cfg = &d
	}
	hostInp := textinput.New()
	hostInp.Placeholder = "ssh host"
	userInp := textinput.New()
	userInp.Placeholder = "ssh user"
	passInp := textinput.New()
	passInp.Placeholder = "ssh password (optional)"
	passInp.EchoMode = textinput.EchoPassword
	passInp.EchoCharacter = '•'
	portInp := textinput.New()
	portInp.Placeholder = "22"
	identityInp := textinput.New()
	identityInp.Placeholder = "~/.ssh/id_ed25519 (optional)"
	knownHostsInp := textinput.New()
	knownHostsInp.Placeholder = "~/.ssh/known_hosts (optional)"
	entryDef := textinput.New()
	mogwaiIn := textinput.New()
	addPath := textinput.New()
	addPath.Placeholder = "/path/to/index.php"
	addLabel := textinput.New()
	addLabel.Placeholder = "label"
	tagInp := textinput.New()
	tagInp.Placeholder = "FR, US, ..."
	entryNameInp := textinput.New()
	entryNameInp.Placeholder = "index.php"

	m := Model{
		cfg:           cfg,
		cfgPath:       cfgPath,
		scr:           scDomains,
		selected:      map[int]struct{}{},
		entryName:     cfg.DefaultEntry,
		vault:         vault,
		hostInp:       hostInp,
		userInp:       userInp,
		passInp:       passInp,
		portInp:       portInp,
		identityInp:   identityInp,
		knownHostsInp: knownHostsInp,
		entryDef:      entryDef,
		mogwaiIn:      mogwaiIn,
		addPathInp:    addPath,
		addLabelInp:   addLabel,
		tagInp:        tagInp,
		entryNameInp:  entryNameInp,
		sshBulkTA:     newSSHBulkTextArea(),
		cloakBulkTA:   newCloakBulkTextArea(),
		cloakCodeTA:   newCloakCodeTextArea(),
	}
	m.syncInputsFromCfg()
	return &m
}

func (m *Model) syncInputsFromCfg() {
	if m.cfg == nil {
		return
	}
	m.hostInp.SetValue(m.cfg.SSHHost)
	m.userInp.SetValue(m.cfg.SSHUser)
	m.passInp.SetValue(m.cfg.SSHPassword)
	if m.cfg.SSHPort > 0 {
		m.portInp.SetValue(fmt.Sprintf("%d", m.cfg.SSHPort))
	}
	m.entryDef.SetValue(m.cfg.DefaultEntry)
	m.mogwaiIn.SetValue(m.cfg.MogwaiCmd)
	m.identityInp.SetValue(m.cfg.SSHIdentityFile)
	m.knownHostsInp.SetValue(m.cfg.SSHKnownHosts)
	if m.entryName == "" {
		m.entryName = m.cfg.DefaultEntry
	}
}

// workingConfigFromInputs merges saved config with current Settings form (no need for ctrl+s before refresh).
func (m *Model) workingConfigFromInputs() *config.Config {
	if m.cfg == nil {
		d := config.Defaults()
		return &d
	}
	c := *m.cfg
	c.SSHHost = strings.TrimSpace(m.hostInp.Value())
	c.SSHUser = strings.TrimSpace(m.userInp.Value())
	c.SSHPassword = m.passInp.Value()
	_, _ = fmt.Sscanf(m.portInp.Value(), "%d", &c.SSHPort)
	if c.SSHPort == 0 {
		c.SSHPort = 22
	}
	c.SSHIdentityFile = strings.TrimSpace(m.identityInp.Value())
	c.SSHKnownHosts = strings.TrimSpace(m.knownHostsInp.Value())
	c.MogwaiCmd = strings.TrimSpace(m.mogwaiIn.Value())
	if c.MogwaiCmd == "" {
		c.MogwaiCmd = "mogwai sites list"
	}
	de := strings.TrimSpace(m.entryDef.Value())
	if de != "" {
		c.DefaultEntry = de
	}
	return &c
}

func sshDialCfg(c *config.Config) sshutil.Config {
	if c == nil {
		return sshutil.Config{}
	}
	return sshutil.Config{
		Host:         c.SSHHost,
		User:         c.SSHUser,
		Port:         c.SSHPort,
		StrictHost:   c.SSHStrictHost,
		IdentityFile: strings.TrimSpace(c.SSHIdentityFile),
		KnownHosts:   strings.TrimSpace(c.SSHKnownHosts),
		Password:     strings.TrimSpace(c.SSHPassword),
	}
}

func (m *Model) setScreen(s screen) {
	m.blurAllSettings()
	m.scr = s
	if m.scr == scSettings {
		m.syncInputsFromCfg()
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.loadCloaksCmd(), m.loadSitesCmd())
}

func (m *Model) loadSitesCmd() tea.Cmd {
	return func() tea.Msg {
		wc := m.workingConfigFromInputs()
		targets := config.EffectiveTargets(wc)
		if len(targets) == 0 {
			return sitesMsg{
				nil,
				fmt.Errorf("SSH: укажите host + user в Settings, или список в bulk (b), или ssh_targets в YAML"),
				"",
			}
		}
		ctx := context.Background()
		var all []fastpanel.Site
		var notes []string
		for _, t := range targets {
			sub := *wc
			sub.SSHHost = t.Host
			sub.SSHUser = t.User
			if t.Port > 0 {
				sub.SSHPort = t.Port
			}
			dialCfg := sshDialCfg(&sub)
			if pw := strings.TrimSpace(t.Password); pw != "" {
				dialCfg.Password = pw
			}
			conn, err := sshutil.Dial(ctx, dialCfg)
			if err != nil {
				notes = append(notes, fmt.Sprintf("%s@%s: %v", t.User, t.Host, err))
				continue
			}
			sites, err := fastpanel.FetchSites(ctx, conn, sub.MogwaiCmd)
			conn.Close()
			if err != nil {
				notes = append(notes, fmt.Sprintf("%s@%s list: %v", t.User, t.Host, err))
				continue
			}
			label := strings.TrimSpace(t.Name)
			if label == "" {
				label = t.Host
			}
			for i := range sites {
				sites[i].SSHHost = t.Host
				sites[i].SSHUser = t.User
				if sub.SSHPort > 0 {
					sites[i].SSHPort = sub.SSHPort
				}
				sites[i].SSHLabel = label
				if pw := strings.TrimSpace(t.Password); pw != "" {
					sites[i].SSHPassword = pw
				}
			}
			all = append(all, sites...)
		}
		if len(all) == 0 {
			msg := "ни одного сайта"
			if len(notes) > 0 {
				msg = strings.Join(notes, "; ")
			}
			return sitesMsg{nil, fmt.Errorf("%s", msg), ""}
		}
		warn := ""
		if len(notes) > 0 {
			warn = strings.Join(notes, "\n")
		}
		return sitesMsg{sites: all, err: nil, warn: warn}
	}
}

func (m *Model) loadCloaksCmd() tea.Cmd {
	if m.vault == nil {
		return nil
	}
	return func() tea.Msg {
		list, err := m.vault.List()
		if err != nil {
			return cloaksListMsg{nil, err}
		}
		return cloaksListMsg{list, nil}
	}
}

type cloaksListMsg struct {
	list []cloaks.Entry
	err  error
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.sshBulkOpen && m.width > 0 {
			m.sshBulkTA.SetWidth(max(20, m.width-8))
		}
		if m.cloakBulkOpen && m.width > 0 {
			m.cloakBulkTA.SetWidth(max(20, m.width-8))
		}
		if m.addStep == 6 && m.width > 0 {
			m.cloakCodeTA.SetWidth(max(20, m.width-8))
			if m.height > 16 {
				m.cloakCodeTA.SetHeight(min(24, m.height-14))
			}
		}
		return m, nil

	case tea.KeyMsg:
		if m.browse.active {
			return m.updateBrowse(msg)
		}
		if m.sshBulkOpen {
			return m.updateSSHBulk(msg)
		}
		if m.cloakBulkOpen {
			return m.updateCloakBulk(msg)
		}
		if m.entryNameEdit {
			return m.updateEntryNameEdit(msg)
		}
		if m.tagEditSite != "" {
			return m.updateTagEdit(msg)
		}
		if m.addStep != 0 {
			return m.updateAddCloak(msg)
		}
		if m.scr == scSettings && m.settingsAnyFocused() {
			if msg.String() == "esc" {
				m.blurAllSettings()
				return m, nil
			}
		}

		inOverlay := m.entryNameEdit || m.tagEditSite != "" || m.addStep != 0
		settingsEditing := m.scr == scSettings && m.settingsAnyFocused()
		if !inOverlay {
			if settingsEditing && keyShiftTab(msg) {
				prev := (m.settingsCurrentField() + settingsFieldCount - 1) % settingsFieldCount
				return m, m.focusSettingsField(prev)
			}
			if !settingsEditing {
				if keyArrowLeft(msg) || keyShiftTab(msg) {
					m.setScreen((m.scr + 3) % 4)
					return m, nil
				}
				if keyArrowRight(msg) {
					m.setScreen((m.scr + 1) % 4)
					return m, nil
				}
			}
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			if m.scr == scRun && m.jobRunning {
				return m, nil
			}
			return m, tea.Quit
		case "tab":
			m.setScreen((m.scr + 1) % 4)
			return m, nil
		case "1", "2", "3", "4":
			m.setScreen(screen(int(msg.String()[0] - '1')))
			return m, nil
		}
		switch m.scr {
		case scDomains:
			return m.updateDomains(msg)
		case scCloaks:
			return m.updateCloaks(msg)
		case scSettings:
			return m.updateSettings(msg)
		case scRun:
			return m.updateRun(msg)
		}

	case sitesMsg:
		m.loading = false
		if msg.err != nil {
			m.sitesErr = msg.err.Error()
			m.sites = nil
			m.sitesLoadWarn = ""
		} else {
			m.sitesErr = ""
			m.sitesLoadWarn = msg.warn
			m.sites = msg.sites
			m.cursor = 0
			m.selected = map[int]struct{}{}
			m.clampDomainCursor(len(m.filteredSites()))
		}
		return m, nil

	case cloaksListMsg:
		if msg.err == nil {
			m.cloakList = msg.list
			sort.Slice(m.cloakList, func(i, j int) bool {
				return m.cloakList[i].Label < m.cloakList[j].Label
			})
			if m.cloakSelID == "" && len(m.cloakList) > 0 {
				m.cloakSelID = m.cloakList[0].ID
			}
		}
		return m, nil

	case progressMsg:
		m.jobIndex = msg.next
		m.jobTotal = msg.total
		m.jobLog = append(m.jobLog, msg.result)
		if !msg.result.OK {
			m.jobFailCount++
		}
		if msg.next >= msg.total {
			m.jobRunning = false
			return m, nil
		}
		return m, m.replaceStepCmd(msg.next)
	}
	return m, nil
}

func (m *Model) updateDomains(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.blurAllSettings()
	list := m.filteredSites()
	m.clampDomainCursor(len(list))
	switch msg.String() {
	case "r":
		m.loading = true
		m.sitesErr = ""
		return m, m.loadSitesCmd()
	case "/":
		// simple cycle: empty -> FR hint — real filter: use 't' + type
		return m, nil
	case "t":
		// tag filter dialog minimal: set tagFilter to next common or clear
		if m.tagFilter != "" {
			m.tagFilter = ""
		} else {
			m.tagFilter = "FR"
		}
		m.clampDomainCursor(len(m.filteredSites()))
		return m, nil
	}
	if keyIsUp(msg) {
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	}
	if keyIsDown(msg) {
		if m.cursor < len(list)-1 {
			m.cursor++
		}
		return m, nil
	}
	switch msg.String() {
	case " ":
		if len(list) == 0 {
			break
		}
		idx := m.realIndex(list, m.cursor)
		if _, ok := m.selected[idx]; ok {
			delete(m.selected, idx)
		} else {
			m.selected[idx] = struct{}{}
		}
	case "a":
		for i := range m.sites {
			m.selected[i] = struct{}{}
		}
	case "A":
		for i := range m.sites {
			if _, ok := m.selected[i]; ok {
				delete(m.selected, i)
			} else {
				m.selected[i] = struct{}{}
			}
		}
	case "n":
		m.scr = scRun
		return m, nil
	case "g":
		list := m.filteredSites()
		if len(list) > 0 && m.cursor < len(list) {
			idx := m.realIndex(list, m.cursor)
			site := m.sites[idx]
			m.tagEditSite = site.ServerName
			m.tagInp.SetValue(strings.Join(m.cfg.TagsFor(site.ServerName), ", "))
			m.tagInp.Focus()
			return m, textinput.Blink
		}
	case "i":
		m.entryNameEdit = true
		m.entryNameInp.SetValue(m.entryName)
		m.entryNameInp.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

func (m *Model) clampDomainCursor(n int) {
	if n == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) updateEntryNameEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.entryNameEdit = false
		m.entryNameInp.Blur()
		return m, nil
	case "enter":
		v := strings.TrimSpace(m.entryNameInp.Value())
		if v != "" {
			m.entryName = v
		}
		m.entryNameEdit = false
		m.entryNameInp.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.entryNameInp, cmd = m.entryNameInp.Update(msg)
	return m, cmd
}

func (m *Model) updateTagEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.tagEditSite = ""
		m.tagInp.Blur()
		return m, nil
	case "enter":
		raw := m.tagInp.Value()
		var tags []string
		for _, p := range strings.Split(raw, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				tags = append(tags, p)
			}
		}
		m.cfg.SetTags(m.tagEditSite, tags)
		_ = config.Save(m.cfgPath, m.cfg)
		m.tagEditSite = ""
		m.tagInp.Blur()
		m.tagInp.SetValue("")
		return m, nil
	}
	var cmd tea.Cmd
	m.tagInp, cmd = m.tagInp.Update(msg)
	return m, cmd
}

func sameSite(a, b fastpanel.Site) bool {
	return a.ID == b.ID && a.ServerName == b.ServerName &&
		a.SSHHost == b.SSHHost && a.SSHUser == b.SSHUser &&
		a.SSHPassword == b.SSHPassword
}

func (m *Model) realIndex(filtered []fastpanel.Site, cursor int) int {
	if cursor < 0 || cursor >= len(filtered) {
		return 0
	}
	target := filtered[cursor]
	for i, s := range m.sites {
		if sameSite(s, target) {
			return i
		}
	}
	return cursor
}

func (m *Model) filteredSites() []fastpanel.Site {
	if m.tagFilter == "" {
		return m.sites
	}
	var out []fastpanel.Site
	tag := strings.ToUpper(strings.TrimSpace(m.tagFilter))
	for _, s := range m.sites {
		for _, t := range m.cfg.TagsFor(s.ServerName) {
			if strings.EqualFold(t, tag) {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

func (m *Model) updateCloaks(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.blurAllSettings()
	if keyIsUp(msg) {
		if m.cloakCursor > 0 {
			m.cloakCursor--
		}
		return m, nil
	}
	if keyIsDown(msg) {
		if m.cloakCursor < len(m.cloakList)-1 {
			m.cloakCursor++
		}
		return m, nil
	}
	switch msg.String() {
	case "enter", " ":
		if m.cloakCursor < len(m.cloakList) {
			m.cloakSelID = m.cloakList[m.cloakCursor].ID
		}
	case "n":
		m.addStep = 5
		m.addLabelInp.SetValue("")
		m.addLabelInp.Focus()
		return m, textinput.Blink
	case "f":
		m.addStep = 1
		m.addPathInp.SetValue("")
		m.addPathInp.Focus()
		return m, textinput.Blink
	case "b":
		m.openCloakBulkEditor()
		return m, textarea.Blink
	case "t":
		m.addStep = 4
		m.addLabelInp.SetValue("")
		m.addLabelInp.Focus()
		return m, textinput.Blink
	case "d":
		if m.vault != nil && m.cloakCursor < len(m.cloakList) {
			id := m.cloakList[m.cloakCursor].ID
			_ = m.vault.Remove(id)
			return m, m.loadCloaksCmd()
		}
	}
	return m, nil
}

func (m *Model) updateAddCloak(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.addStep = 0
		m.addPathInp.Blur()
		m.addLabelInp.Blur()
		m.cloakCodeTA.Blur()
		m.addPathInp.SetValue("")
		m.addLabelInp.SetValue("")
		m.cloakCodeTA.SetValue("")
		return m, nil
	}
	switch m.addStep {
	case 1:
		if msg.String() == "f" {
			start := strings.TrimSpace(m.addPathInp.Value())
			m.openBrowse(browseKindPickFile, start, func(p string) {
				m.addPathInp.SetValue(p)
				m.addPathInp.Focus()
			}, nil)
			return m, nil
		}
		var cmd tea.Cmd
		m.addPathInp, cmd = m.addPathInp.Update(msg)
		if msg.String() == "enter" {
			m.addStep = 2
			m.addPathInp.Blur()
			m.addLabelInp.Focus()
			return m, textinput.Blink
		}
		return m, cmd
	case 2:
		var cmd tea.Cmd
		m.addLabelInp, cmd = m.addLabelInp.Update(msg)
		if msg.String() == "enter" {
			p := strings.TrimSpace(m.addPathInp.Value())
			l := strings.TrimSpace(m.addLabelInp.Value())
			m.addStep = 0
			m.addLabelInp.Blur()
			m.addPathInp.SetValue("")
			m.addLabelInp.SetValue("")
			if p != "" && m.vault != nil {
				if l == "" {
					l = filepath.Base(p)
				}
				_, _ = m.vault.Add(l, p)
			}
			return m, m.loadCloaksCmd()
		}
		return m, cmd
	case 4:
		var cmd tea.Cmd
		m.addLabelInp, cmd = m.addLabelInp.Update(msg)
		if msg.String() == "enter" {
			l := strings.TrimSpace(m.addLabelInp.Value())
			m.addStep = 0
			m.addLabelInp.Blur()
			m.addLabelInp.SetValue("")
			if m.vault != nil && l != "" {
				_, _ = m.vault.CreateEmptyPHP(l)
			}
			return m, m.loadCloaksCmd()
		}
		return m, cmd
	case 5:
		var cmd tea.Cmd
		m.addLabelInp, cmd = m.addLabelInp.Update(msg)
		if msg.String() == "enter" {
			v := strings.TrimSpace(m.addLabelInp.Value())
			if v == "" {
				return m, cmd
			}
			m.addStep = 6
			m.addLabelInp.Blur()
			m.cloakCodeTA.SetValue("<?php\n")
			if m.width > 0 {
				m.cloakCodeTA.SetWidth(max(20, m.width-8))
			}
			if m.height > 16 {
				m.cloakCodeTA.SetHeight(min(24, m.height-14))
			} else {
				m.cloakCodeTA.SetHeight(14)
			}
			m.cloakCodeTA.Focus()
			return m, textarea.Blink
		}
		return m, cmd
	case 6:
		if msg.String() == "ctrl+s" {
			label := strings.TrimSpace(m.addLabelInp.Value())
			body := m.cloakCodeTA.Value()
			m.addStep = 0
			m.cloakCodeTA.Blur()
			m.addLabelInp.SetValue("")
			m.cloakCodeTA.SetValue("")
			if m.vault != nil && label != "" {
				_, _ = m.vault.AddBytes(label, []byte(body), ".php")
			}
			return m, m.loadCloaksCmd()
		}
		var cmd tea.Cmd
		m.cloakCodeTA, cmd = m.cloakCodeTA.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "ctrl+s":
		m.cfg.SSHHost = strings.TrimSpace(m.hostInp.Value())
		m.cfg.SSHUser = strings.TrimSpace(m.userInp.Value())
		m.cfg.SSHPassword = m.passInp.Value()
		fmt.Sscanf(m.portInp.Value(), "%d", &m.cfg.SSHPort)
		if m.cfg.SSHPort == 0 {
			m.cfg.SSHPort = 22
		}
		m.cfg.DefaultEntry = strings.TrimSpace(m.entryDef.Value())
		if m.cfg.DefaultEntry == "" {
			m.cfg.DefaultEntry = "index.php"
		}
		m.entryName = m.cfg.DefaultEntry
		m.cfg.MogwaiCmd = strings.TrimSpace(m.mogwaiIn.Value())
		if m.cfg.MogwaiCmd == "" {
			m.cfg.MogwaiCmd = "mogwai sites list"
		}
		m.cfg.SSHIdentityFile = strings.TrimSpace(m.identityInp.Value())
		m.cfg.SSHKnownHosts = strings.TrimSpace(m.knownHostsInp.Value())
		_ = config.Save(m.cfgPath, m.cfg)
		return m, nil
	case "b":
		m.openSSHBulkEditor()
		return m, textarea.Blink
	}
	if keyIsUp(msg) {
		prev := (m.settingsCurrentField() + settingsFieldCount - 1) % settingsFieldCount
		return m, m.focusSettingsField(prev)
	}
	if keyIsDown(msg) {
		next := (m.settingsCurrentField() + 1) % settingsFieldCount
		return m, m.focusSettingsField(next)
	}
	switch {
	case m.hostInp.Focused():
		m.hostInp, cmd = m.hostInp.Update(msg)
	case m.userInp.Focused():
		m.userInp, cmd = m.userInp.Update(msg)
	case m.passInp.Focused():
		m.passInp, cmd = m.passInp.Update(msg)
	case m.portInp.Focused():
		m.portInp, cmd = m.portInp.Update(msg)
	case m.identityInp.Focused():
		m.identityInp, cmd = m.identityInp.Update(msg)
	case m.knownHostsInp.Focused():
		m.knownHostsInp, cmd = m.knownHostsInp.Update(msg)
	case m.entryDef.Focused():
		m.entryDef, cmd = m.entryDef.Update(msg)
	case m.mogwaiIn.Focused():
		m.mogwaiIn, cmd = m.mogwaiIn.Update(msg)
	default:
		switch msg.String() {
		case "h":
			return m, m.focusSettingsField(0)
		case "u":
			return m, m.focusSettingsField(1)
		case "w":
			return m, m.focusSettingsField(2)
		case "p":
			return m, m.focusSettingsField(3)
		case "i":
			return m, m.focusSettingsField(4)
		case "k":
			return m, m.focusSettingsField(5)
		case "e":
			return m, m.focusSettingsField(6)
		case "m":
			return m, m.focusSettingsField(7)
		case "y":
			start := strings.TrimSpace(m.identityInp.Value())
			m.openBrowse(browseKindPickFile, start, func(p string) {
				m.identityInp.SetValue(p)
				m.identityInp.Focus()
			}, nil)
			return m, nil
		case "o":
			start := strings.TrimSpace(m.knownHostsInp.Value())
			m.openBrowse(browseKindPickFile, start, func(p string) {
				m.knownHostsInp.SetValue(p)
				m.knownHostsInp.Focus()
			}, nil)
			return m, nil
		}
	}
	return m, cmd
}

func (m *Model) updateRun(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.blurAllSettings()
	if m.jobRunning {
		return m, nil
	}
	if msg.String() == "enter" {
		cmd := m.startJobCmd()
		if cmd == nil {
			return m, nil
		}
		return m, cmd
	}
	return m, nil
}

func (m *Model) startJobCmd() tea.Cmd {
	resolved := m.selectedSites()
	if len(resolved) == 0 {
		return nil
	}
	if m.vault == nil {
		return func() tea.Msg {
			return progressMsg{1, 1, replace.SiteResult{Message: "vault_unavailable"}}
		}
	}
	var ent cloaks.Entry
	var ok bool
	man, _ := m.vault.LoadManifest()
	if man != nil {
		ent, ok = man.EntryByID(m.cloakSelID)
	}
	if !ok {
		return func() tea.Msg {
			return progressMsg{1, 1, replace.SiteResult{Message: "no_cloak_selected"}}
		}
	}
	b, err := m.vault.ReadBytes(ent)
	if err != nil {
		return func() tea.Msg {
			return progressMsg{1, 1, replace.SiteResult{Message: err.Error()}}
		}
	}
	entry := strings.TrimSpace(m.entryName)
	if entry == "" {
		entry = m.cfg.DefaultEntry
	}
	wc := m.workingConfigFromInputs()
	cfgCopy := *wc
	m.jobSites = resolved
	m.jobContent = b
	m.jobEntry = entry
	m.jobCfg = &cfgCopy
	m.jobRunning = true
	m.jobIndex = 0
	m.jobTotal = len(resolved)
	m.jobLog = nil
	m.jobFailCount = 0
	return m.replaceStepCmd(0)
}

// replaceStepCmd replaces site at jobSites[idx] and returns progressMsg with next == idx+1.
func (m *Model) replaceStepCmd(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.jobSites) {
		return nil
	}
	site := m.jobSites[idx]
	content := m.jobContent
	entry := m.jobEntry
	cfg := m.jobCfg
	total := len(m.jobSites)
	return func() tea.Msg {
		ctx := context.Background()
		dial := sshDialCfg(cfg)
		if site.SSHHost != "" {
			dial.Host = site.SSHHost
		}
		if site.SSHUser != "" {
			dial.User = site.SSHUser
		}
		if site.SSHPort > 0 {
			dial.Port = site.SSHPort
		}
		if pw := strings.TrimSpace(site.SSHPassword); pw != "" {
			dial.Password = pw
		}
		cli, err := sshutil.Dial(ctx, dial)
		if err != nil {
			return progressMsg{idx + 1, total, replace.SiteResult{Site: site, Message: "ssh:" + err.Error()}}
		}
		defer cli.Close()
		sftpR, err := sshutil.NewSFTP(cli.Raw())
		if err != nil {
			return progressMsg{idx + 1, total, replace.SiteResult{Site: site, Message: "sftp:" + err.Error()}}
		}
		defer sftpR.Close()
		runner := replace.Runner{Config: cfg, Remote: sftpR}
		sum := runner.ReplaceSite(ctx, site, entry, content)
		return progressMsg{idx + 1, total, sum}
	}
}
