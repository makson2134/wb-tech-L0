package service

import (
	"context"
	"log/slog"

	"L0/internal/config"
	"L0/internal/models"
	"L0/internal/repository"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

type OrderService interface {
	GetByUID(ctx context.Context, uid string) (models.Order, error)
	Create(ctx context.Context, order models.Order) error
	GetLatest(ctx context.Context, limit int) ([]models.Order, error)
}

type orderService struct {
	repo  repository.OrderRepository
	cache *expirable.LRU[string, models.Order]
}

func NewOrderService(repo repository.OrderRepository, cfg *config.Config) OrderService {
	cache := expirable.NewLRU[string, models.Order](
		cfg.Cache.Size,
		nil, // без callback'а при удалении
		cfg.Cache.TTL,
	)

	return &orderService{
		repo:  repo,
		cache: cache,
	}
}

func (s *orderService) GetByUID(ctx context.Context, uid string) (models.Order, error) {
	if order, exists := s.cache.Get(uid); exists {
		slog.Info("Order found in cache", "order_uid", uid)
		return order, nil
	}

	slog.Info("Cache miss, querying database", "order_uid", uid)

	order, err := s.repo.GetByUID(ctx, uid)
	if err != nil {
		slog.Error("Order not found in database", "order_uid", uid, "error", err)
		return models.Order{}, models.OrderNotFoundError{OrderUID: uid}
	}

	slog.Info("Order found in database, adding to cache", "order_uid", uid)

	s.cache.Add(uid, order)

	return order, nil
}

func (s *orderService) Create(ctx context.Context, order models.Order) error {
	if err := s.repo.Create(ctx, order); err != nil {
		return err
	}

	slog.Info("Order created, adding to cache", "order_uid", order.OrderUID)

	s.cache.Add(order.OrderUID, order)

	return nil
}

func (s *orderService) GetLatest(ctx context.Context, limit int) ([]models.Order, error) {
	return s.repo.GetLatest(ctx, limit)
}
