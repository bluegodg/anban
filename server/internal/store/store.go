// Package store 提供安伴自有 DB 的连接与迁移。
// 纪律：只有本包知道数据库存在；只放连接/迁移/通用助手，不放任何域的业务表逻辑。
package store

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type Store struct {
	DB *gorm.DB
}

// Open 打开一个 sqlite 库。dsn 为文件路径，或 ":memory:" 用于测试。
func Open(dsn string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	// SQLite 只支持单写者；GORM 默认连接池不限连接数，并发读写会在文件锁上
	// 严重排队（实测单条查询被拖到 1-2s），极端情况下还可能报 "database is locked"。
	// 串行化到单连接是 Go+SQLite 的标准安全设置：所有访问经一个连接排队，每条都快。
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	return &Store{DB: db}, nil
}

// AutoMigrate 由各域在装配时传入自己的模型，建表/改表。
func (s *Store) AutoMigrate(models ...any) error {
	return s.DB.AutoMigrate(models...)
}
