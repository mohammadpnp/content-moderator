# Triton Inference Server – Integration Summary

## موقعیت در پروژه
پوشهٔ `api/triton/` شامل کدهای خودکار تولیدشده از فایل‌های `.proto` سرور NVIDIA Triton است.  
به دلیل حجم بسیار بالای این فایل‌ها (تنها `model_config.pb.go` بیش از ۱۲٬۰۰۰ خط)، این پوشه در فایل `.repomixignore` قرار گرفته و از خروجی Repomix حذف شده است.

## محتوای پوشه (خلاصه)
- **پروتوهای اصلی Triton:**  
  `grpc_service.proto`, `health.proto`, `model_config.proto`  
- **فایل‌های تولیدشدهٔ Go (gRPC و Protobuf):**  
  - `grpc_service.pb.go`, `grpc_service_grpc.pb.go`  
  - `health.pb.go`, `health_grpc.pb.go`  
  - `model_config.pb.go`

## مدل‌های استفاده‌شده در پروژه
| مدل                | هدف                 |
| ------------------ | ------------------- |
| `text_moderation`  | تشخیص محتوای متنی   |
| `image_moderation` | تشخیص محتوای تصویری |

## ساختار درخواست استنتاج (ModelInfer)
- **نام تنسور ورودی متن:** `text_input`  
- **نام تنسور ورودی تصویر:** `image_input`  
- **نوع داده:** `BYTES`  
- **Shape:** `[1]`  
- **تنسورهای خروجی درخواستی:**  
  - `probabilities` (نوع `FP32`، مقدار احتمال)  
  - `categories` (نوع `BYTES`، دسته‌بندی‌ها با جداکنندهٔ کاما)

## مدارشکن (Circuit Breaker)
کلاینت Triton از طریق یک Wrapper به نام `CircuitBreakerAIClient` محافظت می‌شود.  
تنظیمات:
- باز شدن مدار پس از ۵ خطا در ۶۰ ثانیه (با نسبت شکست ≥ ۶۰٪)
- ۳ درخواست آزمایشی در حالت نیمه‌باز
- زمان باز ماندن مدار: ۳۰ ثانیه
- بازنشانی شمارنده‌ها: هر ۶۰ ثانیه

## پیاده‌سازی‌های دستی
کدهایی که مستقیماً به Triton مربوط می‌شوند و توسط تیم نوشته شده‌اند:
- `internal/adapter/outbound/triton/client.go`  
- `internal/adapter/outbound/triton/circuit_breaker.go`

## یادداشت
برای تحلیل‌های بعدی، اطلاعات فنی کامل در همین فایل موجود است و نیازی به مشاهدهٔ کدهای generated نیست.