package main

import (
	"strings"
	"testing"
)

func TestRewriteImports(t *testing.T) {
	src := `package cmd

import (
	"fmt"

	"github.com/stn81/kate/app"
	"github.com/stn81/kate/cmd/kate/template/service/config"
	"github.com/stn81/kate/cmd/kate/template/service/app/kateapp/cmd"
)

var _ = fmt.Sprint(app.GetName(), config.Main, cmd.GlobalFlags)
`
	got, err := rewriteImports([]byte(src), "x.go", "dw-sync2", "myapp")
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, `"dw-sync2/config"`) {
		t.Errorf("module prefix not rewritten: %s", s)
	}
	if !strings.Contains(s, `"dw-sync2/app/myapp/cmd"`) {
		t.Errorf("app dir not renamed in import: %s", s)
	}
	// 框架自身 import 绝不能被误伤（路径段边界）。
	if !strings.Contains(s, `"github.com/stn81/kate/app"`) {
		t.Errorf("framework import damaged: %s", s)
	}
	if strings.Contains(s, templateModule) {
		t.Errorf("template module path残留: %s", s)
	}
}

func TestRewriteAnchorLine(t *testing.T) {
	mk := "X\nAPP  = kateapp\nY"
	got, err := rewriteAnchorLine([]byte(mk), "Makefile", "APP  = kateapp", "APP  = svc1")
	if err != nil || !strings.Contains(string(got), "APP  = svc1") {
		t.Fatalf("got %q err %v", got, err)
	}
	if _, err := rewriteAnchorLine([]byte("nothing"), "Makefile", "APP  = kateapp", "x"); err == nil {
		t.Error("missing anchor line must error")
	}
}

func TestCheckModulePath_LaxAllowsBareNames(t *testing.T) {
	for _, ok := range []string{"dw-sync2", "logcollect", "github.com/acme/x", "x.example.com/a/b"} {
		if err := checkModulePath(ok); err != nil {
			t.Errorf("checkModulePath(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "a b", "/x", "x/", "a//b", "-x"} {
		if err := checkModulePath(bad); err == nil {
			t.Errorf("checkModulePath(%q) = nil, want error", bad)
		}
	}
}
