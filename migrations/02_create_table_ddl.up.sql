CREATE TABLE IF NOT EXISTS roles (
    id UUID NOT NULL PRIMARY KEY,
    role_name VARCHAR NOT NULL,
    role_description VARCHAR NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP 
);

CREATE TABLE IF NOT EXISTS admins (
    id UUID NOT NULL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role_id UUID NOT NULL REFERENCES roles("id"),
    created_at TIMESTAMP DEFAULT NOW()
);
ALTER TABLE admins ADD COLUMN last_login TIMESTAMP ;
ALTER TABLE admins ADD COLUMN  is_superuser BOOLEAN DEFAULT FALSE;
INSERT INTO "roles"(id, role_name, role_description) VALUES('cdd37b47-c947-4faf-becc-0ed0c256d642', 'admin', 'Admin full permission');

INSERT INTO admins(id, username, password_hash, role_id) VALUES (
    '00e1bd9b-d2f6-490a-b706-98b5657c064f',
    'sushitana', 
    '$2a$04$R4MhpSjF.ByeIkAgKHNCk.8xj77fhqf.qHQMl6KYuovQS.A8alG/y',
    'cdd37b47-c947-4faf-becc-0ed0c256d642'
);



CREATE TABLE IF NOT EXISTS access_scopes (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP 
);

CREATE TABLE IF NOT EXISTS role_access_scopes (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    access_scope_id INTEGER NOT NULL REFERENCES access_scopes(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, access_scope_id)
);

INSERT INTO access_scopes (id, name, description)
VALUES
    (1, 'category-read', 'Allows the user to view categories and retrieve category details'),
    (2, 'category-write', 'Allows the user to create, update, or delete categories');

INSERT INTO access_scopes (id, name, description)
VALUES
    (3, 'product-read', 'Allows the user to view products and retrieve detailed product information'),
    (4, 'product-write', 'Allows the user to create, update, or delete products');

INSERT INTO access_scopes (id, name, description)
VALUES
    (5, 'file-read', 'Allows the user to view and retrieve uploaded files'),
    (6, 'file-write', 'Allows the user to upload, update, or delete files');

INSERT INTO role_access_scopes (role_id, access_scope_id)
VALUES
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 1),
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 2),
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 3),
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 4),
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 5),
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 6)
ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS employees(
    id SERIAL NOT NULL PRIMARY KEY,
    name VARCHAR(50) NOT NULL DEFAULT '',
    surname VARCHAR(50) NOT NULL DEFAULT '',
    username VARCHAR(50) UNIQUE DEFAULT '',
    password VARCHAR NOT NULL,
    is_active BOOLEAN DEFAULT true,
    phone_number VARCHAR(13) DEFAULT '',
    role_id UUID NOT NULL REFERENCES roles("id"),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP 
);


INSERT INTO employees (
    name,
    surname,
    username,
    password,
    phone_number,
    role_id,
    is_active
) VALUES (
    'Bahodir',
    'Isomiddinov',
    'backend',
    '$2a$04$R4MhpSjF.ByeIkAgKHNCk.8xj77fhqf.qHQMl6KYuovQS.A8alG/y',
    '+998943341050',
    'cdd37b47-c947-4faf-becc-0ed0c256d642', -- role_id (UUID)
    true
);
CREATE TABLE IF NOT EXISTS images(
    id SERIAL PRIMARY KEY,
    image TEXT NOT NULL DEFAULT '',
    image_type VARCHAR(50) NOT NULL DEFAULT ''
);
