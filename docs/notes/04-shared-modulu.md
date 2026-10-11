# 04 - Shared modülü

`backend/shared/` bütün servislerin ortak kullandığı küçük bir kütüphanedir. Amaç, her serviste aynı kodu kopyalamamaktır: hata biçimi, JSON cevap zarfı, token doğrulama gibi.

## Neden dikkatli olmalı?

Mikroservislerde ortak kod, servisleri birbirine bağlar. `shared` büyüdükçe "bir değişiklik herkesi etkiler" sorunu başlar. Bu yüzden kural: **yalnızca teknik altyapı** buraya girer, **iş mantığı asla** girmez (örn. "kullanıcı nedir" Identity'nin işidir, shared'de olmaz).

## Paketler

### `errorx/errorx.go`: Hata türleri

API istemcisine gösterilecek hatanın tek biçimi `AppError`'dır: HTTP durum kodu + mesaj (+ alan bazlı hatalar). Hazır hatalar: `ErrNotFound` (404), `ErrConflict` (409), `ErrUnauthorized` (401), `ErrForbidden` (403), `ErrValidation` (422), `ErrInternal` (500) vb.

Yardımcılar:
- `WithMessage(base, "...")`: durum kodunu koru, istemci mesajını değiştir.
- `WithCause(base, err)`: asıl hatayı sakla (loglanır, istemciye gitmez).
- `WithFields(base, map)`: hangi alanın neden geçersiz olduğunu ekle.

İstemciye iç hata detayı (SQL hatası gibi) asla sızmaz; yalnızca "Internal server error" gider, ayrıntı loga yazılır.

### `response/response.go`: Başarılı cevap zarfı

Her başarılı cevap aynı şekildedir:

```json
{ "success": true, "data": { ... } }
```

`OK` (200), `Created` (201), `NoContent` (204) ve liste cevapları için `Page[T]` (items, total, page, page_size) burada.

Hatalar da `{"success": false, "message": "...", "fields": {...}}` biçimindedir. Böylece frontend tek bir kalıbı ele alır.

### `httpx/`: HTTP yardımcıları

- **`errors.go` → `NewErrorHandler`**: Fiber'ın "merkezi hata yakalayıcısı". Handler'lar hata olunca kendi cevaplarını yazmaz, sadece `return err` yapar; bu fonksiyon hatayı JSON'a çevirir. Servis kendi **domain hatalarını** (`ErrEmailTaken` gibi) `AppError`'a çeviren bir "mapper" verebilir. 5xx hatalar loglanır.
- **`validate.go` → `BindAndValidate`**: JSON gövdesini struct'a okur ve `validate:"required,email"` gibi etiketleri kontrol eder. Hata mesajlarını alan yoluyla döner (iç içe alanlar için `images[1].url` gibi).
- **`middleware.go`**: `RequestIDMiddleware` (her isteğe kimlik numarası verir/iletir) ve `AccessLog` (her istek için tek satır JSON log: yol, durum, süre, IP, request id; gövde ve Authorization başlığı **asla** loglanmaz).
- **`pagination.go` → `Pagination`**: `?page` ve `?page_size` değerlerini sınırlar içinde okur (en az 1, en çok servis belirler).
- **`health.go` → `RegisterHealth`**: `/healthz` ("süreç ayakta mı", her zaman 200) ve `/readyz` ("bağımlılıklar hazır mı", örn. Postgres'e ping atar, değilse 503). Orkestrasyon araçları ve Compose bunu kullanır.

### `auth/`: Token doğrulama ve yetkilendirme

- **`claims.go`**: Access token'ın içeriği: kullanıcı id (`Subject`), `Role`, süre bilgileri. Rol sabitleri `customer` ve `admin`.
- **`verifier.go`**: Token imzasını, süresini, yayıncısını (`rovena-identity`) kontrol eder. Yalnızca HS256 kabul eder (saldırganın `alg: none` ile sahte token sunması engellenir).
- **`middleware.go`**: `Middleware(verifier)` geçerli `Authorization: Bearer ...` başlığı olmayan isteği 401 ile reddeder; `RequireRole("admin")` yetkisi yetmeyeni 403 ile reddeder.

Burada kritik fikir: **Token'ı yalnızca Identity üretir, ama her servis tek başına doğrulayabilir.** Catalog servisi bir admin işlemi için token görünce Identity'ye sormaz; aynı `JWT_SECRET` ile imzayı kendisi kontrol eder. Bu hem hızlıdır hem Identity düşse bile diğer servisler çalışır. Ayrıntı: [06](06-kimlik-dogrulama.md).

### `logging/logging.go`

Yapılandırılmış (JSON) log açar: `{"time":..., "level":"INFO", "service":"identity", "msg":...}`. Her satıra servis adı eklenir. JSON log, log toplama araçlarında aranabilir ve filtrelenebilir.

### `telemetry/`: İz ve metrik kurulumu

OpenTelemetry'nin tek kurulum noktası: iz sağlayıcı, metrik sağlayıcı (Prometheus biçiminde), `traceparent` taşıma yardımcıları ve `/metrics` işleyicisi. Her servis `main` içinde bir kez `telemetry.Setup` çağırır. `httpx.Tracing()` HTTP isteklerini, `messaging` mesajları izler. Ayrıntı: [16](16-gozlemlenebilirlik-ve-ci.md).

### `events/`: Olay sözleşmeleri (`payment.go`, `order.go`)

Servisler arası olayların adları, sürümleri ve payload yapıları. Davranış içermez, yalnızca veri şekli: yayıncı ve tüm tüketiciler aynı tanımı okur, böylece sessizce birbirinden uzaklaşamazlar. Mantık içermediği için "shared'e iş mantığı girmez" kuralını bozmaz. Bedeli: bir olayın şeklini değiştirmek hem yayıncıyı hem tüketiciyi etkiler, bu yüzden değişiklik yeni bir `version` ile yapılır. `payment.go` ödeme olaylarını, `order.go` `OrderConfirmed` ve `OrderCancelled`'ı tanımlar. Ayrıntı: [13](13-payment-servisi.md), [14](14-order-servisi-ve-saga.md).

### `messaging/` ve `outbox/`: Olaylar ve güvenilir yayın

RabbitMQ ile konuşmanın ortak kodu: olay zarfı, bağlantı ve yayın, tüketici döngüsü (retry, DLQ), Transactional Outbox ve idempotent Inbox. Bunlar da yalnızca teknik altyapıdır; hangi olayların var olduğu ve payload'ları ilgili servisin işidir. Ayrıntı: [12](12-mesajlasma.md).

`outbox/schema.sql` bir Go dosyası değil, servislerin kendi `migrations/` klasörüne kopyaladığı SQL şablonudur: her servisin outbox tablosu kendi veritabanında durur.

## Testler

`auth/verifier_test.go`: geçerli token kabul, yanlış secret, süresi geçmiş, yanlış yayıncı, eksik alanlar ve `alg: none` gibi saldırı senaryolarının reddi.

`messaging/event_test.go`: zarf birim testleri. `messaging/integration_test.go` ve `outbox/integration_test.go`: gerçek RabbitMQ ve PostgreSQL ile retry, DLQ, outbox ve idempotency senaryoları (`make test-integration`).
