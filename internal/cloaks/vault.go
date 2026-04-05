package cloaks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Manifest is stored in vault as manifest.json
type Manifest struct {
	Entries []Entry   `json:"entries"`
	Updated time.Time `json:"updated"`
}

// Entry is one saved cloak template on disk.
type Entry struct {
	ID      string    `json:"id"`
	Label   string    `json:"label"`
	RelPath string    `json:"rel_path"`
	SHA256  string    `json:"sha256"`
	AddedAt time.Time `json:"added_at"`
}

// Vault manages the filesystem cloak library.
type Vault struct {
	Root string
}

// Open ensures root exists.
func Open(root string) (*Vault, error) {
	if root == "" {
		return nil, errors.New("empty vault root")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, "files"), 0o755); err != nil {
		return nil, err
	}
	return &Vault{Root: root}, nil
}

func (v *Vault) manifestPath() string {
	return filepath.Join(v.Root, "manifest.json")
}

// LoadManifest reads or creates empty manifest.
func (v *Vault) LoadManifest() (*Manifest, error) {
	p := v.manifestPath()
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Entries: []Entry{}}, nil
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (v *Vault) saveManifest(m *Manifest) error {
	m.Updated = time.Now().UTC()
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := v.manifestPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, v.manifestPath())
}

// List returns all entries.
func (v *Vault) List() ([]Entry, error) {
	m, err := v.LoadManifest()
	if err != nil {
		return nil, err
	}
	return m.Entries, nil
}

// Add copies file from srcPath into vault and registers entry with label.
func (v *Vault) Add(label string, srcPath string) (*Entry, error) {
	b, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, err
	}
	ext := filepath.Ext(srcPath)
	return v.AddBytes(label, b, ext)
}

// AddBytes writes raw content into the vault (default extension .php).
func (v *Vault) AddBytes(label string, content []byte, ext string) (*Entry, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "untitled"
	}
	ext = strings.TrimSpace(ext)
	if ext == "" {
		ext = ".php"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	sum := sha256.Sum256(content)
	sumHex := hex.EncodeToString(sum[:])

	id := uuid.NewString()
	base := id + ext
	dstRel := filepath.Join("files", base)
	dstAbs := filepath.Join(v.Root, filepath.FromSlash(dstRel))
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(dstAbs, content, 0o644); err != nil {
		return nil, err
	}

	m, err := v.LoadManifest()
	if err != nil {
		return nil, err
	}
	ent := Entry{
		ID:      id,
		Label:   label,
		RelPath: filepath.ToSlash(dstRel),
		SHA256:  sumHex,
		AddedAt: time.Now().UTC(),
	}
	m.Entries = append(m.Entries, ent)
	if err := v.saveManifest(m); err != nil {
		return nil, err
	}
	return &ent, nil
}

// CreateEmptyPHP adds a new minimal PHP file in the vault (like touch + <?php).
func (v *Vault) CreateEmptyPHP(label string) (*Entry, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "untitled"
	}
	id := uuid.NewString()
	base := id + ".php"
	dstRel := filepath.Join("files", base)
	dstAbs := filepath.Join(v.Root, filepath.FromSlash(dstRel))
	content := []byte("<?php\n")
	sum := sha256.Sum256(content)
	sumHex := hex.EncodeToString(sum[:])
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(dstAbs, content, 0o644); err != nil {
		return nil, err
	}
	m, err := v.LoadManifest()
	if err != nil {
		return nil, err
	}
	ent := Entry{
		ID:      id,
		Label:   label,
		RelPath: filepath.ToSlash(dstRel),
		SHA256:  sumHex,
		AddedAt: time.Now().UTC(),
	}
	m.Entries = append(m.Entries, ent)
	if err := v.saveManifest(m); err != nil {
		return nil, err
	}
	return &ent, nil
}

// KeepOnlyIDs drops entries (and files) whose ID is not in idsFromEditor (matched case-insensitively).
// Refuses to wipe all cloaks when the vault is non-empty and the parsed list is empty.
func (v *Vault) KeepOnlyIDs(idsFromEditor []string) error {
	m, err := v.LoadManifest()
	if err != nil {
		return err
	}
	keep := make(map[string]struct{})
	for _, id := range idsFromEditor {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		keep[strings.ToLower(id)] = struct{}{}
	}
	if len(m.Entries) > 0 && len(keep) == 0 {
		return errors.New("refusing to remove all cloaks (no ids left in editor)")
	}
	var next []Entry
	for _, e := range m.Entries {
		if _, ok := keep[strings.ToLower(e.ID)]; ok {
			next = append(next, e)
			continue
		}
		_ = os.Remove(filepath.Join(v.Root, filepath.FromSlash(e.RelPath)))
	}
	m.Entries = next
	return v.saveManifest(m)
}

// Remove deletes entry and file.
func (v *Vault) Remove(id string) error {
	m, err := v.LoadManifest()
	if err != nil {
		return err
	}
	var keep []Entry
	var removed *Entry
	for i := range m.Entries {
		if m.Entries[i].ID == id {
			removed = &m.Entries[i]
			continue
		}
		keep = append(keep, m.Entries[i])
	}
	if removed == nil {
		return fmt.Errorf("entry not found: %s", id)
	}
	m.Entries = keep
	_ = os.Remove(filepath.Join(v.Root, filepath.FromSlash(removed.RelPath)))
	if err := v.saveManifest(m); err != nil {
		return err
	}
	return nil
}

// ReadBytes returns file content for an entry (verifies hash).
func (v *Vault) ReadBytes(e Entry) ([]byte, error) {
	p := filepath.Join(v.Root, filepath.FromSlash(e.RelPath))
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != e.SHA256 {
		return nil, errors.New("hash mismatch for cloak file")
	}
	return b, nil
}

// EntryByID finds entry.
func (m *Manifest) EntryByID(id string) (Entry, bool) {
	for _, e := range m.Entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}
