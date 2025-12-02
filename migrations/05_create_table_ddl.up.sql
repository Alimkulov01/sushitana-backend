INSERT INTO access_scopes (id, name, description)
VALUES
    (11, 'client-read', 'Allows the user to view clients and retrieve role details'),
    (12, 'client-write', 'Allows the user to create, update, or delete roles');

INSERT INTO role_access_scopes (role_id, access_scope_id)
VALUES
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 11),
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 12)
ON CONFLICT DO NOTHING;

ALTER TABLE category ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT FALSE;

CREATE TYPE delivery_type AS ENUM ('pickup', 'courier');
CREATE TYPE payment_method AS ENUM ('cash', 'payme', 'click');
CREATE TYPE order_status AS ENUM ('new', 'pending_payment', 'failed_payment','in_progress', 'packaging', 'out_of_delivery', 'delivered', 'completed', 'cancelled');

CREATE TABLE IF NOT EXISTS orders(
    id SERIAL PRIMARY KEY,
    tg_id INTEGER NOT NULL,
    phone_number VARCHAR NOT NULL DEFAULT '',
    address JSONB NOT NULL DEFAULT '{}',
    total_price INTEGER NOT NULL DEFAULT 0,
    delivery_type delivery_type NOT NULL,
    payment_method payment_method NOT NULL ,
    delivery_price INTEGER NOT NULL DEFAULT 0,
    products JSONB NOT NULL DEFAULT '[]',
    orders_status VARCHAR NOT NULL DEFAULT 'new',
    link TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE clients ADD COLUMN IF NOT EXISTS name VARCHAR DEFAULT '';