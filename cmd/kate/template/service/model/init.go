package model

import (
	// import mysql driver
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/stn81/kate/cmd/kate/template/service/config"
)

// DB 是全局数据库句柄（sqlx）。表名、字段映射用 `db` tag，查询直接写 SQL。
var DB *sqlx.DB

// Init initialize the model setting.
func Init(logger *zap.Logger) {
	conf := config.DB

	db, err := sqlx.Connect("mysql", conf.DataSource)
	if err != nil {
		logger.Fatal("connect mysql failed", zap.Error(err))
	}
	db.SetMaxIdleConns(conf.MaxIdleConns)
	db.SetMaxOpenConns(conf.MaxOpenConns)
	db.SetConnMaxLifetime(conf.ConnMaxLifetime)
	DB = db
}

// Uninit close the database handle.
func Uninit() {
	if DB != nil {
		_ = DB.Close()
	}
}
