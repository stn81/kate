// kate 是框架自带的项目脚手架 CLI。
//
// 冷启动（无需 clone 本仓库）：
//
//	go run github.com/stn81/kate/cmd/kate@latest new <module> [flags]
//
// 模板内嵌在二进制里（embed），内容与 CLI、kate 框架同 tag 原子绑定：
// @vX.Y.Z 拉到的二进制，生成的项目即锁定 require kate vX.Y.Z。
package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

const usage = `kate - scaffold for kate framework services

Usage:
  kate new <module> [flags]   create a new service project
  kate version                print version

Flags of new:
  -name string   app name (default: basename of module path)
  -dir string    target directory (default: ./<app name>; must be empty or absent)
  -grpc          include grpc server component (default false)
  -mysql         include mysql/model component (default true; disable: -mysql=false)
  -redis         include redis component (default true; disable: -redis=false)
  -git           git init + initial commit (default true; disable: -git=false)

Environment:
  KATE_NEW_REPLACE   if set to a local kate repo path, the generated go.mod
                     gets "replace github.com/stn81/kate => <path>" (CI / local dev)

Examples:
  go run github.com/stn81/kate/cmd/kate@latest new dw-sync2
  kate new github.com/acme/logcollect -name logcollect -grpc -redis=false
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "new":
		if err := runNew(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "kate new: %v\n", err)
			os.Exit(1)
		}
	case "version", "-v", "--version":
		fmt.Println(kateVersion())
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}

// kateVersion 返回 CLI 自身（= kate 模块）的版本。
// go run/install module@version 构建时由 toolchain 盖戳；本地源码构建为 (devel)。
func kateVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return "(devel)"
}
