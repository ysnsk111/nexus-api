package models

import (
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username       string `gorm:"uniqueIndex;not null"`
	PasswordHash   string `gorm:"not null"`
	Email          string `gorm:"uniqueIndex"`
	Role           string `gorm:"default:user"`   // admin or user
	Status         string `gorm:"default:active"` // active, disabled, banned
	TotpSecret     string                         // TOTP secret (empty = disabled)
	TotpEnabled    bool   `gorm:"default:false"`
	// Token quotas
	TokensTotal    int64 `gorm:"default:100"` // Total lifetime quota (-1 = unlimited)
	TokensUsed     int64 `gorm:"default:0"`   // Total tokens used
	TokensWeekly   int64 `gorm:"default:-1"`  // Weekly quota (-1 = unlimited)
	TokensWeekUsed int64 `gorm:"default:0"`   // Used this week
	TokensWeekReset int64 `gorm:"default:0"`  // Unix timestamp of weekly window start
	TokensMonthly  int64 `gorm:"default:-1"`  // Monthly quota (-1 = unlimited)
	TokensMonthUsed int64 `gorm:"default:0"`  // Used this month
	TokensMonthReset int64 `gorm:"default:0"` // Unix timestamp of monthly window start
	Tokens5h       int64 `gorm:"default:-1"`  // 5-hour quota (-1 = unlimited)
	Tokens5hUsed   int64 `gorm:"default:0"`   // Used in last 5 hours
	Tokens5hReset  int64 `gorm:"default:0"`   // Unix timestamp of 5h window start
	// Model permissions
	AllowedModels  string `gorm:"default:*"` // comma-separated or "*" for all
	// Routing rules
	RoutingEnabled bool   `gorm:"default:true"`
	RoutingRules   string `gorm:"default:\"\""` // JSON array of rule names
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}
