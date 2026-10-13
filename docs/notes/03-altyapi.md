# 03 - Altyapı

Altyapı, iş kodu olmayan ama servislerin çalışması için gereken parçalardır: veritabanı, mesaj kuyruğu, gateway. Hepsi Docker konteyneri olarak çalışır.

## Docker ve Docker Compose

- **Konteyner:** Bir programı bağımlılıklarıyla birlikte izole çalıştıran paket. "Bende çalışıyor" sorununu bitirir.
- **İmaj:** Konteynerin şablonu. `Dockerfile` ile üretilir.
- **Docker Compose:** Birden fazla konteyneri tek dosyada (`backend/docker-compose.yml`) tanımlayıp tek komutla (`docker compose up`) başlatır. Konteynerler aynı sanal ağa girer ve birbirine **servis adıyla** ulaşır (örn. `postgres:5432`, `identity:8081`).

## `backend/docker-compose.yml` içindeki servisler

| Servis | Ne yapar | Dışarıya açık port |
| --- | --- | --- |
| `postgres` | PostgreSQL 17 veritabanı sunucusu | 127.0.0.1:5433 (yalnızca kendi bilgisayarın) |
| `rabbitmq` | Mesaj kuyruğu (Payment ve Order kullanıyor) | 5672 (mesaj), 15672 (yönetim paneli), 15692 (Prometheus metrikleri) |
| `identity-migrate` | Identity tablolarını kurar, işi bitince kapanır | yok |
| `identity` | Identity API sunucusu | 127.0.0.1:18081 |
| `identity-seed` | İlk admin kullanıcıyı oluşturur (`tools` profilinde, elle çalıştırılır) | yok |
| `catalog-migrate` | Catalog tablolarını kurar, işi bitince kapanır | yok |
| `catalog` | Catalog API sunucusu | 127.0.0.1:8082 |
| `catalog-seed` | Demo kategori ve ürünleri yükler (`tools` profili) | yok |
| `inventory-migrate` | Inventory tablolarını kurar, işi bitince kapanır | yok |
| `inventory` | Inventory API sunucusu (içinde rezervasyon süpürücüsü de çalışır) | 127.0.0.1:8083 |
| `inventory-seed` | Demo varyantlara açılış stoğu yazar (`tools` profili) | yok |
| `payment-migrate` | Payment tablolarını (ve outbox/inbox) kurar, işi bitince kapanır | yok |
| `payment` | Payment servisi: RabbitMQ'dan `PaymentRequested` tüketir, sonuç olaylarını yayınlar; HTTP yalnızca okuma | 127.0.0.1:8085 |
| `order-migrate` | Order tablolarını (ve outbox/inbox) kurar, işi bitince kapanır | yok |
| `order` | Order servisi: sepet, checkout, Saga; Catalog ve Inventory'yi REST ile çağırır, ödeme olaylarını dinler, kurtarma işini çalıştırır | 127.0.0.1:8086 |
| `jaeger` | Trace arayüzü ve toplayıcısı (bellekte saklar); servisler OTLP ile iz gönderir | 127.0.0.1:16686 |
| `prometheus` | Her servisin `/metrics` yolunu ve RabbitMQ kuyruk metriklerini 10 sn'de bir okur | 127.0.0.1:9090 |
| `grafana` | Hazır panolar (`Rovena overview`); Prometheus ve Jaeger'dan okur, geliştirmede girişsiz | 127.0.0.1:3000 |
| `gateway` | Nginx, tek giriş kapısı | 8080 |

Önemli ayrıntılar:

- **`depends_on` + `condition`:** `identity`, `identity-migrate` başarıyla bitmeden başlamaz (`service_completed_successfully`). `identity-migrate` de Postgres "healthy" olmadan başlamaz. Sadece "konteyner başladı" yetmez, "hazır" olması beklenir (`healthcheck`).
- **Volume'ler** (`postgres-data`, `rabbitmq-data`): Konteyner silinse bile veritabanı verisi kalsın diye diskte tutulur. Sıfırdan başlamak için `docker compose down -v`.
- **`${DEGISKEN:?mesaj}`:** `backend/.env` dosyasından değer okur; yoksa Compose hata verir. Böylece parolasız çalışmaz.
- **`127.0.0.1:` öneki:** Port yalnızca yerel makineden erişilebilir; ağdaki başkaları veritabanına bağlanamaz.

## PostgreSQL: servis başına veritabanı

`backend/infrastructure/postgres/init-databases.sh` Postgres ilk kez başlarken bir kez çalışır ve her servis için **ayrı veritabanı + ayrı kullanıcı** oluşturur:

| Veritabanı/kullanıcı | Sahibi |
| --- | --- |
| `identity` | Identity servisi |
| `catalog` | Catalog servisi |
| `inventory` | Inventory servisi |
| `orders` | Order servisi |
| `payment` | Payment servisi |

Tek Postgres sunucusu kullanılıyor (yerel geliştirme için pratik), ama her servisin kullanıcısı yalnızca kendi veritabanına erişebilir (`REVOKE ALL ... FROM PUBLIC`). Identity'nin parolasıyla Catalog'un veritabanını açamazsın. Bu, "servis başına veritabanı" kuralını teknik olarak zorlar. Üretimde bunlar ayrı sunucular da olabilir.

Betik yalnızca **boş volume'da** çalışır. Sonradan değiştirirsen `docker compose down -v` ile volume'u silip yeniden başlatman gerekir.

## Migration nedir?

Veritabanı tablolarının şeması kodla birlikte sürümlenir. `backend/services/identity/migrations/` altındaki SQL dosyaları sırayla uygulanır:

Identity:
- `00001_create_users.sql` → `users` tablosu
- `00002_create_refresh_tokens.sql` → `refresh_tokens` tablosu
- `00003_refresh_token_rotation.sql` → `rotated_at` (yenileme ile çıkış yapmayı ayırır; iki sekme hoşgörüsü için)

Catalog:
- `00001_create_catalog.sql` → kategoriler, ürünler, görseller, varyantlar (+ `pg_trgm` eklentisi)
- `00002_add_search_text.sql` → aksan duyarsız arama sütunu
- `00003_variants_price_index.sql` → fiyat sıralamasını hızlandıran kısmi indeks

Payment:
- `00002_refunds.sql` → `refunded` durumu, `refund_ref`, `refunded_at`; para alınmış sayılan durumlar (`succeeded`, `refunded`) için sipariş başına tek kayıt
- `00003_outbox_cleanup_indexes.sql` → outbox temizleyicisi için yaş indeksleri
- `00001_create_payments.sql` → ödemeler (tutarsız durumları ve sipariş başına ikinci başarıyı engelleyen kısıtlarla) + `outbox_events` + `processed_events`

Order:
- `00001_create_orders.sql` → sepet, siparişler (durum/rezervasyon tutarlılığını koruyan kısıtlarla), sipariş satırları (fiyat görüntüsü), durum geçmişi + `outbox_events` + `processed_events`
- `00002_refunds.sql` → `refund_state`, `refund_requests` (yalnızca iptal edilmiş siparişte iade durumu olabilir)
- `00003_outbox_cleanup_indexes.sql` → outbox temizleyicisi için yaş indeksleri

Inventory:
- `00001_create_inventory.sql` → stok kalemleri, rezervasyonlar, rezervasyon kalemleri, hareket defteri (stoğun söz verilenin altına düşmesini engelleyen `CHECK` kısıtlarıyla)

`00002` dosyası, **uygulanmış bir migration'ın düzenlenmediği**, değişikliğin yeni dosyayla eklendiği örnektir.

Her dosyada `-- +goose Up` (uygula) ve `-- +goose Down` (geri al) bölümü vardır. Hangilerinin uygulandığını [goose](https://github.com/pressly/goose) kütüphanesi veritabanındaki bir tabloda tutar. Yeni bir değişiklik her zaman **yeni bir dosya** olarak eklenir; eski migration düzenlenmez. `identity-migrate` konteyneri bunu `up` ile çalıştırır.

## Nginx: API Gateway

`backend/infrastructure/nginx/nginx.conf`:

- `/api/v1/auth/...` ve `/api/v1/users/...` isteklerini `http://identity:8081`'e, `/api/v1/products`, `/categories` ve `/admin/products|categories` isteklerini `http://catalog:8082`'ye, `/api/v1/stock` ve `/admin/stock/...` isteklerini `http://inventory:8083`'e, `/payments` ve `/admin/payments` isteklerini `http://payment:8085`'e, `/cart`, `/checkout`, `/orders` ve `/admin/orders` isteklerini `http://order:8086`'ya yönlendirir. Checkout, başka bir servisi beklediği tek yer olduğu için ayrı ve daha sıkı bir hız sınırı (burst 10) alır. Yeni servis eklendikçe buraya yeni `location` bloğu eklenir.
- Inventory'nin ve Catalog'un `/api/v1/internal/...` uçları **bilerek yönlendirilmez**: rezervasyon uçları yalnızca servisler arasıdır, internetten erişilebilir olmamalıdır ([11](11-inventory-servisi.md)).
- `limit_req`: IP başına saniyede 30 istek (burst 60) sınırı (kaba saldırı koruması). Aşılırsa `limit_req_status 429` sayesinde JSON zarflı **429** ve `Retry-After` döner. Nginx'in varsayılanı 503'tür ve bir kesinti gibi okunur; bu yüzden bilerek değiştirildi.
- `X-Request-ID` başlığı yoksa üretir ve servise iletir. Bir isteği tüm servislerin loglarında izlemek için kullanılır.
- `resolver 127.0.0.11` + `set $identity ...`: Nginx, servis henüz ayakta değilken de başlayabilsin diye adresi istek anında çözer.
- Bilinmeyen yollar 404 döner.

`proxy_params.conf`, yönlendirilen istekte servise gönderilen başlıkları (`X-Forwarded-For` vb.) tanımlar. Servis, istemcinin gerçek IP'sini bu başlıktan okur (rate limit için gerekli). Servis bu başlığa yalnızca güvenilen proxy adreslerinden gelirse inanır (`TRUSTED_PROXIES`).

## RabbitMQ

Mesaj kuyruğu sunucusu. Servisler arası asenkron olayları taşır (bkz. [12](12-mesajlasma.md)). İki kullanıcısı var: Payment `payment.requests` kuyruğundan `PaymentRequested`'i dinler; Order `order.payment-results` kuyruğundan `PaymentSucceeded` ve `PaymentFailed`'i dinler ve `PaymentRequested`'i kendisi yayınlar. Yönetim paneli: http://localhost:15672 (kullanıcı/parola `backend/.env` içinde).

## `backend/.env` ve gizli değerler

`backend/.env.example` hangi değişkenlerin gerektiğini gösterir. Gerçek `backend/.env`:

- Veritabanı parolaları (servis başına ayrı)
- `RABBITMQ_*`
- `JWT_SECRET`: token imzalama anahtarı, en az 32 karakter, **tüm servislerde aynı** olmalı. Sızarsa herkes admin token'ı üretebilir.
- `INTERNAL_API_KEY`: servisler arası çağrıların ortak anahtarı (en az 32 karakter). Inventory'nin rezervasyon uçları, Catalog'un fiyat arama ucu ve Payment'ın dahili okuma ucu ister; Order servisi bunu Catalog ve Inventory çağrılarında gönderir.
- `ADMIN_EMAIL`, `ADMIN_PASSWORD`: `make seed-admin` ile oluşturulacak ilk admin.

Compose bu değerleri konteynerlere ortam değişkeni olarak verir; servis kodu `internal/infrastructure/config` paketiyle okur.

## Dockerfile (`backend/services/identity/Dockerfile`)

İki aşamalı (multi-stage) build:

1. **build aşaması:** Go imajında kodu derler. Önce yalnızca `go.mod/go.sum` kopyalanıp bağımlılıklar indirilir; kod değişince bu katman yeniden indirilmez (build cache). Üç program derlenir: `api`, `migrate`, `seed`.
2. **çalışma aşaması:** Sadece derlenmiş dosyaları, içinde neredeyse hiçbir şey olmayan `distroless` imajına koyar ve **root olmayan** kullanıcıyla çalıştırır. Küçük imaj, küçük saldırı yüzeyi.

Build context kök dizindir (`docker build -f backend/services/identity/Dockerfile .`), çünkü imaj hem `backend/services/identity` hem `shared` klasörüne ihtiyaç duyar.
