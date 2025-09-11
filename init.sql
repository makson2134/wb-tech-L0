
CREATE TABLE IF NOT EXISTS orders (
    order_uid VARCHAR(50) PRIMARY KEY,
    track_number VARCHAR(50) NOT NULL,
    entry VARCHAR(10),
    locale VARCHAR(5),
    customer_id VARCHAR(50) NOT NULL,
    delivery_service VARCHAR(20),
    shardkey VARCHAR(10),
    sm_id INTEGER NOT NULL,
    date_created TIMESTAMP WITH TIME ZONE,  -- дата создания заказа из Kafka
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),  -- время сохранения в БД
    oof_shard VARCHAR(5),
    internal_signature VARCHAR(100)
);

CREATE TABLE IF NOT EXISTS deliveries (
    order_uid VARCHAR(50) PRIMARY KEY REFERENCES orders(order_uid) ON DELETE CASCADE,
    name VARCHAR(100),
    phone VARCHAR(20),
    zip VARCHAR(20),
    city VARCHAR(50),
    address VARCHAR(200),
    region VARCHAR(50),
    email VARCHAR(100)
);

CREATE TABLE IF NOT EXISTS payments (
    order_uid VARCHAR(50) PRIMARY KEY REFERENCES orders(order_uid) ON DELETE CASCADE,
    transaction VARCHAR(100) NOT NULL,
    request_id VARCHAR(50),
    currency VARCHAR(5),
    provider VARCHAR(20),
    amount INTEGER,
    payment_dt BIGINT,
    bank VARCHAR(20),
    delivery_cost INTEGER,
    goods_total INTEGER,
    custom_fee INTEGER
);

CREATE TABLE IF NOT EXISTS items (
    id SERIAL PRIMARY KEY,
    order_uid VARCHAR(50) NOT NULL REFERENCES orders(order_uid) ON DELETE CASCADE,
    chrt_id BIGINT,
    track_number VARCHAR(50),
    price INTEGER,
    rid VARCHAR(50),
    name VARCHAR(100),
    sale INTEGER,
    size VARCHAR(10),
    total_price INTEGER,
    nm_id BIGINT,
    brand VARCHAR(50),
    status INTEGER
);
