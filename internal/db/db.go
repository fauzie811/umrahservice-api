package db

import (
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"umrahservice-api/internal/config"
)

// Open connects to the MariaDB database using GORM. The schema is owned by the
// Laravel app, so AutoMigrate is intentionally never called here.
func Open(cfg *config.Config) (*gorm.DB, error) {
	logLevel := logger.Warn
	if cfg.IsLocal() {
		logLevel = logger.Info
	}

	gdb, err := gorm.Open(mysql.Open(cfg.DB.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return gdb, nil
}
