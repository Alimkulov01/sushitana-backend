INSERT INTO access_scopes (id, name, description)
VALUES
    (15, 'courier-read', 'Allows the user to view couriers and retrieve role details'),
    (16, 'courier-write', 'Allows the user to create, update, or delete roles');

INSERT INTO role_access_scopes (role_id, access_scope_id)
VALUES
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 15),
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 16)
ON CONFLICT DO NOTHING;