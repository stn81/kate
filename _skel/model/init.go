package model

import (

	// import mysql driver
	_ "github.com/go-sql-driver/mysql"
	"github.com/stn81/kate/orm"
	"go.uber.org/zap"

	"__PACKAGE_NAME__/config"
)

// Init initialize the model setting.
func Init(logger *zap.Logger) {
	conf := config.DB

	orm.Debug = true
	orm.RegisterDB("default", "mysql", conf.DataSource, conf.MaxIdleConns, conf.MaxOpenConns, conf.ConnMaxLifetime)
}
