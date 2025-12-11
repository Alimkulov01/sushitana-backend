CREATE TABLE invoices (
    id UUID PRIMARY KEY,
    click_invoice_id BIGINT NOT NULL,
    click_trans_id BIGINT,
    merchant_trans_id VARCHAR NOT NULL,
    order_id UUID,
    tg_id BIGINT,
    customer_phone VARCHAR,
    amount NUMERIC(12,2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'UZS',
    status VARCHAR NOT NULL,
    comment TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);

ALTER TABLE orders
    ADD COLUMN click_request_id TEXT,
    ADD COLUMN click_transaction_param TEXT,
    ADD COLUMN click_prepare_at TIMESTAMP;