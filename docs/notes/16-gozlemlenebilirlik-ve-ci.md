# 16 - Gözlemlenebilirlik, CI ve kod kalitesi

Servis sayısı arttıkça "bir şey yavaş" ya da "bu sipariş neden takıldı" sorularının cevabı tek bir logda değil, beş servisin ve bir mesaj kuyruğunun arasında kalır. Bu bölüm üç şeyi anlatır: bir isteğin servisler arası **izini sürmek** (trace), sistemin sağlığını **sayılarla görmek** (metrik) ve kodun her değişiklikte **otomatik denetlenmesi** (CI, linter).

## Üç sinyal

| Sinyal | Soru | Nerede |
| --- | --- | --- |
| **Log** | Bu serviste tam olarak ne oldu? | `docker compose logs <servis>`; her satırda `trace_id` |
| **Trace** | Bu istek hangi servislerden, hangi sırayla, ne kadar sürede geçti? | Jaeger: http://localhost:16686 |
| **Metrik** | Sistem genel olarak nasıl gidiyor? (istek sayısı, gecikme, hata, kuyruk birikimi) | Prometheus: http://localhost:9090 |

Üçü birbirine `trace_id` ile bağlanır: Jaeger'da bir iz bulursun, aynı kimlikle logları ararsın.

## Trace: bir isteğin yolculuğu

Her HTTP isteği bir **span** (iz parçası) açar; aynı akışın tüm span'ları aynı `trace_id`'yi taşır. Bir checkout'un gerçek izi (Jaeger'dan):

```text
order      POST /api/v1/checkout
order        POST catalog                       (istemci span'ı: Order'ın Catalog'a çağrısı)
catalog        POST /api/v1/internal/variants/lookup
order        POST inventory
inventory      POST /api/v1/internal/reservations
order        publish PaymentRequested           (outbox'tan çıkıp RabbitMQ'ya)
payment        process payment.requests         (mesajı tüketen taraf)
payment        publish PaymentSucceeded
order            process order.payment-results
order              POST inventory
inventory            POST /api/v1/internal/reservations/:id/commit
```

Dört servis, iki yön, bir mesaj kuyruğu: hepsi tek ağaç.

### Trace servisler arasında nasıl taşınır?

İz bilgisi W3C standardındaki **`traceparent`** değeridir (`00-<trace id>-<span id>-01`).

- **HTTP:** `Order → Catalog/Inventory` çağrılarında istek başlığı olarak gider; karşı taraf `httpx.Tracing()` ile başlığı okuyup iz'e devam eder. Gateway (Nginx) bilinmeyen başlıkları olduğu gibi geçirir.
- **Mesajlar:** Burada ince bir nokta var. Olay, outbox tablosuna **iş yapılırken** yazılır ama RabbitMQ'ya saniyeler sonra, başka bir döngü (relay) tarafından gönderilir. O an izlenen şey artık o HTTP isteği değildir. Bu yüzden `traceparent` **olay zarfının içine** yazılır (`Envelope.Traceparent`), olayla birlikte veritabanında bekler; relay yayınlarken iz'e oradan devam eder. Yayın span'ı ayrıca mesaj başlığına kendi `traceparent`'ini koyar; tüketici o başlığa bağlanır. Böylece zincir kopmaz, eski (alanı olmayan) zarflar da geçerli kalır.
- Tüketicinin handler'ı iz içinde çalıştığı için, handler'ın yayınladığı bir sonraki olay da aynı iz'e bağlanır (`PaymentSucceeded` yukarıdaki gibi).

Testle kanıtlanır: `TestTraceSurvivesTheOutboxTheBrokerAndTheNextHop` (gerçek RabbitMQ). İstek bitmiş, olay bağlamsız bir döngüden yayınlanmış, tüketici başka bir bağlamda ve yine de aynı `trace_id`.

### Dışarı gönderme (export)

`OTEL_EXPORTER_OTLP_ENDPOINT` tanımlıysa (Compose'ta `http://jaeger:4318`) span'lar Jaeger'a gider. Tanımlı değilse span'lar yine oluşur (logda `trace_id` görünür, servisler arası taşınır) ama hiçbir yere gönderilmez; maliyeti ihmal edilebilir. Jaeger bellekte tutar, yeniden başlayınca izler silinir.

## Metrikler

Her servis `/metrics` yolunda Prometheus formatında sayılarını sunar (servis portunda, örn. `localhost:8086/metrics`; gateway bu yolu yönlendirmez). Prometheus 10 saniyede bir okur.

| Metrik | Ne söyler |
| --- | --- |
| `http_server_request_duration_seconds` (histogram) | Her servisin istek sayısı ve gecikmesi; `http_route` (**şablon**, `/orders/:id`), `http_request_method`, `http_response_status_code` etiketleriyle |
| `http_client_request_duration_seconds` | Order'ın Catalog/Inventory çağrıları; `peer` ve sonuç (`200`, `409`, `error`) |
| `messaging_events_published_total` | Yayınlanan olaylar, tip ve sonuç |
| `messaging_events_consumed_total` | Tüketilen olaylar; `result`: `ok`, **`retried`** (yeniden deneme), `dead_lettered` (DLQ) |
| `messaging_handler_duration_seconds` | Bir olayın işlenme süresi |
| `outbox_pending_events`, `outbox_oldest_pending_age_seconds` | **Outbox birikimi**: yayınlanmayı bekleyen olay sayısı ve en eskisinin yaşı. Broker'a ulaşılamıyorsa bu sayı büyür |
| `rabbitmq_detailed_queue_messages_ready` | **Kuyruk birikimi**: kuyrukta bekleyen mesaj (RabbitMQ'nun kendi eklentisinden) |
| `orders_finished_total` | Sonuçlanan siparişler: `confirmed`, `cancelled` + iptal sebebi |
| `saga_recovery_actions_total` | Kurtarma işinin ne yaptığı (`payment_requested_again`, `reserve_again`, ...). Normalde sıfıra yakın olmalı |
| `payments_completed_total` | Tamamlanan ödemeler, durum ve hata kodu |
| `go_*`, `process_*` | Go çalışma zamanı: goroutine, bellek, CPU |

Deneyebileceğin sorgular (Prometheus arayüzünde):

```text
histogram_quantile(0.95, sum by (le, http_route) (rate(http_server_request_duration_seconds_bucket[5m])))   # rota başına p95 gecikme
sum by (queue, result) (rate(messaging_events_consumed_total[5m]))                                          # tüketim sonuçları
outbox_pending_events                                                                                       # birikim
sum by (code) (orders_finished_total{status="cancelled"})                                                   # neden iptal oluyor
```

### Etiket sayısı (cardinality) tuzağı

Metrik etiketi olarak **hiçbir zaman** sipariş id'si ya da ham URL kullanılmaz. `/orders/123` ve `/orders/124` ayrı seri olursa Prometheus'un belleği sınırsız büyür. Bu yüzden rota etiketi şablondur (`/orders/:id`), eşleşmeyen yollar tek `unmatched` etiketine düşer, sağlık ve `/metrics` istekleri hiç sayılmaz. `TestMetricsUseTheRouteTemplateNotTheURL` bunu doğrular.

Bir de gerçek bir hata yakalandı: Fiber'in döndürdüğü metinler bir sonraki istekte yeniden kullanılan tamponlara işaret eder. Metrik etiketine kopyalamadan yazılan `GET`, aynı bağlantıdaki sonraki istekten sonra `GETT`, `PUTETE` gibi bozuk değerlere dönüşüyordu. Birim testlerde görünmedi, canlı yığında göründü. Artık `strings.Clone` ile kopyalanıyor ve gerçek bir soket üzerinden keep-alive bağlantı kullanan bir test var (`TestLabelsSurviveConnectionReuse`).

## Kodun nerede olduğu

| Ne | Yer |
| --- | --- |
| Kurulum, yardımcılar | `backend/shared/telemetry/` |
| HTTP middleware, `/metrics` | `backend/shared/httpx/tracing.go` |
| Log'a `trace_id` | `backend/shared/logging/` |
| Mesaj izi ve metrikleri | `backend/shared/messaging/` |
| Outbox birikimi | `backend/shared/outbox/relay.go` |
| Prometheus ayarı | `backend/infrastructure/prometheus/prometheus.yml` |

Bilerek yapılmayan: veritabanı sorgu span'ları, Grafana panoları, uyarı kuralları (alerting), örnekleme (sampling) ayarı. Hepsi bu temelin üstüne eklenebilir.

---

## CI: her değişiklikte otomatik denetim

`.github/workflows/ci.yml` her `push` ve `pull request` için şunları çalıştırır:

| İş | Ne yapar | Neden |
| --- | --- | --- |
| `static` | `go build`, `go vet`, `gofmt`, `go mod tidy` farkı, `go mod verify` | Derlenmeyen ya da biçimsiz kod, tutarsız bağımlılık dosyası |
| `lint` (6 modül ayrı) | `golangci-lint` | Yoksayılan hatalar, hatalı hata sarmalama, ölü kod |
| `vulncheck` | `govulncheck` | Bağımlılıklardaki bilinen güvenlik açıkları |
| `unit` | `go test -race` | Birim ve HTTP testleri, veri yarışı denetimiyle |
| `integration` | Compose'taki Postgres + RabbitMQ'ya karşı `make test-integration` | Gerçek SQL, kilit, kuyruk |
| `openapi` | `openapi-spec-validator` | Şemalar geçerli mi |
| `e2e` | `make up` + `make e2e` | Beş servis birlikte; hata enjeksiyonu senaryoları |

`e2e` en son çalışır ve yalnızca `unit` ile `integration` geçerse başlar.

**Testcontainers yerine Compose.** Planda "Testcontainers ile kendi kendine yeten" yazıyordu. Amaç, CI'ın elle kurulum istememesiydi. Bunu daha az yeni bağımlılıkla sağlayan yol, CI'da da yerelde kullandığımız aynı `docker-compose.yml`'dir: `scripts/ci-env.sh` rastgele sırlarla bir `.env` üretir, `docker compose up --wait postgres rabbitmq` altyapıyı kaldırır, `scripts/migrate-local.sh` şemaları kurar. Böylece CI ile laptop aynı tanımı kullanır ve test kodu değişmez. Test kodunun kendi konteynerini başlatması gerekirse Testcontainers eklenebilir.

### Yerelde aynı denetimler

```bash
cd backend
make build vet lint tidy-check test     # CI'ın static + lint + unit işleri
make test-integration                   # yığın çalışırken
make e2e
```

`make lint` için `golangci-lint` v2 gerekir (`brew install golangci-lint`). Ayarlar `backend/.golangci.yml`: standart set (errcheck, govet, staticcheck, unused, ineffassign) ve `bodyclose` (kapatılmayan HTTP gövdeleri), `errorlint` (`errors.Is/As`, `%w`), `misspell`, `unconvert`, `copyloopvar`. Testlerde `errcheck` ve `errorlint` kapalı.

### Bu işlerin yakaladıkları

Kuruldukları gün gerçek bulgular çıktı, yani boşuna değil:

- `make tidy-check`: Inventory, Payment ve Order `go.mod` dosyaları `go mod tidy` çıktısıyla uyuşmuyordu.
- Linter: yoksayılan `Close`/`Nack` hataları, `%v` ile sarılıp `errors.Is` ile ayrılamayan hatalar (Order'ın Catalog/Inventory istemcileri), kullanımdan kaldırılmış `bun.In`, kullanılmayan bir test yardımcısı.
- Linter test kodundaki kapatılmamış HTTP gövdelerini de buldu.

## Bilinen eksikler

- CI iş akışı bu makinede çalıştırılamadı (GitHub'a gitmeden denenemez); komutların her biri yerelde ayrı ayrı çalıştırıldı. İlk push'ta bir adım adı/sürümü uyuşmazsa düzeltilmesi gerekir.
- Trace'ler örneklenmez (hepsi kaydedilir). Gerçek trafikte `OTEL_TRACES_SAMPLER` ile oran düşürülmeli.
- Jaeger ve Prometheus verisi kalıcı değil.
- Uyarı kuralı yok: birikim büyürse kimse haberdar edilmiyor, panodan bakmak gerekir.
