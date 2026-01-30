package models

import (
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(databaseURL string) {
	var err error
	DB, err = gorm.Open(sqlite.Open(databaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto Migrate the schema
	err = DB.AutoMigrate(&KeyHistory{})
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
}
