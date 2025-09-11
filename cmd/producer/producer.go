package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"L0/internal/config"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

func main() {
	
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.MustLoad()

	runtime.GOMAXPROCS(1)

	validData, invalidData, err := loadTestData(cfg.Producer.DataPath)
	if err != nil {
		slog.Error("Failed to load test data", "error", err)
		os.Exit(1)
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Kafka.Brokers...),
		Topic:        cfg.Kafka.Topic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
	}
	defer writer.Close()

	slog.Info("Producer started. Sending messages...", "delay", cfg.Producer.Delay)

	ticker := time.NewTicker(cfg.Producer.Delay)
	defer ticker.Stop()

	messageCount := 0

	for range ticker.C {
		var msg kafka.Message
		var msgType string

		messageCount++

		if messageCount%3 != 0 {
			msgType = "VALID"
			orderData, orderUID := generateValidOrder(validData)
			msg = kafka.Message{
				Key:   []byte(orderUID),
				Value: orderData,
			}
		} else {
			msgType = "INVALID"
			invalidMsg := invalidData[rand.Intn(len(invalidData))]
			msg = kafka.Message{
				Key:   []byte(uuid.NewString()),
				Value: invalidMsg,
			}
		}

		// Retry отправки сообщения с backoff
		sendOperation := func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return writer.WriteMessages(ctx, msg)
		}

		bo := backoff.NewExponentialBackOff()
		bo.MaxElapsedTime = 15 * time.Second
		bo.InitialInterval = 500 * time.Millisecond
		bo.MaxInterval = 3 * time.Second

		err := backoff.Retry(sendOperation, bo)
		if err != nil {
			slog.Error("Failed to send message after retries", "type", msgType, "error", err)
		} else {
			slog.Info("Sent message", "type", msgType)
		}
	}
}

func loadTestData(dataPath string) (map[string]interface{}, [][]byte, error) {
	validPath := filepath.Join(dataPath, "valid-order-template.json")
	validFile, err := os.ReadFile(validPath)
	if err != nil {
		return nil, nil, err
	}

	var validData map[string]interface{}
	if err := json.Unmarshal(validFile, &validData); err != nil {
		return nil, nil, err
	}

	var invalidData [][]byte
	invalidFiles, err := filepath.Glob(filepath.Join(dataPath, "error-*.json"))
	if err != nil {
		return nil, nil, err
	}

	for _, file := range invalidFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, nil, err
		}
		invalidData = append(invalidData, data)
	}

	return validData, invalidData, nil
}

func generateValidOrder(template map[string]interface{}) ([]byte, string) {
	orderData := make(map[string]interface{})
	for k, v := range template {
		orderData[k] = v
	}

	newUID := uuid.NewString()
	orderData["order_uid"] = newUID
	orderData["date_created"] = time.Now().UTC().Format(time.RFC3339)

	if payment, ok := orderData["payment"].(map[string]interface{}); ok {
		payment["transaction"] = newUID
	}

	result, _ := json.Marshal(orderData)
	return result, newUID
}
