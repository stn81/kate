package main

import (
	"strings"
	"testing"
)

func TestPruneAnchors_DisabledBlockDropped(t *testing.T) {
	src := `keep1
//kate:begin redis
redis line
//kate:end redis
keep2
;kate:begin mysql
[mysql]
;kate:end mysql
keep3
`
	got, err := pruneAnchors([]byte(src), "f", map[string]bool{"redis": true})
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if strings.Contains(s, "redis line") {
		t.Error("disabled block content must be dropped")
	}
	if !strings.Contains(s, "[mysql]") {
		t.Error("enabled block content must be kept")
	}
	if strings.Contains(s, "kate:") {
		t.Error("anchor marker lines must always be stripped")
	}
	for _, keep := range []string{"keep1", "keep2", "keep3"} {
		if !strings.Contains(s, keep) {
			t.Errorf("plain line %q lost", keep)
		}
	}
}

func TestPruneAnchors_UnbalancedErrors(t *testing.T) {
	cases := []string{
		"//kate:begin redis\nx\n",                          // 未闭合
		"//kate:end redis\n",                               // 无开
		"//kate:begin a\n//kate:begin b\n//kate:end a\n//kate:end b\n", // 交叉
	}
	for _, src := range cases {
		if _, err := pruneAnchors([]byte(src), "f", nil); err == nil {
			t.Errorf("expected error for %q", src)
		}
	}
}

func TestFileOwnedByDisabled(t *testing.T) {
	disabled := map[string]bool{"grpc": true, "mysql": true}
	cases := map[string]bool{
		"grpcsrv/grpcsrv.go":      true,
		"config/grpc.go":          true,
		"model/init.go":           true,
		"config/db.go":            true,
		"config/redis.go":         false, // redis 未关
		"httpsrv/httpsrv.go":      false,
		"app/kateapp/main.go":     false,
	}
	for rel, want := range cases {
		if got := fileOwnedByDisabled(rel, disabled); got != want {
			t.Errorf("fileOwnedByDisabled(%q) = %v, want %v", rel, got, want)
		}
	}
}
