CREATE TABLE IF NOT EXISTS payme_transactions (
    id UUID PRIMARY KEY,
    paycom_transaction_id TEXT NOT NULL UNIQUE,   -- Payme "id" (transaction)
    order_id UUID NOT NULL REFERENCES orders(id),
    amount NUMERIC(12, 2) NOT NULL,
    state INT NOT NULL,                           -- 1 created, 2 performed, -1/-2 canceled
    created_time BIGINT NOT NULL,                 -- ms epoch (Payme create_time)
    perform_time BIGINT,
    cancel_time BIGINT,
    reason INT,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS payme_transactions_order_id_idx
  ON payme_transactions(order_id);

CREATE INDEX IF NOT EXISTS payme_transactions_created_time_idx
  ON payme_transactions(created_time);

CREATE UNIQUE INDEX IF NOT EXISTS payme_one_active_per_order_uq
  ON payme_transactions(order_id)
  WHERE state = 1;