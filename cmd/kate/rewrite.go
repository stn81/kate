package main

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// rewriteImports 对单个 .go 文件做语义级 import 改写（gonew 同款思路，绝非文本 sed）：
//   - 前缀 templateModule → 目标 module（精确路径段边界：== 或 prefix+"/"，
//     不会误伤 github.com/stn81/kate/app 等框架 import）；
//   - 模板 app 目录段 /app/kateapp → /app/<appName>。
func rewriteImports(src []byte, filename, module, appName string) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}

	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("%s: bad import %s: %w", filename, imp.Path.Value, err)
		}
		var newPath string
		switch {
		case p == templateModule:
			newPath = module
		case strings.HasPrefix(p, templateModule+"/"):
			newPath = module + strings.TrimPrefix(p, templateModule)
		default:
			continue
		}
		newPath = strings.Replace(newPath, "/app/"+templateAppName, "/app/"+appName, 1)
		imp.Path.Value = strconv.Quote(newPath)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, fmt.Errorf("format %s: %w", filename, err)
	}
	return buf.Bytes(), nil
}

// rewriteAnchorLine 替换 Makefile / build.sh 里的 app 名锚定行（整行精确匹配）。
// 锚定行缺失说明模板被改坏，直接报错（这是对模板自身的防腐断言）。
func rewriteAnchorLine(content []byte, path, anchor, replacement string) ([]byte, error) {
	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if line == anchor {
			lines[i] = replacement
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("%s: anchor line %q not found (template broken?)", path, anchor)
	}
	return []byte(strings.Join(lines, "\n")), nil
}
