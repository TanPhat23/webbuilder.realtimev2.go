package entity

import (
	"time"

	"gorm.io/gorm"
)

// Project represents a workspace/project in the system.
// Fields and tags mirror the provided schema. Related types are defined as
// lightweight placeholders so the codebase compiles; expand them later as needed.
type Project struct {
	ID              string                 `json:"id" gorm:"primaryKey;column:id"`
	Description     *string                `json:"description,omitempty" gorm:"type:text"`
	Name            string                 `json:"name" gorm:"type:varchar(255)"`
	OwnerID         string                 `json:"ownerId" gorm:"index"`
	Published       bool                   `json:"published" gorm:"default:false;index"`
	Subdomain       *string                `json:"subdomain,omitempty" gorm:"type:varchar(63);index"`
	CreatedAt       time.Time              `json:"createdAt"`
	DeletedAt       gorm.DeletedAt         `json:"deletedAt,omitempty" gorm:"index"`
	UpdatedAt       time.Time              `json:"updatedAt"`
	Styles          map[string]any         `json:"styles,omitempty" gorm:"serializer:json;type:jsonb"`
	Header          map[string]any         `json:"header,omitempty" gorm:"serializer:json;type:jsonb"`

	Collaborators   []Collaborator         `json:"collaborators,omitempty" gorm:"foreignKey:ProjectID"`
	Invitations     []Invitation           `json:"invitations,omitempty" gorm:"foreignKey:ProjectID"`
	MarketplaceItem *MarketplaceItem      `json:"marketplaceItem,omitempty" gorm:"foreignKey:ProjectID"`
	Pages           []Page                 `json:"pages,omitempty" gorm:"foreignKey:ProjectID"`
	Snapshots       []Snapshot             `json:"snapshots,omitempty" gorm:"foreignKey:ProjectID"`
	EventWorkflows  []EventWorkflow        `json:"eventWorkflows,omitempty" gorm:"foreignKey:ProjectID"`
	Owner           *User                  `json:"owner,omitempty" gorm:"foreignKey:OwnerID;references:ID"`
}

// TableName explicitly sets the projects table name.
func (Project) TableName() string {
	return "Project"
}
