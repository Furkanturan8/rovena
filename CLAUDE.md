# Rovena

Moda ve aksesuar e-ticaret marketplace'i. Portföy/öğrenme projesi: Go mikroservisleri, PostgreSQL (servis başına DB), RabbitMQ, Saga + Transactional Outbox. Detaylar: `docs/readme.md`, `docs/architecture.md`.

## Yapı

- `backend/services/<ad>/` → identity, catalog, inventory, order, payment (her biri kendi `go.mod`'u, kendi DB'si)
- `backend/shared/` → servisler arası ortak Go modülü (errorx, response, httpx, auth/JWT doğrulama, logging). Servisler `replace` direktifiyle bağlanır.
- `backend/infrastructure/` → Postgres init, Nginx gateway
- `backend/docker-compose.yml` → yerel ortam. `make` ve `docker compose` komutları `backend/` içinden çalışır. Compose proje adı `rovena` olarak sabit (klasör adına bağlı değil; yoksa volume kaybolur).
- `docs/` kökte kalır (backend + frontend ortak).
- `frontend/web/` → mağaza vitrini: Next.js 16 (App Router) + TypeScript + Tailwind v4, TR/EN. Veriyi gateway'den (`API_BASE_URL`) alır: katalog, stok, üyelik, sepetten siparişe akış (bkz. `docs/notes/20`). Tasarım dili "Editorial Maison" (`frontend/example-design-referans/*/DESIGN.md`): 0px radius, Playfair Display + Plus Jakarta Sans, token'lar `app/globals.css` `@theme`. Rotalar `/{tr|en}/...` (`collection`, `product/[slug]`, `cart`, `checkout`, `account/*`, `gallery`, `about`). Metinler `lib/i18n/dictionaries/{tr,en}.ts` içinde (yeni metin eklerken ikisine de ekle); sunucuda `getI18n(params)`, istemcide `useDict()/useLocale()`, linkler için `components/ui/link.tsx`. API katmanı `lib/api/` (sunucu çağrıları, eşleyiciler), sunucu veri fonksiyonları `lib/data/`, oturum `lib/auth/` (httpOnly çerezler, `proxy.ts` yeniler). Ürün metinleri DB'de (`translations`/`attributes` jsonb); seed `backend/services/catalog/cmd/seed/store.json`. Frontend dev sunucusu 3100 portunda (3000'i Grafana tutuyor): `npm run dev -- -p 3100`. Animasyonlar `motion` ile (`components/motion/`); videolar Pexels (ücretsiz lisans) kaynaklı, `lib/media.ts`'den tanımlı; medya dosyaları Supabase Storage'da (`NEXT_PUBLIC_MEDIA_URL`).

## Mimari kararlar (stacks/go'dan sapmalar)

- Servis içi yapı `docs/architecture.md`'deki gibi: `domain / application / transport/http / repository / infrastructure` (stacks/go'daki düz `model/service/handler` değil).
- Fiber v2 + Bun kullanılır. PK'ler UUID (v7).
- API mesajları İngilizce.
- Application katmanı domain hataları döner; `backend/shared/httpx` ErrorHandler'ı servise özgü mapper ile bunları `errorx.AppError`'a çevirir. Handler'lar `return err` yapar.
- Sipariş `confirmed → shipped → delivered` ile ilerler (son ikisi admin uçlarıyla, Saga dışında). Checkout `shipping_address` ister; sipariş adresin kopyasını saklar. Identity adres defteri, şifre ve e-posta uçlarını `/users/me/*` altında sunar.
- Servisler birbirinin tablosuna dokunmaz. Servisler arası: REST + RabbitMQ eventleri.
- Access token HS256 JWT (`JWT_SECRET` tüm servislerde aynı), refresh token opak + DB'de hash'li.

- `docs/observability-lab/` → Prometheus/Grafana/Jaeger/RabbitMQ'yu öğreten bağımsız, derlemesiz interaktif laboratuvar (`python3 -m http.server`). Metrik/kuyruk adlarını değiştirirsen `js/scenarios*.js`, `js/guide.js` ve `js/architecture.js`'i güncelle. Detay: `docs/notes/18`.
- `docs/saga-lab/` → checkout Saga'sını öğreten laboratuvar (`docs/` kökünden servis edilir; `../observability-lab/js` ortak dosyalarını kullanır). Saga davranışı (durumlar, telafiler, kurtarma kuralları) değişirse `js/sim.js` ve `js/code.js`'i güncelle. Detay: `docs/notes/19`.

## Yol haritası

`docs/todo.md` backend planını fazlara böler (mesajlaşma altyapısı, Payment, Order/Saga, uçtan uca testler, gözlemlenebilirlik/CI). Bir faz bitince kutuları işaretle.

## Komutlar

- `make up` / `make down` → compose
- `make test` → tüm modüllerde `go test ./...`
- `make lint` / `make tidy-check` / `make build` → golangci-lint, go.mod denetimi, derleme (CI ile aynı)
- `make test-integration`, `make e2e` → gerçek Postgres/RabbitMQ testleri, uçtan uca ve hata enjeksiyonu senaryoları
- İade: iptal edilmiş siparişin parası otomatik iade edilir (Payment `RefundRequested` tüketir); müşteri `POST /orders/{id}/cancel` ile ödenmemiş siparişini iptal edebilir. Ayrıntı `docs/notes/17`.
- Gözlemlenebilirlik: Jaeger `localhost:16686`, Prometheus `localhost:9090`, her servisin `/metrics` yolu (bkz. `docs/notes/16`)
- Yayın: backend VPS'te `make prod-up` (gateway 127.0.0.1:18080; HTTPS'i sunucudaki nginx sağlar), frontend Vercel'de; adımlar `docs/deploy.md`
- `make seed-admin` / `make seed-catalog` → ilk admin / demo ürünler

## Öğrenme notları

`docs/notes/` projeyi mikroservisleri hiç bilmeyen biri için anlatan bölümlü rehberdir (Türkçe). Her yeni servis/özellik eklendiğinde ilgili bölümleri güncelle, `00-icindekiler.md` içindeki ilerleme günlüğünü ve gerekirse yeni bölüm/sözlük terimi ekle.
