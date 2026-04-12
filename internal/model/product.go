package model

import (
	"time"

	"github.com/google/uuid"
)

// Product is the domain model and GraphQL bound type for catalog products.
type Product struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name          string    `json:"name" gorm:"size:255;not null;index:idx_products_name"`
	Description   *string   `json:"description" gorm:"type:text"`
	Price         float64   `json:"price" gorm:"type:numeric(12,2);not null"`
	Currency      string    `json:"currency" gorm:"size:3;not null;default:USD"`
	CategoryID    uuid.UUID `json:"categoryId" gorm:"type:uuid;not null;index:idx_products_category_id"`
	Category      *Category `json:"category,omitempty" gorm:"foreignKey:CategoryID"`
	StockQuantity int       `json:"stockQuantity" gorm:"not null;default:0"`
	CreatedAt     time.Time `json:"createdAt" gorm:"not null;default:now()"`
	UpdatedAt     time.Time `json:"updatedAt" gorm:"not null;default:now()"`
}

func (Product) TableName() string {
	return "products"
}
