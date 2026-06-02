MAKEFILE=$(realpath $(lastword $(MAKEFILE_LIST)))
HOME_DIR=$(shell dirname $(MAKEFILE))

OS := $(shell uname -s)

#程序名称
APP  = __APP_NAME__

GO	 = $(GOROOT)/bin/go
LINT = golangci-lint
OUT_DIR = $(HOME_DIR)/outputs
BIN_DIR = $(OUT_DIR)/bin
BIN  = $(BIN_DIR)/$(APP)

__APP_NAME___SRC  = $(HOME_DIR)/app/__APP_NAME__/main.go

#程序版本号
VER_MAJOR   = 1
VER_MINOR   = 0
VER_PATCH   = 0

REVISION	= $(shell git rev-parse HEAD >/dev/null 2>&1 && echo "revision_found")
BUILD_DATE	= $(shell date +%Y-%m-%dT%H:%M:%S)

ifeq ($(strip $(REVISION)),)
REVISION 	= unknown
LAST_AUTHOR = unknown
LAST_DATE 	= unknown
else
REVISION	= $(shell git rev-parse HEAD)
LAST_AUTHOR	= $(shell git --no-pager show -s --format='%ae' $(REVISION))
LAST_T      = $(shell git --no-pager show -s --format='%at' $(REVISION))
ifeq ($(OS), Darwin)
LAST_DATE   = $(shell date -r $(LAST_T) +%Y-%m-%dT%H:%M:%S)
else
LAST_DATE   = $(shell date -d @$(LAST_T) +%Y-%m-%dT%H:%M:%S)
endif
endif

# 注意：Go module 模式下，-ldflags -X 的目标必须是包的导入路径
# （github.com/stn81/kate/app），即使 vendor 也不要写成 vendor 路径，
# 否则 -X 静默不生效、version 命令打印为空。
KATE_APP_PKG=github.com/stn81/kate/app

GCFLAGS =
ifeq ($(debug), 1)
GCFLAGS = all=-N -l
endif

LDFLAGS  = -X $(KATE_APP_PKG).VersionMajor=$(VER_MAJOR)
LDFLAGS += -X $(KATE_APP_PKG).VersionMinor=$(VER_MINOR)
LDFLAGS += -X $(KATE_APP_PKG).VersionPatch=$(VER_PATCH)
LDFLAGS += -X $(KATE_APP_PKG).Revision=$(REVISION)
LDFLAGS += -X $(KATE_APP_PKG).LastAuthor=$(LAST_AUTHOR)
LDFLAGS += -X $(KATE_APP_PKG).LastDate=$(LAST_DATE)
LDFLAGS += -X $(KATE_APP_PKG).BuildDate=$(BUILD_DATE)

all: 
	@echo "make: PWD=$(shell pwd)"
	@echo "make: GOROOT=$(GOROOT)"
	@echo "make: GOPATH=$(GOPATH)"
	@echo "make: CGO_CFLAGS=$(CGO_CFLAGS)"
	@echo "make: CGO_LDFLAGS=$(CGO_LDFLAGS)"
	@echo "make: LIBRARY_PATH=$(LIBRARY_PATH)"
	@echo "make: LD_LIBRARY_PATH=$(LD_LIBRARY_PATH)"
	$(GO) build -mod vendor -gcflags="$(GCFLAGS)" -ldflags "$(LDFLAGS)" -o $(BIN) $(__APP_NAME___SRC)

lint:
	$(LINT) run

clean:
	rm -rf $(OUT_DIR)
