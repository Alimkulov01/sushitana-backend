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
    id BIGSERIAL PRIMARY KEY,
    name JSONB NOT NULL DEFAULT '{}'::jsonb,
    post_id VARCHAR DEFAULT '',
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP DEFAULT now()
);

ALTER TABLE category ADD COLUMN IF NOT EXISTS index INT DEFAULT 0;

CREATE TABLE product(
    id BIGSERIAL PRIMARY KEY,
    name JSONB NOT NULL DEFAULT '{}'::jsonb,
    category_id INTEGER NOT NULL REFERENCES category("id"),
    img_url VARCHAR DEFAULT '',
    price INTEGER DEFAULT 0,
    count INTEGER DEFAULT 0,
    decription JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP DEFAULT now()
);
