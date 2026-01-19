package entity

import (
	"time"
)

type Setting struct {
	ID          string         `json:"id" gorm:"primaryKey;column:Id"`
	ElementID   string         `json:"elementId" gorm:"column:ElementId;uniqueIndex"`
	Name        string         `json:"name" gorm:"type:varchar(255)"`
	SettingType string         `json:"settingType" gorm:"type:varchar(100)"`
	Settings    map[string]any `json:"settings" gorm:"serializer:json;type:jsonb"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	Element     *Element       `json:"element,omitempty" gorm:"foreignKey:ElementID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (Setting) TableName() string {
	return "Setting"
}
