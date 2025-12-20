ALTER TABLE orders
ADD COLUMN IF NOT EXISTS last_notified_status order_status;

CREATE INDEX IF NOT EXISTS idx_orders_order_number ON orders(order_number);
CREATE INDEX IF NOT EXISTS idx_orders_iiko_order_id ON orders(iiko_order_id);
CREATE INDEX IF NOT EXISTS idx_orders_iiko_delivery_id ON orders(iiko_delivery_id);
CREATE INDEX IF NOT EXISTS idx_orders_tg_id ON orders(tg_id);
