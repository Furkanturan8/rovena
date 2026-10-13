# Rovena API şemaları (OpenAPI 3.0)

| Dosya | Servis | Kapsam |
| --- | --- | --- |
| [identity.yaml](identity.yaml) | Identity | kayıt, giriş, refresh, logout, profil, admin kullanıcı yönetimi |
| [catalog.yaml](catalog.yaml) | Catalog | kategoriler, ürün arama/filtre, admin ürün/varyant/kategori yönetimi |
| [inventory.yaml](inventory.yaml) | Inventory | stok sorgusu (herkese açık), admin depo yönetimi, Order servisi için **dahili** rezervasyon uçları |
| [order.yaml](order.yaml) | Order | sepet, checkout (`Idempotency-Key`), müşterinin siparişleri, admin sipariş uçları, sipariş durumları ve olaylar |
| [payment.yaml](payment.yaml) | Payment | ödeme okuma uçları (müşteri, admin, **dahili**) ve `PaymentRequested` / `PaymentSucceeded` / `PaymentFailed` olay sözleşmeleri, test kartı token'ları |

Her şema koddaki DTO, doğrulama etiketleri ve hata eşlemeleriyle uyumlu yazıldı ve çalışan API'ye karşı doğrulandı (aşağıya bak). Tüm istekler **gateway** üzerinden gider: `http://localhost:18080`. Yeni servis eklendikçe buraya yeni bir dosya eklenir.

## Hızlı başlangıç: API'yi elle denemek

```bash
make up              # servisleri başlat
make seed-admin      # admin@rovena.local hesabı (parola: .env içindeki ADMIN_PASSWORD)
make seed-catalog    # demo kategori ve ürünler
make seed-inventory  # demo ürünlerin stokları
```

1. Şemayı bir istemciye aktar. Postman, Insomnia, Bruno ve Hoppscotch `.yaml` dosyasını doğrudan içe aktarır (Import > dosyayı seç). Her uç, açıklama ve örnek gövdeyle gelir.
2. `POST /api/v1/auth/login` ile giriş yap, cevaptaki `data.tokens.access_token` değerini kopyala.
3. Korumalı uçlar için `Authorization: Bearer <access_token>` ekle (istemcide "Auth > Bearer Token"). `/admin/...` uçları için admin hesabının token'ı gerekir.
4. **`/internal/...` uçları** (Inventory'nin rezervasyonları, Catalog'un fiyat araması, Payment'ın dahili okuması) gateway'den geçmez ve JWT istemez. Servis portuna (Catalog `http://localhost:8082`, Inventory `http://localhost:8083`, Payment `http://localhost:8085`) `X-Internal-Key: <INTERNAL_API_KEY>` başlığıyla istek atılır; anahtar `backend/.env` içindedir. Şemada bu uçların kendi sunucusu ve `internalKey` güvenlik şeması tanımlıdır; Postman'de başlığı elle eklemen gerekir. Bu anahtar yalnızca servisler arası çağrılar içindir, tarayıcıya asla verilmez.
5. Access token 15 dakika geçerlidir; dolunca `POST /api/v1/auth/refresh` ile yenile. Her refresh token **tek kullanımlıktır** (tekrar kullanılırsa tüm oturumlar kapanır).

Swagger UI ile bakmak istersen (tarayıcıdan "Try it out" için CORS izni gerekir):

```bash
docker run --rm -p 8090:8080 -v "$PWD/docs/openapi:/usr/share/nginx/html/specs" \
  -e URLS='[{"url":"/specs/identity.yaml","name":"Identity"},{"url":"/specs/catalog.yaml","name":"Catalog"},{"url":"/specs/inventory.yaml","name":"Inventory"},{"url":"/specs/payment.yaml","name":"Payment"},{"url":"/specs/order.yaml","name":"Order"}]' \
  swaggerapi/swagger-ui
```

Servisler varsayılan olarak yalnızca `http://localhost:5173` kaynağına CORS izni verir. Swagger UI'ın (`http://localhost:8090`) istek atabilmesi için `backend/.env` içinde `ALLOWED_ORIGINS=http://localhost:5173,http://localhost:8090` yapıp servisleri yeniden başlat (`make up`). Postman gibi masaüstü istemcilerde buna gerek yoktur.

## Bilinmesi gerekenler

- **Cevap zarfı:** başarı `{"success": true, "data": ...}`, hata `{"success": false, "message": "...", "fields": {...}}`. `fields` yalnızca 422'de gelir ve `variants[0].price_cents` gibi alan yolları içerir.
- **Para** kuruş cinsinden tam sayıdır: `459900` = 4.599,00 TL.
- **Limitler:** `/auth/*` için IP başına dakikada 20 istek (aşılınca 429). Gateway IP başına saniyede 30 istek (burst 60) izin verir; aşılırsa JSON zarflı **429** ve `Retry-After: 1` döner (ayrıca `/auth/*` kendi 429'unu servisten alır). Betikle çok hızlı istek atarsan bunu görürsün.
- `/healthz` ve `/readyz` gateway'den geçmez; servis portlarındadır (Identity 8081, Catalog 8082, Inventory 8083, Payment 8085, Order 8086).
- Stok sorgusu (`GET /stock`) `Cache-Control: no-store` döner: stok saniyeler içinde değişir, önbellekteki cevap olmayan stoğu vaat ederdi.
- Public katalog uçları `Cache-Control: public, max-age=60` döner; arka arkaya aynı isteği atıp eski veri görürsen istemci önbelleğindendir.

## Şemaların doğruluğunu kontrol etmek

```bash
python3 -m venv .venv && .venv/bin/pip install openapi-spec-validator jsonschema pyyaml requests
.venv/bin/openapi-spec-validator docs/openapi/*.yaml                                      # biçim geçerli mi
.venv/bin/python docs/openapi/check_contract.py                                          # API şemaya uyuyor mu
```

`check_contract.py` çalışan yığına gerçek istekler atar. Her yanıt için durum kodunun şemada belgelenmiş olduğunu, gövdenin şemaya uyduğunu ve **belgelenmemiş alan** bulunmadığını doğrular; spec içindeki tüm örnekleri kendi şemalarına karşı sınar; her korumalı uç için tokensız (401) ve customer token'lı (403) çağrı yapar. Kendi oluşturduğu test kayıtlarını (`speccheck...` ürün/kullanıcılar, `SPC-...` stok kalemleri) betik çökse bile sonda siler.

Catalog'un, Inventory'nin ve Payment'ın dahili uçları için çalışan **konteynerlerin başlatıldığı** anahtar gerekir. Order bölümü gerçek bir akış çalıştırır (sepet, checkout, ödeme sonucunu bekleme) ve bunun için Catalog, Inventory, Payment ve Order servislerinin hepsi çalışıyor olmalıdır; bu bölümün yarattığı her şeyi (kullanıcının siparişleri, sepeti, ödemeleri, rezervasyonları) sonunda siler. Payment bölümü, Order bölümündeki gerçek siparişlerin (bir ödenmiş, bir reddedilmiş) ödemelerini okur ve çalışan `payment` servisinin işlemesini bekler; bu yüzden yığın ve `payment` ayakta olmalı. Genelde `backend/.env` içindeki değerdir; konteyner farklı bir anahtarla başlatıldıysa (ör. kabuk ortamından) aynı değeri `INTERNAL_API_KEY` ortam değişkeniyle ver, ya da `docker compose up -d inventory` ile servisi `backend/.env` değeriyle yeniden oluştur. Hem spec hem API değiştikçe çalıştırmak, ikisinin birbirinden sessizce uzaklaşmasını önler. Kimlik uçlarından yaklaşık 12 istek harcadığı için bir dakika içinde iki kez çalıştırma.
