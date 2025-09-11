package service

import (
    "context"
    "errors"
    "testing"
    "time"

    "L0/internal/config"
    "L0/internal/models"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func createTestConfig() *config.Config {
    return &config.Config{
        Cache: config.Cache{
            Size: 1000,
            TTL:  30 * time.Minute,
        },
    }
}

type MockOrderRepository struct {
    mock.Mock
}

func (m *MockOrderRepository) Create(ctx context.Context, order models.Order) error {
    args := m.Called(ctx, order)
    return args.Error(0)
}

func (m *MockOrderRepository) GetByUID(ctx context.Context, uid string) (models.Order, error) {
    args := m.Called(ctx, uid)
    return args.Get(0).(models.Order), args.Error(1)
}

func (m *MockOrderRepository) GetLatest(ctx context.Context, limit int) ([]models.Order, error) {
    args := m.Called(ctx, limit)
    return args.Get(0).([]models.Order), args.Error(1)
}

func TestOrderService_GetByUID_FromCache(t *testing.T) {
    mockRepo := &MockOrderRepository{}
    cfg := createTestConfig()
    service := NewOrderService(mockRepo, cfg)

    testOrder := models.Order{OrderUID: "test-uid"}
    ctx := context.Background()

    mockRepo.On("Create", ctx, testOrder).Return(nil).Once()
    err := service.Create(ctx, testOrder)
    assert.NoError(t, err)

    result, err := service.GetByUID(ctx, "test-uid")
    assert.NoError(t, err)
    assert.Equal(t, testOrder.OrderUID, result.OrderUID)

    mockRepo.AssertNotCalled(t, "GetByUID", mock.Anything, mock.Anything)
}

func TestOrderService_GetByUID_FromRepository(t *testing.T) {
    mockRepo := &MockOrderRepository{}
    cfg := createTestConfig()
    service := NewOrderService(mockRepo, cfg)

    testOrder := models.Order{OrderUID: "test-uid"}
    ctx := context.Background()

    mockRepo.On("GetByUID", ctx, "test-uid").Return(testOrder, nil).Once()

    result, err := service.GetByUID(ctx, "test-uid")
    assert.NoError(t, err)
    assert.Equal(t, testOrder.OrderUID, result.OrderUID)

    mockRepo.AssertExpectations(t)
}

func TestOrderService_GetByUID_OrderNotFound(t *testing.T) {
    mockRepo := &MockOrderRepository{}
    cfg := createTestConfig()
    service := NewOrderService(mockRepo, cfg)

    ctx := context.Background()

    // Проверяем, что возвращается кастомная ошибка, если заказ не найден
    mockRepo.On("GetByUID", ctx, "non-existent-uid").Return(models.Order{}, errors.New("not found")).Once()

    _, err := service.GetByUID(ctx, "non-existent-uid")
    assert.Error(t, err)
    assert.IsType(t, models.OrderNotFoundError{}, err)
    assert.Contains(t, err.Error(), "non-existent-uid")

    mockRepo.AssertExpectations(t)
}

func TestOrderService_Create_Success(t *testing.T) {
    mockRepo := &MockOrderRepository{}
    cfg := createTestConfig()
    service := NewOrderService(mockRepo, cfg)

    testOrder := models.Order{OrderUID: "test-uid"}
    ctx := context.Background()

    mockRepo.On("Create", ctx, testOrder).Return(nil).Once()

    err := service.Create(ctx, testOrder)
    assert.NoError(t, err)

    mockRepo.AssertExpectations(t)

    cachedOrder, err := service.GetByUID(ctx, "test-uid")
    assert.NoError(t, err)
    assert.Equal(t, testOrder.OrderUID, cachedOrder.OrderUID)
}

func TestOrderService_Create_Error(t *testing.T) {
    mockRepo := &MockOrderRepository{}
    cfg := createTestConfig()
    service := NewOrderService(mockRepo, cfg)

    testOrder := models.Order{OrderUID: "test-uid"}
    ctx := context.Background()

    mockRepo.On("Create", ctx, testOrder).Return(errors.New("database error")).Once()

    err := service.Create(ctx, testOrder)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "database error")

    mockRepo.AssertExpectations(t)
}

func TestOrderService_GetLatest(t *testing.T) {
    mockRepo := &MockOrderRepository{}
    cfg := createTestConfig()
    service := NewOrderService(mockRepo, cfg)

    orders := []models.Order{
        {OrderUID: "uid1"},
        {OrderUID: "uid2"},
    }
    ctx := context.Background()
    limit := 10

    mockRepo.On("GetLatest", ctx, limit).Return(orders, nil).Once()

    result, err := service.GetLatest(ctx, limit)
    assert.NoError(t, err)
    assert.Len(t, result, 2)
    assert.Equal(t, "uid1", result[0].OrderUID)
    assert.Equal(t, "uid2", result[1].OrderUID)

    mockRepo.AssertExpectations(t)
}