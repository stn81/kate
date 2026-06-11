package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// 生成类测试较重（每组合一次 tidy+vendor+build），本地快循环用 go test -short 跳过，CI 全跑。

func repoRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func runIn(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

// TestNew_AllCombos 对组件 2^3 全组合真实生成 + go build/vet：
// 锚注释裁剪的"删除后必可编译"承诺由本矩阵兜底（中间组合的悬挂引用在这里红灯）。
func TestNew_AllCombos(t *testing.T) {
	if testing.Short() {
		t.Skip("heavy generation matrix; skipped with -short")
	}
	t.Setenv("KATE_NEW_REPLACE", repoRoot(t))

	bools := []bool{false, true}
	for _, grpc := range bools {
		for _, mysql := range bools {
			for _, redis := range bools {
				grpc, mysql, redis := grpc, mysql, redis
				name := fmt.Sprintf("grpc=%v,mysql=%v,redis=%v", grpc, mysql, redis)
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					dir := filepath.Join(t.TempDir(), "svc")
					// 裸 module 名与域名形态轮换覆盖（lax 校验的两种真实用法）。
					module := "gensvc"
					if grpc {
						module = "example.com/gen/svc"
					}
					err := runNew([]string{
						module, "-dir", dir, "-git=false",
						fmt.Sprintf("-grpc=%v", grpc),
						fmt.Sprintf("-mysql=%v", mysql),
						fmt.Sprintf("-redis=%v", redis),
					})
					if err != nil {
						t.Fatalf("runNew: %v", err)
					}
					// runNew 自身已跑 go build（硬合同）；再补 vet。
					runIn(t, dir, "go", "vet", "./...")
					assertCleanScaffold(t, dir)
				})
			}
		}
	}
}

// assertCleanScaffold 断言生成产物没有任何模板痕迹残留。
func assertCleanScaffold(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); err != nil {
		t.Errorf(".gitignore missing (embed dotfile rename broken): %v", err)
	}
	var leftovers []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && (d.Name() == "vendor" || d.Name() == ".git") {
				return filepath.SkipDir
			}
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		s := string(b)
		if strings.Contains(s, "kate:begin") || strings.Contains(s, "kate:end") {
			leftovers = append(leftovers, p+": anchor")
		}
		if strings.Contains(s, templateModule) {
			leftovers = append(leftovers, p+": template module path")
		}
		return nil
	})
	if len(leftovers) > 0 {
		t.Errorf("template traces left in scaffold:\n%s", strings.Join(leftovers, "\n"))
	}
}

// TestNew_HTTPOnlySmoke 最小 http-only 变体的运行级烟测：
// 真实走 scripts/build.sh dev → outputs/bin/<app> start → GET /hello，防"能编译但跑不起来"。
func TestNew_HTTPOnlySmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("runtime smoke; skipped with -short")
	}
	t.Setenv("KATE_NEW_REPLACE", repoRoot(t))

	dir := filepath.Join(t.TempDir(), "smokesvc")
	if err := runNew([]string{
		"smokesvc", "-dir", dir, "-git=false",
		"-grpc=false", "-mysql=false", "-redis=false",
	}); err != nil {
		t.Fatalf("runNew: %v", err)
	}

	// 避开本机常用端口。
	conf := filepath.Join(dir, "scripts/conf/dev.ini")
	b, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.Replace(string(b), "addr = :8080", "addr = :18098", 1)
	s = strings.Replace(s, "port = 18000", "port = 18019", 1)
	if err := os.WriteFile(conf, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}

	build := exec.Command("./scripts/build.sh", "dev")
	build.Dir = dir
	build.Env = append(os.Environ(), "GOROOT="+runtime.GOROOT())
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build.sh dev: %v\n%s", err, out)
	}

	srv := exec.Command(filepath.Join(dir, "outputs/bin/smokesvc"), "start")
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = srv.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() { _, _ = srv.Process.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = srv.Process.Kill()
		}
	}()

	deadline := time.Now().Add(15 * time.Second)
	for {
		resp, err := http.Get("http://127.0.0.1:18098/hello")
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET /hello status = %d", resp.StatusCode)
			}
			return // 200 即烟测通过（envelope 断言由模板自带的 httpsrv 单测覆盖）
		}
		if time.Now().After(deadline) {
			t.Fatalf("service did not become ready: %v", err)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func TestParseNewArgs_FlagPositionMix(t *testing.T) {
	opts, err := parseNewArgs([]string{"dw-sync2", "-grpc", "-redis=false"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.module != "dw-sync2" || opts.appName != "dw-sync2" || opts.dir != "./dw-sync2" {
		t.Errorf("defaults wrong: %+v", opts)
	}
	if !opts.components["grpc"] || opts.components["redis"] || !opts.components["mysql"] {
		t.Errorf("components wrong: %+v", opts.components)
	}

	opts2, err := parseNewArgs([]string{"-name", "api", "github.com/acme/payment-api"})
	if err != nil {
		t.Fatal(err)
	}
	if opts2.appName != "api" || opts2.module != "github.com/acme/payment-api" {
		t.Errorf("flag-first parse wrong: %+v", opts2)
	}
}
