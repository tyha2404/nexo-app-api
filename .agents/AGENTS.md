# Nexo App API - Quy tắc & Hướng dẫn dành cho AI Agent

Tài liệu này định nghĩa các quy tắc phát triển, kiến trúc dự án và quy trình làm việc dành cho AI Agent khi tương tác và lập trình trong repository `nexo-app-api`.

## 1. Tổng quan dự án & Công nghệ

`nexo-app-api` là dịch vụ backend API phục vụ cho ứng dụng Nexo. Dự án được phát triển bằng ngôn ngữ **Go** và sử dụng các thư viện/công nghệ chính sau:

- **Ngôn ngữ chính**: Go (Golang) v1.25.1+
- **Web Framework / Router**: [go-chi/chi/v5](https://github.com/go-chi/chi) - Router gọn nhẹ và tối ưu cho Go.
- **ORM / Database**: [GORM](https://gorm.io/) với PostgreSQL Driver (`gorm.io/driver/postgres`).
- **Xác thực**: JWT (`golang-jwt/jwt/v5`).
- **Ghi log**: Uber Zap Logger (`go-uber.org/zap`).
- **Tài liệu API (Swagger)**: [swag](https://github.com/swaggo/swag) & `http-swagger`.
- **Validation**: [go-playground/validator/v10](https://github.com/go-playground/validator).
- **Hot Reload (Dev)**: [Air](https://github.com/cosmtrek/air) (`.air.toml`).

## 2. Kiến trúc & Thiết kế Dự án (Architecture & Design)

Mã nguồn được tổ chức theo mô hình phân lớp (Layered Architecture) trong thư mục `internal`:

```
nexo-app-api
├── cmd/
│   └── server/          # Điểm chạy ứng dụng chính (main.go)
├── internal/            # Chứa toàn bộ logic nghiệp vụ (Private application code)
│   ├── config/          # Quản lý cấu hình hệ thống
│   ├── constant/        # Các hằng số dùng chung trong dự án
│   ├── db/              # Khởi tạo kết nối cơ sở dữ liệu
│   ├── dto/             # Data Transfer Objects (Structs nhận/gửi dữ liệu và validate)
│   ├── handler/         # Lớp Handler (Nhận HTTP request, validate và gọi service)
│   ├── logger/          # Bộ ghi log (Zap wrapper)
│   ├── middleware/      # Các HTTP middleware (xác thực JWT, log request, recovery,...)
│   ├── migration/       # Định nghĩa các database migration/seeder
│   ├── model/           # Định nghĩa GORM Models đại diện cho các bảng DB
│   ├── repository/      # Lớp Repository (Thao tác trực tiếp dữ liệu với DB)
│   ├── response/        # Chuẩn hóa định dạng JSON response trả về client
│   ├── router/          # Định nghĩa và đăng ký các routes/endpoints
│   ├── service/         # Lớp Service (Chứa logic nghiệp vụ chính của ứng dụng)
│   └── util/            # Các hàm tiện ích bổ trợ
```

---

## 3. Quy tắc bắt buộc dành cho AI Agent (Agent Rules)

Để đảm bảo an toàn hệ thống và kiểm soát tốt mã nguồn, Agent **phải tuân thủ tuyệt đối** các quy tắc dưới đây:

### ⚠️ KHÔNG TỰ Ý COMMIT CODE
- **Không bao giờ** tự động thực hiện lệnh `git commit`, `git push`, hoặc tự động tạo Pull Request mà chưa được sự đồng ý và xác nhận rõ ràng từ phía User.
- Khi hoàn thành tác vụ, Agent chỉ nên chỉnh sửa file trên local và báo cáo trạng thái để User tự kiểm tra và commit.

### ⚠️ KHÔNG TỰ Ý CHẠY MIGRATION
- **Không tự động** chạy các lệnh migrate database (ví dụ: chạy các file migration thay đổi cấu trúc DB trực tiếp, tự động gọi hàm AutoMigrate khi khởi chạy môi trường dev mà không hỏi trước).
- Mọi thay đổi về cấu trúc bảng (schema) trong thư mục `internal/model` hoặc `internal/migration` cần được trình bày rõ ràng dưới dạng kế hoạch cho User phê duyệt trước khi thực thi.

### ⚙️ Quy tắc phát triển Code
1. **Tránh Lạm dụng Generics trong Repository & Service**:
   - Không sử dụng các base generic repository/service chung (`BaseRepo`, `BaseService`).
   - Mọi queries SQL và business logic phải được viết tường minh (explicit) trong từng repository/service cụ thể để đảm bảo tính trực quan, tối ưu hóa hiệu năng truy vấn của GORM và dễ debug.
2. **Xử lý Lỗi Tường minh (Explicit Error Handling)**:
   - Go yêu cầu xử lý lỗi rõ ràng. Luôn kiểm tra `err != nil`.
   - Đối với HTTP API, cấm đóng gói status code quá sâu hoặc tự động map status code ngầm.
   - Các Handler phải truyền trực tiếp HTTP Status Code tường minh khi gọi `errorHandler.HandleError(w, err, httpStatus, message, operation)` để lập trình viên đọc code handler biết ngay API trả về HTTP Status gì.
3. **Đồng bộ Swagger Docs**:
   - Khi thêm hoặc sửa bất kỳ Endpoint nào trong `handler`, hãy cập nhật chú thích (annotations) của Swagger đầy đủ và chạy lệnh generate lại docs (`swag init -g cmd/server/main.go`).
   - Lưu ý: Tránh sử dụng các kiểu Generics lồng nhau trong annotations (ví dụ: `response.BaseResponse[dto.TransactionResponse]`) vì bộ phân tích của `swag` có thể bị lỗi cú pháp; hãy định nghĩa struct cụ thể đại diện cho response để Swagger build chính xác.
4. **Tuân thủ đúng phân lớp**:
   - Handler chỉ xử lý HTTP Request/Response, Validation và gán HTTP Status code. Không viết logic nghiệp vụ tại Handler.
   - Service xử lý logic nghiệp vụ. Không thao tác trực tiếp với Database ở đây, hãy gọi Repository.
   - Repository chịu trách nhiệm truy vấn DB.

