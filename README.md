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
  <img src="https://img.shields.io/badge/Phase-4%20Completed-brightgreen" alt="Phase">
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
