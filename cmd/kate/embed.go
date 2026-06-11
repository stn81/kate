package main

import "embed"

// templateFS 内嵌整个模板树。
//
// 约束（破坏任意一条都会让脚手架静默缺文件）：
//   - go:embed 是包目录作用域：模板必须住在本包子树内（cmd/kate/template/...）。
//   - 模板内不允许出现 go.mod —— 会被当嵌套模块从父模块编译与 embed 中同时剔除；
//     生成项目的 go.mod 由 `go mod init` 现场生成。
//   - 点文件需 all: 前缀才会被收入；.gitignore 以无点文件名 gitignore 存放
//     （避免它反噬 kate 仓自身的 git 跟踪），落盘时按 renameOnMaterialize 还原。
//   - embed 不收空目录：每个模板目录必须至少有一个文件。
//
// 模板文件清单的完整性由 cmd/kate 的生成测试断言（防新增文件被静默丢弃）。
//
//go:embed all:template
var templateFS embed.FS

// templateRoot 是 embed 内的模板根；生成项目 = 该子树的实例化。
const templateRoot = "template/service"

// templateModule 是模板在 kate 主模块内的真实 import 前缀，实例化时被语义改写为目标 module。
const templateModule = "github.com/stn81/kate/cmd/kate/template/service"

// templateAppName 是模板里 app 目录与构建脚本锚定行使用的规范 app 名。
const templateAppName = "kateapp"

// renameOnMaterialize 落盘时的文件名还原表。
var renameOnMaterialize = map[string]string{
	"gitignore": ".gitignore",
}
