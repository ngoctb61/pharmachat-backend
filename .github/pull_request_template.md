<!--
Tiêu đề PR theo conventional commits: <type>(<scope>): <tóm tắt>
Ví dụ: feat(auth): dang nhap bang email va mat khau (GO-07)
Xóa các mục không áp dụng thay vì để trống.
-->

## Task và SPEC

- Task: GO-xx <!-- docs/MIGRATION_GO_BACKEND.md mục 7 -->
- SPEC: SPEC-0x <!-- .sdd/specs/ -->
- Quyết định liên quan: <!-- B-1..B-6 nếu có, hoặc xóa dòng này -->

## Thay đổi gì và tại sao

<!-- 2-5 gạch đầu dòng. Nêu lý do, không liệt kê lại từng file. -->

-

## Kiểm chứng

<!-- Bằng chứng thật: test nào chứng minh điều kiện xong, lệnh đã chạy, kết quả thấy được. -->

- [ ] `make lint` sạch
- [ ] `make test` pass (gồm test tích hợp)
- [ ] Đã dựng stack thật bằng `/verify-stack` <!-- chỉ khi đổi hạ tầng, Dockerfile, compose, Caddy hoặc endpoint -->

Chi tiết:

-

## Checklist

**Dữ liệu và schema** (xóa nếu không đổi schema)

- [ ] Migration mới, không sửa migration đã merge; có cả `Up` và `Down`
- [ ] Bảng mới có `tenant_id` đã bật RLS + policy `tenant_isolation`, đã `GRANT` cho `pharmachat_app`
- [ ] Đã chạy `make gen`, không sửa tay `internal/store/*.go`

**Bảo mật** (constitution Art. 3)

- [ ] Truy vấn dữ liệu tenant chạy trong `db.WithTenant`
- [ ] Không log số điện thoại, email, nội dung tin nhắn, token hay API key
- [ ] Không có bí mật trong code hoặc commit; bí mật lưu DB được mã hóa bằng `crypto.Sealer`
- [ ] Lỗi trả client dùng `httpx.Error`, không lộ chi tiết nội bộ

**Hợp đồng và tài liệu**

- [ ] Đổi API public: đã cập nhật `api/openapi.yaml` và báo cho repo frontend
- [ ] Đã cập nhật trạng thái task trong `.sdd/TASKS.md`
- [ ] PR chạm tới bảo mật (auth, RLS, mã hóa, webhook): cần 2 người review (constitution Art. 7)

## Ngoài phạm vi

<!-- Lỗi hoặc việc phát hiện được nhưng cố ý không làm trong PR này, kèm lý do. Xóa nếu không có. -->
