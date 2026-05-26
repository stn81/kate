package db

// Importing the driver packages for their init side effects registers them
// with database/sql. Users who pin a different driver version or registered
// name can override via Config.DriverName.
import (
	_ "github.com/ClickHouse/clickhouse-go/v2"     // registers "clickhouse"
	_ "github.com/go-sql-driver/mysql"             // registers "mysql"
	_ "github.com/lib/pq"                          // registers "postgres"
)
