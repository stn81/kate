package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type newOptions struct {
	module  string
	appName string
	dir     string
	git     bool
	// components: comp → enabled
	components map[string]bool
	// kateReplace 非空时在生成项目里 replace kate 到本地路径（CI / 本地开发用）。
	kateReplace string
}

func runNew(args []string) error {
	opts, err := parseNewArgs(args)
	if err != nil {
		return err
	}

	target, err := filepath.Abs(opts.dir)
	if err != nil {
		return err
	}
	if err := ensureTargetUsable(target); err != nil {
		return err
	}

	// staging 与目标同父目录（保证同文件系统可原子 rename）；失败自动清理，无半成品。
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(target), ".kate-new-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging) // 成功 rename 后该目录已不存在，RemoveAll no-op

	disabled := map[string]bool{}
	for comp, enabled := range opts.components {
		if !enabled {
			disabled[comp] = true
		}
	}

	if err := materialize(staging, opts, disabled); err != nil {
		return err
	}

	degraded, err := setupGoModule(staging, opts)
	if err != nil {
		return err
	}

	if opts.git {
		if err := gitInit(staging); err != nil {
			fmt.Fprintf(os.Stderr, "warning: git init failed (project kept): %v\n", err)
		}
	}

	// 原子落盘：空目录先移除再 rename。
	_ = os.Remove(target) // 仅当 target 是已存在的空目录时成功
	if err := os.Rename(staging, target); err != nil {
		return fmt.Errorf("finalize %s: %w", target, err)
	}

	printNextSteps(target, opts, degraded)
	return nil
}

func parseNewArgs(args []string) (*newOptions, error) {
	opts := &newOptions{components: map[string]bool{}}
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	name := fs.String("name", "", "app name (default: basename of module)")
	dir := fs.String("dir", "", "target directory (default: ./<app name>)")
	grpc := fs.Bool("grpc", false, "include grpc server component")
	mysql := fs.Bool("mysql", true, "include mysql/model component")
	redis := fs.Bool("redis", true, "include redis component")
	git := fs.Bool("git", true, "git init + initial commit")

	// 支持位置参数与 flag 任意顺序混排（stdlib flag 遇首个非 flag 即停，循环续 parse）。
	var positional []string
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	for fs.NArg() > 0 {
		positional = append(positional, fs.Arg(0))
		if err := fs.Parse(fs.Args()[1:]); err != nil {
			return nil, err
		}
	}
	if len(positional) != 1 {
		return nil, fmt.Errorf("want exactly one <module> argument, got %v", positional)
	}

	opts.module = positional[0]
	if err := checkModulePath(opts.module); err != nil {
		return nil, err
	}
	opts.appName = *name
	if opts.appName == "" {
		opts.appName = path.Base(opts.module)
	}
	if !appNameRe.MatchString(opts.appName) {
		return nil, fmt.Errorf("invalid app name %q (want [a-zA-Z0-9._-]+)", opts.appName)
	}
	opts.dir = *dir
	if opts.dir == "" {
		opts.dir = "./" + opts.appName
	}
	opts.components["grpc"] = *grpc
	opts.components["mysql"] = *mysql
	opts.components["redis"] = *redis
	opts.git = *git
	opts.kateReplace = os.Getenv("KATE_NEW_REPLACE")
	return opts, nil
}

var appNameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// checkModulePath 是 go mod init 级别的宽松校验：放行裸名（如 dw-sync2），
// 仅拦明显非法（空、空白、反斜杠、空路径段、首尾斜杠）。
func checkModulePath(p string) error {
	switch {
	case p == "":
		return fmt.Errorf("module path is empty")
	case strings.ContainsAny(p, " \t\\"):
		return fmt.Errorf("module path %q contains whitespace or backslash", p)
	case strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") || strings.Contains(p, "//"):
		return fmt.Errorf("module path %q has empty path elements", p)
	case strings.HasPrefix(p, "-"):
		return fmt.Errorf("module path %q starts with dash", p)
	}
	return nil
}

func ensureTargetUsable(target string) error {
	entries, err := os.ReadDir(target)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return err
	case len(entries) > 0:
		return fmt.Errorf("target directory %s is not empty", target)
	}
	return nil
}

// materialize 把 embed 模板实例化到 staging：裁剪 → 锚剥除 → import 改写 → 改名落盘。
func materialize(staging string, opts *newOptions, disabled map[string]bool) error {
	makefileAnchor := "APP  = " + templateAppName
	buildshAnchor := "APP=" + templateAppName

	return fs.WalkDir(templateFS, templateRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(p, templateRoot+"/")
		if fileOwnedByDisabled(rel, disabled) {
			return nil
		}

		content, err := fs.ReadFile(templateFS, p)
		if err != nil {
			return err
		}

		if prunableFile(rel) {
			if content, err = pruneAnchors(content, rel, disabled); err != nil {
				return err
			}
		}
		switch {
		case strings.HasSuffix(rel, ".go"):
			if content, err = rewriteImports(content, rel, opts.module, opts.appName); err != nil {
				return err
			}
		case rel == "Makefile":
			if content, err = rewriteAnchorLine(content, rel, makefileAnchor, "APP  = "+opts.appName); err != nil {
				return err
			}
		case rel == "scripts/build.sh":
			if content, err = rewriteAnchorLine(content, rel, buildshAnchor, "APP="+opts.appName); err != nil {
				return err
			}
		}

		// 路径变换：app 目录改名 + 文件名还原表。
		outRel := rel
		if appDir := "app/" + templateAppName + "/"; strings.HasPrefix(outRel, appDir) {
			outRel = "app/" + opts.appName + "/" + strings.TrimPrefix(outRel, appDir)
		}
		base := path.Base(outRel)
		if renamed, ok := renameOnMaterialize[base]; ok {
			outRel = path.Join(path.Dir(outRel), renamed)
		}

		outPath := filepath.Join(staging, filepath.FromSlash(outRel))
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		perm := os.FileMode(0o644)
		if strings.HasSuffix(outRel, ".sh") { // embed 不保留可执行位
			perm = 0o755
		}
		return os.WriteFile(outPath, content, perm)
	})
}

// setupGoModule 在 staging 内完成 go.mod 合成与依赖收口。
// tidy/vendor 网络失败不毁项目：降级为 warning + 补救命令（返回 degraded=true）。
func setupGoModule(staging string, opts *newOptions) (degraded bool, err error) {
	run := func(name string, args ...string) error {
		cmd := exec.Command(name, args...)
		cmd.Dir = staging
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if err := run("go", "mod", "init", opts.module); err != nil {
		return false, fmt.Errorf("go mod init: %w", err)
	}
	if opts.kateReplace != "" {
		abs, err := filepath.Abs(opts.kateReplace)
		if err != nil {
			return false, err
		}
		if err := run("go", "mod", "edit", "-replace=github.com/stn81/kate="+abs); err != nil {
			return false, fmt.Errorf("go mod edit -replace kate: %w", err)
		}
	}
	// 与 kate 自身一致的 grpc 源 replace。注意：写入字面 "latest" 后任何 go mod edit
	// 都无法再解析该文件、只有 tidy/get 能把它落成具体版本 —— 这必须是最后一个 edit。
	if err := run("go", "mod", "edit", "-replace=google.golang.org/grpc=github.com/grpc/grpc-go@latest"); err != nil {
		return false, fmt.Errorf("go mod edit -replace grpc: %w", err)
	}

	steps := [][]string{{"go", "mod", "tidy"}}
	if opts.kateReplace == "" {
		if v := kateVersion(); v != "(devel)" {
			// 生成项目锁定与本 CLI 同版本的 kate：模板形态与框架依赖严格同源。
			// 必须在 tidy 之后执行（那时 go.mod 已是规范形式）。
			steps = append(steps, []string{"go", "get", "github.com/stn81/kate@" + v})
		}
	}
	steps = append(steps, []string{"go", "mod", "vendor"})

	for _, step := range steps {
		if err := run(step[0], step[1:]...); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s failed (network?); project kept, finish manually:\n"+
				"  cd <project> && go mod tidy && go mod vendor && go build ./...\n"+
				"hint: freshly published tags may lag on proxy.golang.org — retry with GOPROXY=direct\n",
				strings.Join(step, " "))
			return true, nil
		}
	}

	// 硬合同：生成即编译通过。
	if err := run("go", "build", "./..."); err != nil {
		return false, fmt.Errorf("generated project does not build: %w", err)
	}
	return false, nil
}

func gitInit(staging string) error {
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"commit", "-q", "-m", "initial scaffold by kate " + kateVersion()},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = staging
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s: %v: %s", args[0], err, out)
		}
	}
	return nil
}

func printNextSteps(target string, opts *newOptions, degraded bool) {
	fmt.Printf("\nproject created: %s (module %s, app %s)\n", target, opts.module, opts.appName)
	if degraded {
		fmt.Printf("\n  NOTE: dependency download was skipped, run first:\n    cd %s && go mod tidy && go mod vendor\n", target)
	}
	fmt.Printf(`
next steps:
  cd %s
  ./scripts/build.sh dev          # 构建到 outputs/（bin/conf/log/run）
  ./outputs/bin/%s start          # 默认读 outputs/conf/%s.ini
  curl http://127.0.0.1:8080/hello
`, target, opts.appName, opts.appName)
}
