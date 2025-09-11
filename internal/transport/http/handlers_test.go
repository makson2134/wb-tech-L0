package http

import (
    "context"
    "errors"
    "net/http"
    "net/http/httptest"
    "testing"

    "L0/internal/models"

    "github.com/go-chi/chi/v5"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

// MockOrderService - мок для сервиса заказов
type MockOrderService struct {
    mock.Mock
}

func (m *MockOrderService) GetByUID(ctx context.Context, uid string) (models.Order, error) {
    args := m.Called(ctx, uid)
    return args.Get(0).(models.Order), args.Error(1)
}

func (m *MockOrderService) Create(ctx context.Context, order models.Order) error {
    args := m.Called(ctx, order)
    return args.Error(0)
}

func (m *MockOrderService) GetLatest(ctx context.Context, limit int) ([]models.Order, error) {
    args := m.Called(ctx, limit)
    return args.Get(0).([]models.Order), args.Error(1)
}

func TestGetOrderByPath_Success(t *testing.T) {
    mockService := &MockOrderService{}

    testOrder := models.Order{
        OrderUID:    "test-order-123",
        TrackNumber: "TRACK123",
        CustomerID:  "customer-1",
    }

    mockService.On("GetByUID", mock.Anything, "test-order-123").Return(testOrder, nil)

    // Создаем handler без template 
    handler := &OrderHandler{service: mockService}

    r := chi.NewRouter()
    r.Get("/order/{order_uid}", handler.GetOrderByPath)

    req := httptest.NewRequest("GET", "/order/test-order-123", nil)
    w := httptest.NewRecorder()

    r.ServeHTTP(w, req)

    mockService.AssertExpectations(t)
    assert.Equal(t, http.StatusOK, w.Code)
    assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestGetOrderByPath_NotFound(t *testing.T) {
    mockService := &MockOrderService{}

    mockService.On("GetByUID", mock.Anything, "non-existent").Return(models.Order{}, models.OrderNotFoundError{OrderUID: "non-existent"})

    handler := &OrderHandler{service: mockService}

    r := chi.NewRouter()
    r.Get("/order/{order_uid}", handler.GetOrderByPath)

    req := httptest.NewRequest("GET", "/order/non-existent", nil)
    w := httptest.NewRecorder()

    r.ServeHTTP(w, req)

    mockService.AssertExpectations(t)
    assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetOrderByPath_ServiceError(t *testing.T) {
    mockService := &MockOrderService{}

    mockService.On("GetByUID", mock.Anything, "test-uid").Return(models.Order{}, errors.New("database error"))

    handler := &OrderHandler{service: mockService}

    r := chi.NewRouter()
    r.Get("/order/{order_uid}", handler.GetOrderByPath)

    req := httptest.NewRequest("GET", "/order/test-uid", nil)
    w := httptest.NewRecorder()

    r.ServeHTTP(w, req)

    mockService.AssertExpectations(t)
    assert.Equal(t, http.StatusInternalServerError, w.Code)
}