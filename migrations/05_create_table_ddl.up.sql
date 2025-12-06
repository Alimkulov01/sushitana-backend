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



ALTER TABLE clients
ALTER COLUMN language DROP DEFAULT;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS name VARCHAR DEFAULT '';