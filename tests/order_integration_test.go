package tests

import (
    "L0/internal/config"
    "L0/internal/models"
    repoPostgres "L0/internal/repository/postgres"
    "context"
    "os"
    "testing"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go"
    tcPostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
    t.Helper()

    ctx := context.Background()

    pgContainer, err := tcPostgres.Run(ctx,
        "postgres:16-alpine",
        tcPostgres.WithDatabase("test-db"),
        tcPostgres.WithUsername("user"),
        tcPostgres.WithPassword("password"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).
                WithStartupTimeout(5*time.Second),
        ),
    )
    require.NoError(t, err)

    connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)

    pool, err := pgxpool.New(ctx, connStr)
    require.NoError(t, err)

    schema, err := os.ReadFile("../init.sql")
    require.NoError(t, err)
    _, err = pool.Exec(ctx, string(schema))
    require.NoError(t, err)

    cleanup := func() {
        pool.Close()
        err := pgContainer.Terminate(ctx)
        assert.NoError(t, err, "failed to terminate postgres container")
    }

    return pool, cleanup
}

func TestRepository_Integration_Create(t *testing.T) {
    pool, cleanup := setupTestDB(t)
    defer cleanup()

    repo := repoPostgres.New(pool, &config.Config{
        Retry: config.Retry{
            MaxElapsedTimeDB:   5 * time.Second,
            MaxElapsedTimeRead: 3 * time.Second,
            InitialInterval:    100 * time.Millisecond,
            MaxIntervalDB:      1 * time.Second,
            MaxIntervalRead:    500 * time.Millisecond,
        },
    })

    order := models.Order{
        OrderUID:    "integration-test-uid",
        TrackNumber: "TRACK123",
        CustomerID:  "customer-1",
        DateCreated: time.Now().UTC().Truncate(time.Millisecond),
        Delivery: models.Delivery{
            Name:    "Test User",
            Phone:   "+123456789",
            Email:   "test@user.com",
            Address: "123 Test St",
            City:    "Testville",
        },
        Payment: models.Payment{
            Transaction:  "integration-test-uid",
            Currency:     "USD",
            Amount:       1500,
            DeliveryCost: 200,
        },
        Items: []models.Item{
            {ChrtID: 1, TrackNumber: "TRACK123", Price: 500, Name: "Item 1", Brand: "Brand A"},
            {ChrtID: 2, TrackNumber: "TRACK123", Price: 800, Name: "Item 2", Brand: "Brand B"},
        },
    }

    err := repo.Create(context.Background(), order)
    require.NoError(t, err)


    t.Run("verify order", func(t *testing.T) {
        var o models.Order
        row := pool.QueryRow(context.Background(), "SELECT order_uid, track_number, customer_id FROM orders WHERE order_uid = $1", order.OrderUID)
        err := row.Scan(&o.OrderUID, &o.TrackNumber, &o.CustomerID)
        require.NoError(t, err)
        assert.Equal(t, order.OrderUID, o.OrderUID)
        assert.Equal(t, order.TrackNumber, o.TrackNumber)
    })

    t.Run("verify delivery", func(t *testing.T) {
        var d models.Delivery
        row := pool.QueryRow(context.Background(), "SELECT name, phone, email FROM deliveries WHERE order_uid = $1", order.OrderUID)
        err := row.Scan(&d.Name, &d.Phone, &d.Email)
        require.NoError(t, err)
        assert.Equal(t, order.Delivery.Name, d.Name)
    })

    t.Run("verify payment", func(t *testing.T) {
        var p models.Payment
        row := pool.QueryRow(context.Background(), "SELECT order_uid, currency, amount FROM payments WHERE order_uid = $1", order.OrderUID)
        var orderUID string
        err := row.Scan(&orderUID, &p.Currency, &p.Amount)
        require.NoError(t, err)
        assert.Equal(t, order.Payment.Amount, p.Amount)
    })

    t.Run("verify items", func(t *testing.T) {
        rows, err := pool.Query(context.Background(), "SELECT chrt_id, name, brand FROM items WHERE order_uid = $1 ORDER BY chrt_id", order.OrderUID)
        require.NoError(t, err)
        defer rows.Close()

        var items []models.Item
        for rows.Next() {
            var i models.Item
            err := rows.Scan(&i.ChrtID, &i.Name, &i.Brand)
            require.NoError(t, err)
            items = append(items, i)
        }
        require.NoError(t, rows.Err())
        require.Len(t, items, 2)
        assert.Equal(t, order.Items[0].Name, items[0].Name)
        assert.Equal(t, order.Items[1].Brand, items[1].Brand)
    })
}