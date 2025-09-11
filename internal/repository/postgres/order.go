package postgres

import (
    "context"
    "fmt"
    "log/slog"
    "strings"

    "github.com/cenkalti/backoff/v4"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"

    "L0/internal/config"
    "L0/internal/models"
)

type Repository struct {
    db     *pgxpool.Pool
    config *config.Config
}

func New(db *pgxpool.Pool, cfg *config.Config) *Repository {
    return &Repository{
        db:     db,
        config: cfg,
    }
}

func (r *Repository) Create(ctx context.Context, order models.Order) error {
    const op = "repository.postgres.Create"

    operation := func() error {
        tx, err := r.db.Begin(ctx)
        if err != nil {
            return fmt.Errorf("failed to begin transaction: %w", err)
        }
        defer func() {
            if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
                slog.Error("failed to rollback transaction", "error", err)
            }
        }()

        orderSQL := `INSERT INTO orders (order_uid, track_number, entry, locale, internal_signature, customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
        if _, err := tx.Exec(ctx, orderSQL, order.OrderUID, order.TrackNumber, order.Entry, order.Locale, order.InternalSignature, order.CustomerID, order.DeliveryService, order.Shardkey, order.SmID, order.DateCreated, order.OofShard); err != nil {
            return fmt.Errorf("%s: %w", op, err)
        }

        deliverySQL := `INSERT INTO deliveries (order_uid, name, phone, zip, city, address, region, email)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
        if _, err := tx.Exec(ctx, deliverySQL, order.OrderUID, order.Delivery.Name, order.Delivery.Phone, order.Delivery.Zip, order.Delivery.City, order.Delivery.Address, order.Delivery.Region, order.Delivery.Email); err != nil {
            return fmt.Errorf("%s: %w", op, err)
        }

        paymentSQL := `INSERT INTO payments (order_uid, transaction, request_id, currency, provider, amount, payment_dt, bank, delivery_cost, goods_total, custom_fee)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
        if _, err := tx.Exec(ctx, paymentSQL, order.OrderUID, order.Payment.Transaction, order.Payment.RequestID, order.Payment.Currency, order.Payment.Provider, order.Payment.Amount, order.Payment.PaymentDt, order.Payment.Bank, order.Payment.DeliveryCost, order.Payment.GoodsTotal, order.Payment.CustomFee); err != nil {
            return fmt.Errorf("%s: %w", op, err)
        }

        itemSQL := `INSERT INTO items (chrt_id, order_uid, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
        for _, item := range order.Items {
            if _, err := tx.Exec(ctx, itemSQL, item.ChrtID, order.OrderUID, item.TrackNumber, item.Price, item.Rid, item.Name, item.Sale, item.Size, item.TotalPrice, item.NmID, item.Brand, item.Status); err != nil {
                return fmt.Errorf("%s: %w", op, err)
            }
        }

        if err := tx.Commit(ctx); err != nil {
            return fmt.Errorf("failed to commit transaction: %w", err)
        }
        return nil
    }

    // Retry только для временных ошибок (connection issues)
    bo := backoff.NewExponentialBackOff()
    bo.MaxElapsedTime = r.config.Retry.MaxElapsedTimeDB
    bo.InitialInterval = r.config.Retry.InitialInterval
    bo.MaxInterval = r.config.Retry.MaxIntervalDB

    retryable := func() error {
        err := operation()
        if err != nil {
            // Не retry constraint violations (дубликаты)
            if strings.Contains(err.Error(), "duplicate key") {
                return backoff.Permanent(err)
            }
            slog.Warn("Database operation failed, retrying...", "error", err)
        }
        return err
    }

    return backoff.Retry(retryable, backoff.WithContext(bo, ctx))
}

func (r *Repository) GetByUID(ctx context.Context, uid string) (models.Order, error) {
    const op = "repository.postgres.GetByUID"

    operation := func() (models.Order, error) {
        tx, err := r.db.Begin(ctx)
        if err != nil {
            return models.Order{}, fmt.Errorf("%s: %w", op, err)
        }
        defer func() {
            if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
                slog.Error("failed to rollback transaction", "error", err)
            }
        }()

        orderSQL := `SELECT order_uid, track_number, entry, locale, internal_signature, customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard
            FROM orders WHERE order_uid = $1`
        var order models.Order
        err = tx.QueryRow(ctx, orderSQL, uid).Scan(
            &order.OrderUID, &order.TrackNumber, &order.Entry, &order.Locale,
            &order.InternalSignature, &order.CustomerID, &order.DeliveryService,
            &order.Shardkey, &order.SmID, &order.DateCreated, &order.OofShard)
        if err != nil {
            if err == pgx.ErrNoRows {
                return models.Order{}, fmt.Errorf("order not found")
            }
            return models.Order{}, fmt.Errorf("%s: %w", op, err)
        }

        // Получаем связанные данные (delivery, payment, items)
        if err := r.queryDeliveryPaymentItems(ctx, tx, map[string]*models.Order{uid: &order}, []string{uid}); err != nil {
            return models.Order{}, fmt.Errorf("%s: %w", op, err)
        }

        if err := tx.Commit(ctx); err != nil {
            return models.Order{}, fmt.Errorf("%s: commit: %w", op, err)
        }

        return order, nil
    }

    // Retry для read операций
    bo := backoff.NewExponentialBackOff()
    bo.MaxElapsedTime = r.config.Retry.MaxElapsedTimeRead
    bo.InitialInterval = r.config.Retry.InitialInterval
    bo.MaxInterval = r.config.Retry.MaxIntervalRead

    var result models.Order
    retryable := func() error {
        order, err := operation()
        if err != nil {
            if strings.Contains(err.Error(), "order not found") {
                return backoff.Permanent(err)
            }
            slog.Warn("Database read operation failed, retrying...", "error", err)
            return err
        }
        result = order
        return nil
    }

    err := backoff.Retry(retryable, backoff.WithContext(bo, ctx))
    if err != nil {
        return models.Order{}, err
    }
    return result, nil
}

func (r *Repository) GetLatest(ctx context.Context, limit int) ([]models.Order, error) {
    const op = "repository.postgres.GetLatest"

    operation := func() ([]models.Order, error) {
        tx, err := r.db.Begin(ctx)
        if err != nil {
            return nil, fmt.Errorf("%s: %w", op, err)
        }
        defer func() {
            if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
                slog.Error("failed to rollback transaction", "error", err)
            }
        }()

        orderRows, err := tx.Query(ctx, `
            SELECT order_uid, track_number, entry, locale, internal_signature, customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard
            FROM orders
            ORDER BY date_created DESC
            LIMIT $1
        `, limit)
        if err != nil {
            return nil, fmt.Errorf("%s: query orders: %w", op, err)
        }
        defer orderRows.Close()

        var orders []models.Order
        orderMap := make(map[string]*models.Order)

        for orderRows.Next() {
            var o models.Order
            if err := orderRows.Scan(&o.OrderUID, &o.TrackNumber, &o.Entry, &o.Locale, &o.InternalSignature, &o.CustomerID, &o.DeliveryService, &o.Shardkey, &o.SmID, &o.DateCreated, &o.OofShard); err != nil {
                return nil, fmt.Errorf("%s: scan order: %w", op, err)
            }
            orders = append(orders, o)
            orderMap[o.OrderUID] = &orders[len(orders)-1]
        }
        if err := orderRows.Err(); err != nil {
            return nil, fmt.Errorf("%s: iterate orders: %w", op, err)
        }

        if len(orders) == 0 {
            if err := tx.Commit(ctx); err != nil {
                return nil, fmt.Errorf("%s: commit: %w", op, err)
            }
            return []models.Order{}, nil
        }

        orderUIDs := make([]string, 0, len(orders))
        for _, o := range orders {
            orderUIDs = append(orderUIDs, o.OrderUID)
        }

        // Получаем связанные данные для всех заказов
        if err := r.queryDeliveryPaymentItems(ctx, tx, orderMap, orderUIDs); err != nil {
            return nil, fmt.Errorf("%s: %w", op, err)
        }

        if err := tx.Commit(ctx); err != nil {
            return nil, fmt.Errorf("%s: commit: %w", op, err)
        }

        return orders, nil
    }

    // Retry для read операций
    bo := backoff.NewExponentialBackOff()
    bo.MaxElapsedTime = r.config.Retry.MaxElapsedTimeRead
    bo.InitialInterval = r.config.Retry.InitialInterval
    bo.MaxInterval = r.config.Retry.MaxIntervalRead

    var result []models.Order
    retryable := func() error {
        orders, err := operation()
        if err != nil {
            slog.Warn("Database read operation failed, retrying...", "error", err)
            return err
        }
        result = orders
        return nil
    }

    err := backoff.Retry(retryable, backoff.WithContext(bo, ctx))
    if err != nil {
        return nil, err
    }
    return result, nil
}

func (r *Repository) queryDeliveryPaymentItems(ctx context.Context, tx pgx.Tx, orderMap map[string]*models.Order, orderUIDs []string) error {
    const op = "repository.postgres.queryDeliveryPaymentItems"

    // Delivery
    deliveryRows, err := tx.Query(ctx, `
        SELECT order_uid, name, phone, zip, city, address, region, email
        FROM deliveries WHERE order_uid = ANY($1)
    `, orderUIDs)
    if err != nil {
        return fmt.Errorf("%s: query deliveries: %w", op, err)
    }
    defer deliveryRows.Close()

    for deliveryRows.Next() {
        var d models.Delivery
        var orderUID string
        if err := deliveryRows.Scan(&orderUID, &d.Name, &d.Phone, &d.Zip, &d.City, &d.Address, &d.Region, &d.Email); err != nil {
            return fmt.Errorf("%s: scan delivery: %w", op, err)
        }
        if order, ok := orderMap[orderUID]; ok {
            order.Delivery = d
        }
    }

    // Payment
    paymentRows, err := tx.Query(ctx, `
        SELECT order_uid, transaction, request_id, currency, provider, amount, payment_dt, bank, delivery_cost, goods_total, custom_fee
        FROM payments WHERE order_uid = ANY($1)
    `, orderUIDs)
    if err != nil {
        return fmt.Errorf("%s: query payments: %w", op, err)
    }
    defer paymentRows.Close()

    for paymentRows.Next() {
        var p models.Payment
        var orderUID string
        if err := paymentRows.Scan(&orderUID, &p.Transaction, &p.RequestID, &p.Currency, &p.Provider, &p.Amount, &p.PaymentDt, &p.Bank, &p.DeliveryCost, &p.GoodsTotal, &p.CustomFee); err != nil {
            return fmt.Errorf("%s: scan payment: %w", op, err)
        }
        if order, ok := orderMap[orderUID]; ok {
            order.Payment = p
        }
    }

    // Items
    itemRows, err := tx.Query(ctx, `
        SELECT order_uid, chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status
        FROM items WHERE order_uid = ANY($1)
    `, orderUIDs)
    if err != nil {
        return fmt.Errorf("%s: query items: %w", op, err)
    }
    defer itemRows.Close()

    for itemRows.Next() {
        var i models.Item
        var orderUID string
        if err := itemRows.Scan(&orderUID, &i.ChrtID, &i.TrackNumber, &i.Price, &i.Rid, &i.Name, &i.Sale, &i.Size, &i.TotalPrice, &i.NmID, &i.Brand, &i.Status); err != nil {
            return fmt.Errorf("%s: scan item: %w", op, err)
        }
        if order, ok := orderMap[orderUID]; ok {
            order.Items = append(order.Items, i)
        }
    }

    return nil
}