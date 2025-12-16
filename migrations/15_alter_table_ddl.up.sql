ALTER TABLE orders
  ADD COLUMN IF NOT EXISTS iiko_pos_id varchar DEFAULT '',
  ADD COLUMN IF NOT EXISTS iiko_correlation_id varchar DEFAULT '';