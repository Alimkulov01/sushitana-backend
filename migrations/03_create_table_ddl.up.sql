INSERT INTO access_scopes (id, name, description)
VALUES
    (7, 'role-read', 'Allows the user to view roles and retrieve role details'),
    (8, 'role-write', 'Allows the user to create, update, or delete roles');

INSERT INTO access_scopes (id, name, description)
VALUES
    (9, 'employee-read', 'Allows the user to view employees and retrieve employee details'),
    (10, 'employee-write', 'Allows the user to create, update, or delete employees');

INSERT INTO role_access_scopes (role_id, access_scope_id)
VALUES
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 7),
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 8),
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 9),
  ('cdd37b47-c947-4faf-becc-0ed0c256d642', 10)
ON CONFLICT DO NOTHING;