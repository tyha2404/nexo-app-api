-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION public.handle_new_supabase_user()
RETURNS TRIGGER AS $$
DECLARE
    new_username VARCHAR(50);
BEGIN
    -- Lấy username từ metadata hoặc email
    new_username := COALESCE(
        NEW.raw_user_meta_data->>'username',
        SPLIT_PART(NEW.email, '@', 1)
    );

    -- Đảm bảo không trùng username nếu đã tồn tại
    IF EXISTS (SELECT 1 FROM public.users WHERE username = new_username) THEN
        new_username := SUBSTRING(new_username FROM 1 FOR 40) || '_' || SUBSTRING(NEW.id::text FROM 1 FOR 8);
    END IF;

    -- 1. Thêm vào public.users
    INSERT INTO public.users (id, username, email, password, role, created_at, updated_at)
    VALUES (
        NEW.id,
        new_username,
        NEW.email,
        '', -- Password do Supabase Auth quản lý
        'user',
        NOW(),
        NOW()
    )
    ON CONFLICT (id) DO NOTHING;

    -- 2. Seed Default Categories cho User
    INSERT INTO public.categories (id, user_id, name, type, description, created_at, updated_at)
    VALUES
        (gen_random_uuid(), NEW.id, 'Ăn uống', 'EXPENSE', 'Chi tiêu cho thực phẩm, ăn uống hàng ngày', NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Di chuyển', 'EXPENSE', 'Chi phí đi lại, xăng xe, dịch vụ vận chuyển', NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Nhà cửa', 'EXPENSE', 'Tiền thuê nhà, hóa đơn điện nước và dịch vụ', NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Giải trí', 'EXPENSE', 'Xem phim, mua sắm giải trí, du lịch', NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Y tế & Sức khỏe', 'EXPENSE', 'Khám bệnh, thuốc men, tập thể dục', NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Giáo dục', 'EXPENSE', 'Học phí, sách vở, khóa học phát triển', NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Tiền nhà', 'EXPENSE', 'Chi phí thuê nhà, tiền nhà hàng tháng', NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Tiền cầu lông', 'EXPENSE', 'Chi phí chơi cầu lông, sân bãi, dụng cụ', NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Tiền hẹn hò', 'EXPENSE', 'Chi phí hẹn hò, ăn uống giải trí cùng đối phương', NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Lương', 'INCOME', 'Thu nhập từ lương công việc chính', NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Thưởng', 'INCOME', 'Tiền thưởng hiệu suất, thưởng tháng 13', NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Đầu tư', 'INCOME', 'Lợi nhuận từ cổ phiếu, tiền gửi tiết kiệm', NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Khác', 'INCOME', 'Các nguồn thu nhập vãng lai khác', NOW(), NOW())
    ON CONFLICT DO NOTHING;

    -- 3. Seed Default Wallets cho User
    INSERT INTO public.wallets (id, user_id, name, type, icon, balance, is_included_in_total, jar_category, allocation_percent, created_at, updated_at)
    VALUES
        (gen_random_uuid(), NEW.id, 'Ví Tiền mặt', 'CASH', '💵', 0, true, NULL, 0, NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Tài khoản Ngân hàng', 'BANK', '🏦', 0, true, NULL, 0, NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Ví Điện tử', 'E_WALLET', '📱', 0, true, NULL, 0, NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Hũ Thiết yếu (50%)', 'JAR', '🏠', 0, true, 'NEC', 50.0, NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Hũ Đầu tư & Tiết kiệm (30%)', 'JAR', '📈', 0, true, 'INVEST', 30.0, NOW(), NOW()),
        (gen_random_uuid(), NEW.id, 'Hũ Giải trí & Cá nhân (20%)', 'JAR', '🎯', 0, true, 'PLAY', 20.0, NOW(), NOW())
    ON CONFLICT DO NOTHING;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS on_auth_user_created ON auth.users;
CREATE TRIGGER on_auth_user_created
    AFTER INSERT ON auth.users
    FOR EACH ROW EXECUTE FUNCTION public.handle_new_supabase_user();

-- +goose Down
DROP TRIGGER IF EXISTS on_auth_user_created ON auth.users;
DROP FUNCTION IF EXISTS public.handle_new_supabase_user();
