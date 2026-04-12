package model

import (
	"time"

	"github.com/google/uuid"
)

// Category is the domain model and GraphQL bound type for product categories.
type Category struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name      string     `json:"name" gorm:"size:120;not null;uniqueIndex"`
	CreatedAt time.Time  `json:"createdAt" gorm:"not null;default:now()"`
	Products  []*Product `json:"products,omitempty" gorm:"foreignKey:CategoryID"`
}

func (Category) TableName() string {
	return "categories"
}
