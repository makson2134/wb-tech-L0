package models

import "strings"

type APIError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

type ErrorResponse struct {
	Error APIError `json:"error"`
}

type OrderNotFoundError struct {
	OrderUID string
}

func (e OrderNotFoundError) Error() string {
	return "order not found: " + e.OrderUID
}

type InvalidOrderDataError struct {
	Field   string
	Message string
}

func (e InvalidOrderDataError) Error() string {
	return "invalid order data: " + e.Field + " - " + e.Message
}

type DatabaseError struct {
	Operation string
	Err       error
}

func (e DatabaseError) Error() string {
	return "database error during " + e.Operation + ": " + e.Err.Error()
}

type KafkaError struct {
	Operation string
	Err       error
}

func (e KafkaError) Error() string {
	return "kafka error during " + e.Operation + ": " + e.Err.Error()
}

type ValidationError struct {
	Errors []string
}

func (e ValidationError) Error() string {
	if len(e.Errors) == 1 {
		return "validation error: " + e.Errors[0]
	}
	return "validation errors: " + strings.Join(e.Errors, "; ")
}
