package models

import "gorm.io/gorm"

type Setting struct {
	gorm.Model
	Key   string `gorm:"uniqueIndex;not null"`
	Value string
}

func GetSetting(key string, defaultValue string) string {
	var setting Setting
	if err := DB.Where("key = ?", key).First(&setting).Error; err != nil {
		return defaultValue
	}
	return setting.Value
}

func SetSetting(key string, value string) error {
	var setting Setting
	if err := DB.Where("key = ?", key).First(&setting).Error; err != nil {
		setting = Setting{Key: key, Value: value}
		return DB.Create(&setting).Error
	}
	setting.Value = value
	return DB.Save(&setting).Error
}
