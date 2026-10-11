# Rovena

Moda ve aksesuar e-ticaret marketplace'i. Portföy/öğrenme projesi: Go mikroservisleri, PostgreSQL (servis başına DB), RabbitMQ, Saga + Transactional Outbox. Detaylar: `docs/readme.md`, `docs/architecture.md`.

## Yapı

- `backend/services/<ad>/` → identity, catalog, inventory, order, payment (her biri kendi `go.mod`'u, kendi DB'si)
- `backend/shared/` → servisler arası ortak Go modülü (errorx, response, httpx, auth/JWT doğrulama, logging). Servisler `replace` direktifiyle bağlanır.
- `backend/infrastructure/` → Postgres init, Nginx gateway
- `backend/docker-compose.yml` → yerel ortam. `make` ve `docker compose` komutları `backend/` içinden çalışır. Compose proje adı `rovena` olarak sabit (klasör adına bağlı değil; yoksa volume kaybolur).
- `docs/` kökte kalır (backend + ileride frontend ortak). Frontend eklenince `frontend/` kökte `backend/` ile yan yana olur.

## Mimari kararlar (stacks/go'dan sapmalar)

- Servis içi yapı `docs/architecture.md`'deki gibi: `domain / application / transport/http / repository / infrastructure` (stacks/go'daki düz `model/service/handler` değil).
- Fiber v2 + Bun kullanılır. PK'ler UUID (v7).
- API mesajları İngilizce.
- Application katmanı domain hataları döner; `backend/shared/httpx` ErrorHandler'ı servise özgü mapper ile bunları `errorx.AppError`'a çevirir. Handler'lar `return err` yapar.
- Servisler birbirinin tablosuna dokunmaz. Servisler arası: REST + RabbitMQ eventleri.
- Access token HS256 JWT (`JWT_SECRET` tüm servislerde aynı), refresh token opak + DB'de hash'li.

## Yol haritası

`docs/todo.md` backend planını fazlara böler (mesajlaşma altyapısı, Payment, Order/Saga, uçtan uca testler, gözlemlenebilirlik/CI). Bir faz bitince kutuları işaretle.

## Komutlar

- `make up` / `make down` → compose
- `make test` → tüm modüllerde `go test ./...`
- `make lint` / `make tidy-check` / `make build` → golangci-lint, go.mod denetimi, derleme (CI ile aynı)
- `make test-integration`, `make e2e` → gerçek Postgres/RabbitMQ testleri, uçtan uca ve hata enjeksiyonu senaryoları
- İade: iptal edilmiş siparişin parası otomatik iade edilir (Payment `RefundRequested` tüketir); müşteri `POST /orders/{id}/cancel` ile ödenmemiş siparişini iptal edebilir. Ayrıntı `docs/notes/17`.
- Gözlemlenebilirlik: Jaeger `localhost:16686`, Prometheus `localhost:9090`, her servisin `/metrics` yolu (bkz. `docs/notes/16`)
- `make seed-admin` / `make seed-catalog` → ilk admin / demo ürünler

## Öğrenme notları

`docs/notes/` projeyi mikroservisleri hiç bilmeyen biri için anlatan bölümlü rehberdir (Türkçe). Her yeni servis/özellik eklendiğinde ilgili bölümleri güncelle, `00-icindekiler.md` içindeki ilerleme günlüğünü ve gerekirse yeni bölüm/sözlük terimi ekle.
