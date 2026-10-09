# 08 - Çalıştırma ve test

## Ayağa kaldırma

```bash
cp .env.example .env     # değerleri doldur; JWT_SECRET için: openssl rand -base64 48
make up                  # tüm konteynerleri derler ve başlatır
make seed-admin          # ilk admin kullanıcıyı oluşturur
make seed-catalog        # demo kategori ve ürünleri yükler
docker compose ps        # durumları gör
make logs                # logları izle
make down                # durdur (veri kalır)
docker compose down -v   # durdur ve TÜM veriyi sil (sıfırdan başlamak için)
```

Başlatma sırası: `postgres` (sağlıklı olmalı) → `identity-migrate` (tabloları kurar, çıkar) → `identity` → `gateway`.

## API'yi elle deneme

Gateway `http://localhost:8080` üzerindedir.

```bash
# Kayıt
curl -s -X POST localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"jane@example.com","password":"s3cretpass1","first_name":"Jane"}'

# Giriş (cevaptaki access_token ve refresh_token'ı not et)
curl -s -X POST localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"jane@example.com","password":"s3cretpass1"}'

# Kendi profilin
curl -s localhost:8080/api/v1/users/me -H "Authorization: Bearer $ACCESS_TOKEN"

# Token yenileme
curl -s -X POST localhost:8080/api/v1/auth/refresh \
  -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}"
```

Beklenen hata durumları: aynı e-posta ile ikinci kayıt 409, yanlış parola 401, tokensız `/users/me` 401, customer ile `/users` listesi 403, kısa parola 422 (alan bazlı mesajla).

Catalog'u deneme (önce `make seed-catalog`):

```bash
curl -s 'localhost:8080/api/v1/products?q=canta'
curl -s 'localhost:8080/api/v1/products?category=giyim&color=siyah&sort=price_asc'
curl -s localhost:8080/api/v1/products/deri-ceket

# Admin işlemleri için admin hesabıyla giriş yapıp token al
curl -s -X POST localhost:8080/api/v1/admin/products \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Yeni Ürün","category_id":"<kategori-uuid>","variants":[{"sku":"NEW-1","size":"M","price_cents":10000}]}'
```

Sağlık: `curl localhost:8081/readyz` (Identity), `curl localhost:8082/readyz` (Catalog) → `{"checks":{"postgres":"ok"},"ready":true}`.

Veritabanına bakmak için (parolayı `.env` içinden al):

```bash
docker compose exec postgres psql -U identity -d identity -c 'select email, role from users;'
```

(Bu komut için `identity` kullanıcısının parolası istenebilir.)

## Testler

```bash
make test               # shared + identity + catalog (veritabanı gerekmez)
make test-integration   # gerçek Postgres'e karşı Catalog testleri (önce make up)
make vet     # statik analiz
```

### Mevcut testler

| Dosya | Neyi doğrular |
| --- | --- |
| `shared/auth/verifier_test.go` | Token doğrulama: geçerli, yanlış secret, süresi geçmiş, yanlış yayıncı, `alg: none` vb. |
| `identity/internal/application/auth_service_test.go` | Kayıt (e-posta normalizasyonu, rol), çift kayıt, çok uzun parola, giriş hataları, refresh rotasyonu, **refresh yeniden kullanımında tüm oturumların iptali**, süresi dolmuş token, logout idempotentliği |
| `identity/internal/application/user_service_test.go` | Son admin düşürülemez, başka admin varsa düşürülebilir, geçersiz rol |
| `catalog/internal/application/service_test.go` | Slug çakışması, taslak varsayılanı, yayın ve son aktif varyant kuralları, taslak/arşivin herkese açık görünmemesi |
| `catalog/internal/domain/slug_test.go` | Türkçe slug ve arama metni normalizasyonu |
| `catalog/.../postgres/integration_test.go` | (Gerçek Postgres) aynı anda iki varyantın kapatılması, yayın kuralı, olmayan ürüne varyant |

### Testler nasıl veritabansız çalışıyor?

`application` katmanı repository'nin **arayüzüne** bağlı olduğu için testlerde `fakes_test.go` içindeki bellek-içi sahte repository'ler kullanılır. Hızlıdır (milisaniye) ve iş kurallarını izole test eder.

### Henüz olmayanlar

- Gerçek Postgres'e karşı **entegrasyon testleri**: Catalog'da eşzamanlılık testleri var (`make test-integration`); eksikler Identity (unique kısıtı, eşzamanlı refresh) ve Catalog arama/filtre SQL'i, kategori ağacı. Testcontainers ile eklenebilir.
- HTTP katmanı (handler/route) testleri.
- Uçtan uca testler (birden fazla servis olunca).

Bu eksikler bilerek not edilmiştir; ilerleyen adımlarda eklenecek.

## Sık karşılaşılan sorunlar

| Belirti | Sebep / çözüm |
| --- | --- |
| `port is already allocated` | Başka bir program aynı portu kullanıyor. Postgres için host portu bu yüzden 5433. |
| Compose "set in .env" hatası | `.env` eksik ya da bir değişken boş. |
| Parolayı değiştirdim ama Postgres eski parolayı kullanıyor | Init betiği sadece ilk kurulumda çalışır. `docker compose down -v` ile volume'u sil. |
| `identity` sürekli yeniden başlıyor | `docker compose logs identity`: genelde `JWT_SECRET` 32 karakterden kısa veya veritabanına erişilemiyor. |
