package config

import (
	"testing"
	"time"
)

// 只断言 main/http/profiling 这些与可选组件无关的段，保证任意组件裁剪后测试仍成立。
func TestLoadDevIni(t *testing.T) {
	if err := Load("../scripts/conf/dev.ini"); err != nil {
		t.Fatalf("load dev.ini: %v", err)
	}
	if HTTP.Addr != ":8080" {
		t.Errorf("http addr = %q, want :8080", HTTP.Addr)
	}
	if HTTP.HandleTimeout != 30*time.Second {
		t.Errorf("handle_timeout = %v, want 30s", HTTP.HandleTimeout)
	}
	if HTTP.MaxBodyBytes != 16777216 {
		t.Errorf("max_body_bytes = %d, want 16M", HTTP.MaxBodyBytes)
	}
	if !Profiling.Enabled || Profiling.Port != 18000 {
		t.Errorf("profiling = %+v, want enabled/18000", Profiling)
	}
}
