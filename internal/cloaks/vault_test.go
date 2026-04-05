package cloaks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddBytesAndKeepOnly(t *testing.T) {
	root := t.TempDir()
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	e1, err := v.AddBytes("a", []byte("<?php\n"), ".php")
	if err != nil {
		t.Fatal(err)
	}
	e2, err := v.AddBytes("b", []byte("x"), ".php")
	if err != nil {
		t.Fatal(err)
	}
	list, _ := v.List()
	if len(list) != 2 {
		t.Fatalf("list len=%d", len(list))
	}
	if err := v.KeepOnlyIDs([]string{e1.ID}); err != nil {
		t.Fatal(err)
	}
	list, _ = v.List()
	if len(list) != 1 || list[0].ID != e1.ID {
		t.Fatalf("%+v", list)
	}
	p2 := filepath.Join(root, filepath.FromSlash(e2.RelPath))
	if _, err := os.Stat(p2); !os.IsNotExist(err) {
		t.Fatal("removed file should be gone")
	}
}

func TestParseBulkIDs(t *testing.T) {
	s := "# c\n\n" + "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee\tlabel\n"
	ids := ParseBulkIDs(s)
	if len(ids) != 1 || ids[0] != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Fatalf("%q", ids)
	}
}
