package bb_storage

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/0xdevelop/bioBox/bb_storage/bb_storage_config"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_sqlite"
)

func openSqlite(cfg *bb_storage_config.StorageSqliteConfig) error {
	if cfg == nil {
		return errors.New("storage_sqlite is not configured")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBFilePath), 0755); err != nil {
		return err
	}
	opened, err := gtbox_orm_sqlite.New(cfg.GTORMSqliteConfig)
	if err != nil {
		return err
	}
	opened.SqliteDB.Config.NowFunc = func() time.Time { return time.Now().UTC() }
	pool, err := opened.SqliteDB.DB()
	if err != nil {
		_ = opened.Close()
		return err
	}
	// SQLite has one writer. A single connection avoids deferred transaction upgrade contention.
	pool.SetMaxOpenConns(1)
	pool.SetMaxIdleConns(1)
	globalSqlite = opened
	return nil
}
