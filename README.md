<p align="center">
  <img src="https://img.icons8.com/fluency/96/artificial-intelligence.png" alt="AI Shield" width="120"/>
</p>

<h1 align="center">🛡️ Content Moderator · AI‑Powered Real‑Time Filtering</h1>

<p align="center">
  <em>A high‑performance, event‑driven service that automatically detects and filters toxic, spam, or adult content in real time using deep‑learning models – built with Go and Hexagonal Architecture.</em>
</p>

<p align="center">
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white" alt="Go version"></a>
  <a href="https://github.com/mohammadpnp/content-moderator/actions"><img src="https://img.shields.io/badge/tests-passing-success?logo=githubactions" alt="CI"></a>
  <a href="#"><img src="https://img.shields.io/badge/coverage-83%25-brightgreen" alt="Coverage"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="License"></a>
  <a href="#"><img src="https://img.shields.io/badge/status-production%20ready-%23brightgreen" alt="Status"></a>
</p>

---

## 🌟 Highlights

- **Hexagonal Architecture** – business logic completely isolated from transport and infrastructure.
- **Dual API**: REST (Fiber) + gRPC with server‑side streaming and bidirectional streaming.
- **Real‑time User Notification** via WebSocket, backed by Redis Pub/Sub and a notification bridge.
- **Asynchronous AI Processing** using NATS JetStream and a configurable worker pool with rate‑limiting and idempotency.
- **AI Model Serving** with NVIDIA Triton Inference Server (text & image moderation).
- **Resilience Patterns**: Circuit Breaker, Exponential Backoff Retries, Dead Letter Queue (DLQ), and Distributed Cache Stampede Protection.
- **Comprehensive Test Suite** (>80% coverage) – unit, integration, streaming, load tests, and extensive mocking.
- **Production Observability** – Prometheus metrics, Jaeger tracing, structured logging, and pprof profiling.

---

## 🧠 Use Case

In a modern social platform, users constantly upload text posts and images. Every piece of content must be screened for **hate speech, spam, violence, and adult material** before it becomes visible. This service provides:

- **Instant acknowledgement** of submission (HTTP/gRPC).
- **Asynchronous AI moderation** with low latency.
- **Real‑time push** of the moderation decision to the user’s device (WebSocket).

---

## 🧱 Hexagonal Architecture (Ports & Adapters)
