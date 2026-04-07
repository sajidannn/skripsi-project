-- ─── Multi-DB master seed data ───────────────────────────────────────────────
-- Seed tenant routing entries in the master (pos_master) database.
-- Each tenant gets its own database: pos_tenant1, pos_tenant2, pos_tenant3.
-- NOTE: db_user/db_password below match the Docker dev postgres superuser.
--       In production, create a dedicated user per tenant with limited privileges.

INSERT INTO tenants (name, db_name, db_user, db_password) VALUES
    ('Warung Maju',    'pos_tenant1', 'postgres', 'supersecret'),
    ('Toko Berkah',    'pos_tenant2', 'postgres', 'supersecret'),
    ('Kedai Nusantara','pos_tenant3', 'postgres', 'supersecret');
