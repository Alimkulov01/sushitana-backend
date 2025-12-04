CREATE TABLE IF NOT EXISTS clients (
    id BIGSERIAL PRIMARY KEY,
    tgid BIGINT UNIQUE,                    
    phone VARCHAR DEFAULT '',
    language TEXT DEFAULT 'uz',
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP DEFAULT now()
);

CREATE TABLE state (
    user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,
    state TEXT NOT NULL DEFAULT '',
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (user_id, chat_id)
);


CREATE UNIQUE INDEX IF NOT EXISTS idx_state_on_user_id_chat_id ON state(user_id, chat_id);

CREATE TABLE category (
    id VARCHAR PRIMARY KEY,
    name JSONB NOT NULL DEFAULT '{}'::jsonb,
    post_id VARCHAR DEFAULT '',
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP DEFAULT now()
);

ALTER TABLE category ADD COLUMN IF NOT EXISTS index INT DEFAULT 0;

CREATE TABLE product(
    id VARCHAR PRIMARY KEY,
    name JSONB NOT NULL DEFAULT '{}'::jsonb,
    group_id VARCHAR NOT NULL DEFAULT '',
    product_category_id VARCHAR DEFAULT '',
    type VARCHAR NOT NULL DEFAULT '',
    order_item_type VARCHAR NOT NULL DEFAULT '',
    measure_unit VARCHAR NOT NULL DEFAULT '',
    size_prices JSONB NOT NULL DEFAULT '[]'::jsonb,
    do_not_print_in_cheque BOOLEAN DEFAULT FALSE,
    parent_group VARCHAR DEFAULT '',
    "order" INT DEFAULT 0,
    payment_subject VARCHAR DEFAULT '',
    code VARCHAR DEFAULT '',
    is_deleted BOOLEAN DEFAULT FALSE,
    can_set_open_price BOOLEAN DEFAULT FALSE,
    splittable BOOLEAN DEFAULT FALSE,
    weight FLOAT DEFAULT 0,
    "index" INT DEFAULT 0,
    is_new BOOLEAN DEFAULT FALSE,
    img_url TEXT DEFAULT '',
    is_active BOOLEAN DEFAULT FALSE,
    is_have_box BOOLEAN DEFAULT FALSE,
    box_count INT DEFAULT 0,
    box_price INT DEFAULT 0,
    description JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP DEFAULT now()
);
