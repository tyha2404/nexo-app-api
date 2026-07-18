-- +goose Up
-- Chèn "Tiền nhà" cho những user chưa có
INSERT INTO categories (id, user_id, name, type, description, created_at, updated_at)
SELECT 
    uuid_generate_v4(), 
    u.id, 
    'Tiền nhà', 
    'EXPENSE', 
    'Chi phí thuê nhà, tiền nhà hàng tháng', 
    NOW(), 
    NOW()
FROM users u
WHERE NOT EXISTS (
    SELECT 1 FROM categories c WHERE c.user_id = u.id AND c.name = 'Tiền nhà'
);

-- Chèn "Tiền cầu lông" cho những user chưa có
INSERT INTO categories (id, user_id, name, type, description, created_at, updated_at)
SELECT 
    uuid_generate_v4(), 
    u.id, 
    'Tiền cầu lông', 
    'EXPENSE', 
    'Chi phí chơi cầu lông, sân bãi, dụng cụ', 
    NOW(), 
    NOW()
FROM users u
WHERE NOT EXISTS (
    SELECT 1 FROM categories c WHERE c.user_id = u.id AND c.name = 'Tiền cầu lông'
);

-- Chèn "Tiền hẹn hò" cho những user chưa có
INSERT INTO categories (id, user_id, name, type, description, created_at, updated_at)
SELECT 
    uuid_generate_v4(), 
    u.id, 
    'Tiền hẹn hò', 
    'EXPENSE', 
    'Chi phí hẹn hò, ăn uống giải trí cùng đối phương', 
    NOW(), 
    NOW()
FROM users u
WHERE NOT EXISTS (
    SELECT 1 FROM categories c WHERE c.user_id = u.id AND c.name = 'Tiền hẹn hò'
);

-- +goose Down
DELETE FROM categories WHERE name IN ('Tiền nhà', 'Tiền cầu lông', 'Tiền hẹn hò') AND description IN (
    'Chi phí thuê nhà, tiền nhà hàng tháng',
    'Chi phí chơi cầu lông, sân bãi, dụng cụ',
    'Chi phí hẹn hò, ăn uống giải trí cùng đối phương'
);

