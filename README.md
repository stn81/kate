kate service framework.

## 新建项目（脚手架）

无需 clone 本仓库：

```bash
go run github.com/stn81/kate/cmd/kate@latest new <module> [flags]

# 例：
go run github.com/stn81/kate/cmd/kate@latest new dw-sync2
go run github.com/stn81/kate/cmd/kate@latest new github.com/acme/logcollect -grpc -redis=false
```

- 组件开关：`-grpc`（默认关）、`-mysql` / `-redis`（默认开，`-mysql=false` 关闭）
- 模板是本仓库 `cmd/kate/template/service/` 下**真实可编译的参考实现**，与框架同仓同 tag 演进，
  CI 持续编译、测试并对组件全组合做生成验证 —— 模板不会腐烂。
- 生成项目锁定与所用 CLI 同版本的 kate（`@v1.x.y` 生成的项目 require kate v1.x.y）。
- 生成后：`./scripts/build.sh dev && ./outputs/bin/<app> start`
