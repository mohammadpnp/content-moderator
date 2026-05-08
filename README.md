<p align="center">
  <img src="https://img.icons8.com/fluency/96/artificial-intelligence.png" alt="AI Moderator" width="120"/>
</p>

<h1 align="center">🛡️ Real‑Time AI Content Moderator</h1>

<p align="center">
  <em>سیستم بلادرنگ پالایش محتوای نامناسب با معماری Hexagonal و قدرت Go</em>
  <br>
  <strong>۱۰۰۰+ درخواست در ثانیه • زیر ۲ ثانیه پاسخ • پشتیبانی از ۱۰۰,۰۰۰ کاربر هم‌زمان</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white" alt="Go version">
  <img src="https://img.shields.io/badge/Phase-5%20Completed-brightgreen" alt="Phase">
  <img src="https://img.shields.io/badge/Tests-Passing-success?logo=githubactions" alt="Tests">
  <img src="https://img.shields.io/badge/Coverage-80%25%2B-brightgreen" alt="Coverage">
  <img src="https://img.shields.io/badge/License-MIT-blue" alt="License">
</p>

---

## 📖 مقدمه

در یک پلتفرم اجتماعی مدرن، جلوگیری از انتشار محتوای نامناسب (متنی و تصویری) یک نیاز حیاتی است.  
این پروژه یک سیستم **مقیاس‌پذیر** و **بلادرنگ** برای تشخیص خودکار محتوای مخرب با استفاده از هوش مصنوعی ارائه می‌دهد.

کاربر محتوا را ارسال می‌کند، سیستم آن را تحلیل می‌کند و نتیجه را بلافاصله به صورت **اعلان لحظه‌ای** (WebSocket) به کاربر بازمی‌گرداند.

---

## 🧱 معماری (Hexagonal Architecture)

پروژه با الگوی **Ports & Adapters** طراحی شده تا هسته‌ی کسب‌و‌کار کاملاً از زیرساخت‌ها جدا باشد.

cmd/server/ ← نقطه ورود
internal/
domain/entity/ ← Entity های اصلی (Content, Moderation, Notification)
domain/port/inbound/ ← پورت‌های ورودی (سرویس‌ها)
domain/port/outbound/ ← پورت‌های خروجی (Repository, AI, Cache, ...)
service/ ← منطق کسب‌و‌کار
adapter/inbound/ ← آداپتورهای ورودی (HTTP/Fiber, gRPC, WebSocket)
adapter/outbound/ ← آداپتورهای خروجی (PostgreSQL, Redis, NATS, Triton)
worker/ ← Worker Pool برای پردازش غیرهمزمان
test/mock/ ← Mock های تست

text

---

## 🚀 وضعیت فازها

| فاز | عنوان                                              | وضعیت            |
| --- | -------------------------------------------------- | ---------------- |
| ۰   | Foundation & Setup                                 | ✅ تکمیل          |
| ۱   | HTTP API & PostgreSQL                              | ✅ تکمیل          |
| ۲   | gRPC & AI Model Integration                        | ✅ تکمیل          |
| ۳   | Async Processing & Worker Pool (NATS)              | ✅ تکمیل          |
| ۴   | WebSocket & Real-Time Notification (Redis Pub/Sub) | ✅ تکمیل          |
| ۵   | Caching & Performance Optimization                 | ✅ تکمیل          |
| ۶   | Monitoring & Observability                         | 🚧 در حال اجرا    |
| ۷   | Integration Testing & Load Testing                 | 🚧 برنامه‌ریزی شده |

---

## ⚙️ تکنولوژی‌ها و کتابخانه‌ها

| حوزه                 | تکنولوژی                                        |
| -------------------- | ----------------------------------------------- |
| **زبان**             | Go 1.25+                                        |
| **HTTP Router**      | [Fiber](https://gofiber.io/)                    |
| **gRPC**             | google.golang.org/grpc                          |
| **Database**         | PostgreSQL + sqlx                               |
| **Migration**        | golang-migrate                                  |
| **Cache**            | Redis (go-redis)                                |
| **Message Broker**   | NATS (JetStream)                                |
| **AI Inference**     | NVIDIA Triton Inference Server                  |
| **WebSocket**        | Fiber WebSocket + Gorilla WebSocket             |
| **Observability**    | Prometheus, Grafana, Jaeger, Zerolog (در فاز ۶) |
| **Containerization** | Docker, Docker Compose                          |
| **CI**               | GitHub Actions                                  |

---

## 🛠️ راه‌اندازی سریع

### ۱. پیش‌نیازها

- Go 1.25+
- Docker & Docker Compose
- Make (اختیاری)

### ۲. اجرا با Docker

```bash
cp .env.example .env
# تنظیمات دلخواه را در .env انجام دهید
make docker-up
این دستور همه سرویس‌ها را بالا می‌آورد:

App : http://localhost:8080

gRPC : localhost:9090

PostgreSQL : localhost:5432

Redis : localhost:6379

NATS : localhost:4222

pprof : http://localhost:6060

۳. توسعه محلی
bash
# نصب ابزارها
make install-tools

# اجرای وابستگی‌ها (PostgreSQL, Redis, NATS)
make dev

# اجرای تست‌ها
make test
📡 API Endpoints (HTTP)
Method	Path	توضیح
POST	/api/v1/contents	ایجاد محتوای جدید
GET	/api/v1/contents/:id	دریافت یک محتوا
GET	/api/v1/users/:userID/contents	لیست محتوای کاربر
DELETE	/api/v1/contents/:id	حذف محتوا
GET	/ws?token=...	WebSocket برای اعلان‌ها
GET	/health	بررسی سلامت
GET	/metrics	متریک‌های Prometheus
همه درخواست‌ها (جز health/metrics) نیاز به Header Authorization: Bearer <token> دارند.

🧪 تست‌ها و Coverage
bash
# اجرای کل تست‌ها
make test

# تست با پوشش کد
make test-coverage

# فقط تست‌های Unit
make test-unit
پوشش تست در CI بررسی می‌شود (هدف ≥ ۸۰٪).
Mockها در test/mock/ قرار دارند و تمام Interfaceها را پیاده‌سازی می‌کنند.

⚡ Profiling و Benchmark
pprof Endpoints (پورت ۶۰۶۰)
Endpoint	کاربرد
/debug/pprof/heap	حافظه Heap
/debug/pprof/goroutine	وضعیت Goroutineها
/debug/pprof/profile?seconds=30	CPU Profile
/debug/pprof/allocs	تخصیص حافظه
/debug/pprof/trace	Execution Trace
گرفتن CPU Profile:

bash
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
# سپس در interactive shell:
(pprof) top10
(pprof) list ModerationService.ModerateContent
گرفتن Heap Profile:

bash
go tool pprof http://localhost:6060/debug/pprof/heap
Escape Analysis
بررسی فرار متغیرها به Heap:

bash
go build -gcflags="-m" ./internal/service/ 2>&1 | grep "escapes to heap"
GC Trace
مانیتور کردن Garbage Collector در زمان واقعی:

bash
GODEBUG=gctrace=1 go run ./cmd/server
Benchmark Suite
bash
# اجرای همه‌ی بنچمارک‌ها
go test -bench=. -benchmem ./internal/adapter/outbound/redis/

# نمونه خروجی:
BenchmarkJSONMarshal_NoPool-8         1000000  1500 ns/op   512 B/op   5 allocs/op
BenchmarkJSONMarshal_WithPool-8       2000000   800 ns/op   128 B/op   1 allocs/op
BenchmarkCacheSet-8                    300000  1200 ns/op   256 B/op   2 allocs/op
BenchmarkCacheGet-8                    500000   900 ns/op   120 B/op   1 allocs/op
بنچمارک‌های موجود:

BenchmarkJSONMarshal_NoPool / WithPool – تأثیر sync.Pool

BenchmarkCacheSet / BenchmarkCacheGet – عملیات Redis Cache

BenchmarkCacheAside_Hit – Cache-Aside Pattern

Cache Stampede Protection Test
bash
go test -v -run TestWithCacheLock ./internal/adapter/outbound/redis/
این تست با ۵۰ گوروتین همزمان تأیید می‌کند که فقط یک بار fetchFn صدا زده می‌شود.

🔧 Makefile Commands
bash
make help               # نمایش همه دستورات
make build              # ساخت باینری
make run                # اجرای برنامه
make test               # اجرای تست‌ها
make test-coverage      # تست با گزارش پوشش
make lint               # اجرای linter
make proto              # تولید کدهای gRPC از فایل‌های proto
make docker-up          # بالا آوردن همه سرویس‌ها
make docker-down        # پایین آوردن همه سرویس‌ها
make dev                # اجرای وابستگی‌ها و برنامه به صورت محلی
make db-connect         # اتصال به PostgreSQL
📁 ساختار پروژه
text
.
├── api/                    # Proto files و کدهای تولیدی gRPC
├── cmd/server/             # نقطه ورود اپلیکیشن
├── deploy/
│   ├── docker/             # Dockerfile و فایل‌های مرتبط
│   └── migrations/         # Migrationهای دیتابیس
├── internal/
│   ├── adapter/
│   │   ├── inbound/        # gRPC, HTTP, WebSocket
│   │   └── outbound/       # PostgreSQL, Redis, NATS, Triton
│   ├── domain/
│   │   ├── entity/         # Entity ها
│   │   └── port/           # Interface های ورودی و خروجی
│   ├── service/            # منطق کسب‌و‌کار
│   └── worker/             # Worker Pool
├── test/mock/              # Mock های تست
├── docker-compose.yml      # سرویس‌های زیرساختی
├── go.mod
├── Makefile
└── README.md
🗺️ مسیر یادگیری
text
فاز ۰ (Foundation)
    ↓
فاز ۱ (HTTP + DB)
    ↓
فاز ۲ (gRPC + AI)
    ↓
فاز ۳ (Async Processing)
    ↓
فاز ۴ (WebSocket Real-Time)
    ↓
فاز ۵ (Performance Tuning)  ← شما اینجایید
    ↓
فاز ۶ (Monitoring)
    ↓
فاز ۷ (Testing & Tuning)
📜 License
MIT © 2025 – mohammadpnp
ساخته شده با ❤️ برای یادگیری و قدرت Go

