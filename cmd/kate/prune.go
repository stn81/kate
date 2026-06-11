package main

import (
	"fmt"
	"regexp"
	"strings"
)

// 可选组件 → 整文件/整目录归属（路径相对模板根，目录以 / 结尾）。
// 行级归属用锚注释标记：Go 文件 //kate:begin <comp> ... //kate:end <comp>，
// ini 文件 ;kate:begin <comp> ... ;kate:end <comp>。
// 裁剪是纯减法：关闭组件 = 删文件 + 删锚块；锚注释行本身无论开关一律剥除。
// 约定的兜底：生成测试对组件全组合做 go build/vet，悬挂引用在 CI 红灯。
var componentFiles = map[string][]string{
	"grpc":  {"grpcsrv/", "config/grpc.go"},
	"mysql": {"model/", "config/db.go"},
	"redis": {"config/redis.go"},
}

// validComponents 供 flag 校验与测试矩阵使用。
func validComponents() []string {
	return []string{"grpc", "mysql", "redis"}
}

// fileOwnedByDisabled 判断模板相对路径是否归属于某个已关闭的组件。
func fileOwnedByDisabled(rel string, disabled map[string]bool) bool {
	for comp, paths := range componentFiles {
		if !disabled[comp] {
			continue
		}
		for _, p := range paths {
			if strings.HasSuffix(p, "/") {
				if strings.HasPrefix(rel, p) {
					return true
				}
			} else if rel == p {
				return true
			}
		}
	}
	return false
}

// 锚注释行：行首空白 + (// 或 ;) + kate:begin|end + 组件名。
var anchorRe = regexp.MustCompile(`^\s*(//|;)kate:(begin|end)\s+([a-z]+)\s*$`)

// pruneAnchors 按锚注释裁剪文本内容：
//   - 已关闭组件的锚块整体删除（含标记行）；
//   - 开启组件的锚块保留内容、剥除标记行；
//   - 标记不配对 / 交叉嵌套直接报错（模板自身的健壮性由此守住）。
func pruneAnchors(content []byte, path string, disabled map[string]bool) ([]byte, error) {
	lines := strings.SplitAfter(string(content), "\n")
	var out strings.Builder
	var stack []string // 当前嵌套的组件栈
	dropDepth := 0     // >0 表示处于被删除的块内

	for i, line := range lines {
		m := anchorRe.FindStringSubmatch(strings.TrimRight(line, "\n"))
		if m == nil {
			if dropDepth == 0 {
				out.WriteString(line)
			}
			continue
		}
		kind, comp := m[2], m[3]
		switch kind {
		case "begin":
			stack = append(stack, comp)
			if disabled[comp] || dropDepth > 0 {
				dropDepth++
			}
		case "end":
			if len(stack) == 0 || stack[len(stack)-1] != comp {
				return nil, fmt.Errorf("%s:%d: unbalanced anchor //kate:end %s", path, i+1, comp)
			}
			stack = stack[:len(stack)-1]
			if dropDepth > 0 {
				dropDepth--
			}
		}
	}
	if len(stack) != 0 {
		return nil, fmt.Errorf("%s: unclosed anchor for component %q", path, stack[len(stack)-1])
	}
	return []byte(out.String()), nil
}

// prunableFile 判断该文件是否需要做锚注释扫描（其余文件原样落盘）。
func prunableFile(rel string) bool {
	return strings.HasSuffix(rel, ".go") ||
		strings.HasSuffix(rel, ".ini") ||
		strings.HasSuffix(rel, ".sh") ||
		strings.HasSuffix(rel, "Makefile")
}
