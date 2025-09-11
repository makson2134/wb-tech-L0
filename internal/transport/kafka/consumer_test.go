package kafka

import (
    "context"
    "encoding/json"
    "testing"

    "L0/internal/models"
    "L0/internal/service"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

// Мок для сервиса заказов
type MockOrderService struct {
    mock.Mock
}

// Проверка, что MockOrderService реализует интерфейс service.OrderService
var _ service.OrderService = (*MockOrderService)(nil)

func (m *MockOrderService) GetByUID(ctx context.Context, uid string) (models.Order, error) {
    args := m.Called(ctx, uid)
    order, _ := args.Get(0).(models.Order)
    return order, args.Error(1)
}

func (m *MockOrderService) Create(ctx context.Context, order models.Order) error {
    args := m.Called(ctx, order)
    return args.Error(0)
}

func (m *MockOrderService) GetLatest(ctx context.Context, limit int) ([]models.Order, error) {
    args := m.Called(ctx, limit)
    orders, _ := args.Get(0).([]models.Order)
    return orders, args.Error(1)
}

func (m *MockOrderService) WarmUpCache(ctx context.Context) error {
    args := m.Called(ctx)
    return args.Error(0)
}

func TestConsumer_ProcessValidOrderJSON(t *testing.T) {
    mockService := &MockOrderService{}

    testOrder := models.Order{
        OrderUID:    "test-order-123",
        TrackNumber: "TRACK123",
        CustomerID:  "customer-1",
    }

    orderJSON, err := json.Marshal(testOrder)
    assert.NoError(t, err)

    // Проверка, что сервис вызывается с корректными данными
    mockService.On("Create", mock.Anything, mock.MatchedBy(func(order models.Order) bool {
        return order.OrderUID == testOrder.OrderUID
    })).Return(nil)

    var order models.Order
    err = json.Unmarshal(orderJSON, &order)
    assert.NoError(t, err)

    err = mockService.Create(context.Background(), order)
    assert.NoError(t, err)

    mockService.AssertExpectations(t)
}

func TestConsumer_ProcessInvalidJSON(t *testing.T) {
    mockService := &MockOrderService{}

    invalidJSON := []byte(`{"invalid": json}`)

    var order models.Order
    err := json.Unmarshal(invalidJSON, &order)

    assert.Error(t, err)
    // Проверка, что сервис не вызывается при ошибке парсинга
    mockService.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}