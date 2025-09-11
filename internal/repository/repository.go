package repository

import (
	"context"

	"L0/internal/models"
)

type OrderRepository interface {
	Create(ctx context.Context, order models.Order) error
	GetByUID(ctx context.Context, uid string) (models.Order, error)
	GetLatest(ctx context.Context, limit int) ([]models.Order, error)
}
