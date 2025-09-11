package http

import (
    "encoding/json"
    "errors"
    "html/template"
    "log/slog"
    "net/http"

    "L0/internal/models"
    "L0/internal/service"

    "github.com/go-chi/chi/v5"
)

type OrderHandler struct {
    service service.OrderService
    tmpl    *template.Template
}

func NewOrderHandler(srv service.OrderService, templatePath string) (*OrderHandler, error) {
    tmpl, err := template.ParseFiles(templatePath)
    if err != nil {
        return nil, err
    }

    return &OrderHandler{
        service: srv,
        tmpl:    tmpl,
    }, nil
}

type ErrorResponse struct {
    Error string `json:"error"`
}

// GetOrderPage godoc
// @Summary Поиск заказа по UID
// @Description Возвращает HTML-страницу с деталями заказа
// @Tags orders
// @Accept  html
// @Produce html
// @Param order_uid query string true "UID заказа"
// @Success 200 {string} string "HTML страница"
// @Failure 404 {string} string "Заказ не найден"
// @Router / [get]
func (h *OrderHandler) GetOrderPage(w http.ResponseWriter, r *http.Request) {
    uidQuery := r.URL.Query().Get("order_uid")

    pageData := struct {
        UIDQuery string
        Order    *models.Order
        Error    string
    }{
        UIDQuery: uidQuery,
    }

    if uidQuery != "" {
        order, err := h.service.GetByUID(r.Context(), uidQuery)
        if err != nil {
            pageData.Error = err.Error()
        } else {
            pageData.Order = &order
            pageData.Error = ""
        }
    }

    err := h.tmpl.Execute(w, pageData)
    if err != nil {
        slog.Error("failed to execute template", "error", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    }
}

// GetOrderByPath godoc
// @Summary Get order by UID (path parameter)
// @Description Get order information by order UID from URL path
// @Tags orders
// @Accept json
// @Produce json
// @Param order_uid path string true "Order UID"
// @Success 200 {object} models.Order
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /order/{order_uid} [get]
func (h *OrderHandler) GetOrderByPath(w http.ResponseWriter, r *http.Request) {
    orderUID := chi.URLParam(r, "order_uid")
    
    if orderUID == "" {
        writeJSONError(w, "order_uid is required", http.StatusBadRequest)
        return
    }

    order, err := h.service.GetByUID(r.Context(), orderUID)
    if err != nil {
        var notFoundErr models.OrderNotFoundError
        if errors.As(err, &notFoundErr) {
            writeJSONError(w, err.Error(), http.StatusNotFound)
            return
        }
        writeJSONError(w, err.Error(), http.StatusInternalServerError)
        return
    }

    writeJSON(w, order, http.StatusOK)
}

func writeJSON(w http.ResponseWriter, data interface{}, status int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(data); err != nil {
        slog.Error("failed to encode JSON", "error", err)
    }
}

func writeJSONError(w http.ResponseWriter, message string, status int) {
    writeJSON(w, ErrorResponse{Error: message}, status)
}