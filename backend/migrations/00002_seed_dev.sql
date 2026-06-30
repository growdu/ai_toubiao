-- +goose Up
-- +goose StatementBegin

-- Dev-only seed data. Never run in production.
-- Passwords are bcrypt hashes of password123 (cost=10).
-- Replace with real hashes before any non-dev use.

INSERT INTO tenants (id, name, slug, plan) VALUES
    ('11111111-1111-1111-1111-111111111111', '演示租户 A', 'demo-a', 'pro'),
    ('22222222-2222-2222-2222-222222222222', '演示租户 B', 'demo-b', 'free')
ON CONFLICT DO NOTHING;

INSERT INTO users (id, tenant_id, email, password_hash, display_name, role) VALUES
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     '11111111-1111-1111-1111-111111111111',
     'admin@demo-a.test',
     '$2b$10$c/oVpBmo/v4FzAXuZidA/.5J7/LLVBFZfxSyNmwTTVoUYDXIEhde2',
     '管理员 A',
     'owner'),
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
     '11111111-1111-1111-1111-111111111111',
     'member@demo-a.test',
     '$2b$10$CEtuZc0frRC2syRa79aP6ODJhKKwA9JjBYYC5Jtk/I.hHDepyBxM.',
     '成员 A',
     'member'),
    ('cccccccc-cccc-cccc-cccc-cccccccccccc',
     '22222222-2222-2222-2222-222222222222',
     'admin@demo-b.test',
     '$2b$10$q5DPkrXoLvebe9y.nn.9U.fujLaEi.3ulc9S/IdOGeBRKftuWGN1S',
     '管理员 B',
     'owner')
ON CONFLICT DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
TRUNCATE users, tenants CASCADE;
-- +goose StatementEnd
