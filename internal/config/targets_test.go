package config

import (
	"testing"
)

func TestEffectiveTargets_list(t *testing.T) {
	c := &Config{
		SSHUser: "fallback",
		SSHPort: 22,
		SSHTargets: []SSHTarget{
			{Host: "a.example.com", User: "root"},
			{Host: "b.example.com", User: "", Port: 2222},
		},
	}
	got := EffectiveTargets(c)
	if len(got) != 2 {
		t.Fatalf("len=%d %#v", len(got), got)
	}
	if got[0].Host != "a.example.com" || got[0].User != "root" || got[0].Port != 22 {
		t.Fatalf("first: %#v", got[0])
	}
	if got[1].User != "fallback" || got[1].Port != 2222 {
		t.Fatalf("second: %#v", got[1])
	}
}

func TestEffectiveTargets_legacy(t *testing.T) {
	c := &Config{SSHHost: "h", SSHUser: "u", SSHPort: 22}
	got := EffectiveTargets(c)
	if len(got) != 1 || got[0].Host != "h" || got[0].User != "u" {
		t.Fatalf("%#v", got)
	}
}

func TestParseSSHLine(t *testing.T) {
	s, err := ParseSSHLine("root@10.0.0.1:2222")
	if err != nil || s.User != "root" || s.Host != "10.0.0.1" || s.Port != 2222 || s.Password != "" {
		t.Fatalf("%#v %v", s, err)
	}
	s2, err := ParseSSHLine("deploy@panel.local")
	if err != nil || s2.User != "deploy" || s2.Host != "panel.local" || s2.Port != 0 {
		t.Fatalf("%#v %v", s2, err)
	}
	s3, err := ParseSSHLine("u@h:22\tsec ret")
	if err != nil || s3.Password != "sec ret" || s3.Host != "h" {
		t.Fatalf("%#v %v", s3, err)
	}
	_, err = ParseSSHLine("bad")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFormatSSHBulkPasswordRoundTrip(t *testing.T) {
	in := []SSHTarget{{User: "a", Host: "b", Port: 2, Password: "p1"}, {User: "c", Host: "d"}}
	text := FormatSSHBulk(in)
	out, errs := ParseSSHBulk(text)
	if len(errs) != 0 || len(out) != 2 {
		t.Fatalf("%v %#v", errs, out)
	}
	if out[0].Password != "p1" || out[1].Password != "" {
		t.Fatalf("%#v", out)
	}
}
