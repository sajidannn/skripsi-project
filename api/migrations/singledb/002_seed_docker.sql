-- Docker init script for single-DB mode.
-- Runs automatically when the postgres container starts for the first time.

-- Seed a test tenant so you can immediately generate a JWT and test the API.
INSERT INTO tenants (name) VALUES ('tenant-alpha') ON CONFLICT DO NOTHING;
INSERT INTO tenants (name) VALUES ('tenant-beta')  ON CONFLICT DO NOTHING;
