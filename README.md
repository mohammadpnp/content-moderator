# Real-time AI Content Moderator

سیستم بلادرنگ تشخیص محتوای نامناسب با معماری Hexagonal

## ساختار پروژه
- **cmd/**: نقطه ورود برنامه
- **internal/domain/**: منطق کسب و کار خالص
- **internal/service/**: سرویس‌های برنامه
- **internal/adapter/**: پیاده‌سازی‌های زیرساختی

## تکنولوژی‌ها
- Go 1.22+
- PostgreSQL + sqlx
- Redis
- NATS
- Triton Inference Server
