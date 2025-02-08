package database

import (
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const maxRetry = 60 // 60 * 5s = 300s (5m)

var db *gorm.DB

func Db() *gorm.DB {
	if db == nil {
		lInterface := logger.New(
			log.New(os.Stdout, "\r\n", log.Ldate|log.Ltime),
			logger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  logger.Info,
				IgnoreRecordNotFoundError: false,
				ParameterizedQueries:      false,
				Colorful:                  true,
			},
		)
		var err error

		for range maxRetry {
			db, err = gorm.Open(postgres.Open(os.Getenv("DB_DSN")), &gorm.Config{
				Logger: lInterface,
			})
			if err != nil {
				time.Sleep(5 * time.Second)
				continue
			}

			return db
		}

		return nil
	}

	return db
}

func SetupDatabase() {
	sqlDB, err := Db().DB()
	if err != nil {
		panic("Failed to get sql.DB")
	}

	if sqlDB == nil {
		panic("sqlDB is nil")
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
}
