module github.com/stn81/kate

go 1.21.3

require (
	github.com/cloudflare/tableflip v1.2.3
	github.com/davecgh/go-spew v1.1.1
	github.com/go-sql-driver/mysql v1.7.1
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0
	github.com/modern-go/gls v0.0.0-20220109145502-612d0167dce5
	github.com/redis/go-redis/v9 v9.3.0
	github.com/rogpeppe/fastuuid v1.2.0
	github.com/stn81/dynamic v1.0.0
	github.com/stn81/govalidator v1.0.0
	github.com/stretchr/testify v1.8.4
	go.uber.org/atomic v1.11.0
	go.uber.org/zap v1.26.0
	google.golang.org/grpc v0.0.0-00010101000000-000000000000
	gopkg.in/ini.v1 v1.67.0
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.14.0 // indirect
	golang.org/x/sys v0.11.0 // indirect
	golang.org/x/text v0.12.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace google.golang.org/grpc => github.com/grpc/grpc-go v1.59.0
