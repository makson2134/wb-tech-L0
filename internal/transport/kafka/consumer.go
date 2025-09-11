package kafka

import (
    "context"
    "encoding/json"
    "log/slog"
    "time"

    "L0/internal/config"
    "L0/internal/models"
    "L0/internal/service"

    "github.com/cenkalti/backoff/v4"
    "github.com/segmentio/kafka-go"
)

type Consumer struct {
    service service.OrderService
    reader  *kafka.Reader
    cfg     *config.Config
}

func NewConsumer(srv service.OrderService, cfg *config.Config) *Consumer {
    r := kafka.NewReader(kafka.ReaderConfig{
        Brokers:  cfg.Kafka.Brokers,
        Topic:    cfg.Kafka.Topic,
        GroupID:  "l0-orders-group",
        MinBytes: 10e3, // 10кб
        MaxBytes: 10e6, // 10мб
    })

    return &Consumer{
        service: srv,
        reader:  r,
        cfg:     cfg,
    }
}

func (c *Consumer) Run(ctx context.Context) {
    slog.Info("Starting Kafka consumer...")

    for {
        select {
        case <-ctx.Done():
            slog.Info("Consumer context cancelled, stopping...")
            return
        default:
        }

        var m kafka.Message
        var err error

        operation := func() error {
            m, err = c.reader.ReadMessage(ctx)
            if err != nil {
                slog.Warn("failed to read message from kafka, retrying...", "error", err)
                return err
            }
            return nil
        }

        bo := backoff.NewExponentialBackOff()
        bo.MaxElapsedTime = 30 * time.Second
        bo.InitialInterval = 1 * time.Second
        bo.MaxInterval = 5 * time.Second

        if err := backoff.Retry(operation, backoff.WithContext(bo, ctx)); err != nil {
            slog.Error("failed to read message after retries", "error", err)
            continue
        }

        var order models.Order
        if err := json.Unmarshal(m.Value, &order); err != nil {
            slog.Error("failed to unmarshal order", "error", err, "message_value", string(m.Value))
            continue
        }

        saveOperation := func() error {
            return c.service.Create(ctx, order)
        }

        saveBo := backoff.NewExponentialBackOff()
        saveBo.MaxElapsedTime = 10 * time.Second
        saveBo.InitialInterval = 500 * time.Millisecond
        saveBo.MaxInterval = 2 * time.Second

        if err := backoff.Retry(saveOperation, backoff.WithContext(saveBo, ctx)); err != nil {
            slog.Error("failed to save order after retries", "error", err, "order_uid", order.OrderUID)
            continue
        }

        slog.Info("Successfully processed order", "order_uid", order.OrderUID)

        if err := c.reader.CommitMessages(ctx, m); err != nil {
            slog.Error("failed to commit message", "error", err)
        }
    }
}

func (c *Consumer) Close() {
    slog.Info("Closing Kafka consumer...")

    // kafka-go reader сам обрабатывает graceful shutdown
    if err := c.reader.Close(); err != nil {
        slog.Error("failed to close kafka reader", "error", err)
    }

    slog.Info("Kafka consumer closed.")
}