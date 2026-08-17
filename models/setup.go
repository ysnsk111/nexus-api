package models

import (
	"log"
	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB() {
	dbPath := "/app/data/nexus.db"
	if os.Getenv("ENV") == "dev" {
		dbPath = "nexus.db"
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Enable WAL mode for better concurrent access
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA foreign_keys=ON")

	// Migrate schema
	err = db.AutoMigrate(&User{}, &APIKey{}, &NexusKey{}, &Setting{}, &UsageStat{})
	if err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	DB = db
	InitDefaultSettings()
}

func InitDefaultSettings() {
	defaults := map[string]string{
		"default_tokens":          "100",
		"easter_egg_message":      "我喜欢你喵！",
		"announcement":            "Welcome to NexusAPI Flow!",
		"registration_enabled":    "true",
		"default_allowed_models":  "*",
		"easter_egg_models":       "free_llm_chat",
	}
	for k, v := range defaults {
		if GetSetting(k, "") == "" {
			SetSetting(k, v)
		}
	}
}

func InitDefaultAdmin() {
	// Allow env-override of admin credentials
	adminUser := os.Getenv("ADMIN_USER")
	adminPass := os.Getenv("ADMIN_PASS")
	if adminUser == "" {
		adminUser = "nexusapi"
	}
	if adminPass == "" {
		adminPass = "nexusapi"
	}

	var admin User
	result := DB.Where("role = ?", "admin").First(&admin)
	if result.Error != nil {
		// Create default admin
		admin = User{
			Username:      adminUser,
			Role:          "admin",
			Status:        "active",
			TokensTotal:   -1,
			TokensWeekly:  -1,
			TokensMonthly: -1,
			Tokens5h:      -1,
			AllowedModels: "*",
			RoutingEnabled: true,
		}
		admin.SetPassword(adminPass)
		if err := DB.Create(&admin).Error; err != nil {
			log.Printf("Warning: failed to create admin account: %v", err)
		} else {
			log.Printf("Default admin account created: %s", adminUser)
		}
	}
}
