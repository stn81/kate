// 模块边界标记：把 skel/（kate_init 的项目模板，含 __PACKAGE_NAME__ 占位符，
// 本身不可编译）从 kate 父模块的 ./... 包树里隔离出去，go build/vet/test 与
// gopls 不再加载本目录。kate_init 复制模板后会先 rm 掉本文件再 go mod init。
module skel

go 1.26.3
