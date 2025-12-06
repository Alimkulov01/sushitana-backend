CREATE TYPE delivery_type AS ENUM ('DELIVERY', 'PICKUP');
CREATE TYPE payment_status AS ENUM ('UNPAID', 'PENDING', 'PAID');
CREATE TYPE payment_method AS ENUM ('CASH', 'PAYME', 'CLICK');
CREATE TYPE order_status AS ENUM (
    'WAITING_OPERATOR',
    'WAITING_PAYMENT',
    'REJECTED',
    'SENT_TO_IIKO',
    'COOKING',
    'READY_FOR_PICKUP',
    'ON_THE_WAY',
    'DELIVERED',
    'CANCELLED'
);

CREATE TABLE IF NOT EXISTS orders (
    id UUID NOT NULL PRIMARY KEY,
    tg_id INT DEFAULT 0,
    delivery_type delivery_type NOT NULL,
    payment_method payment_method NOT NULL,
    payment_status payment_status NOT NULL,
    order_status order_status NOT NULL,
    address JSONB NOT NULL DEFAULT '{}'::jsonb,
    comment TEXT DEFAULT '',
    iiko_order_id VARCHAR DEFAULT '',
    iiko_delivery_id VARCHAR DEFAULT '',
    items JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO access_scopes (id, name, description)
VALUES
    (13, 'order-read', 'Allows the user to view orders and retrieve role details'),
    (14, 'order-write', 'Allows the user to create, update, or delete roles');

INSERT INTO role_access_scopes (role_id, access_scope_id)
VALUES
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 13),
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 14)
ON CONFLICT DO NOTHING;