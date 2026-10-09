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
make test               # shared + identity + catalog, HTTP testleri dahil (veritabanı gerekmez)
make test-integration   # gerçek Postgres'e karşı Catalog ve Identity testleri (önce make up)
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
| `identity/.../transport/http/http_test.go` | HTTP katmanı: kayıt/giriş/refresh/logout akışları, doğrulama tabloları, enumeration, mass assignment, rate limit, yetki matrisi (`alg: none` dahil), admin uçları, log hijyeni |
| `identity/.../postgres/integration_test.go` | (Gerçek Postgres, her test kendi şemasında) kullanıcı ve refresh token repository'leri, eşzamanlı kayıt/refresh, son admin yarışı |
| `catalog/.../transport/http/http_test.go` | HTTP katmanı: yetki matrisi, girdi doğrulama, hata eşleme, cache/CORS/request id, body limiti, sağlık uçları |
| `catalog/.../postgres/integration_test.go` | (Gerçek Postgres) aynı anda iki varyantın kapatılması, yayın kuralı, olmayan ürüne varyant |
| `catalog/.../postgres/search_integration_test.go` | (Gerçek Postgres) arama, filtre, sıralama, sayfalama SQL'i |

### Testlerin üç katmanı

1. **Birim:** `application` iş kuralları, sahte repository'lerle, milisaniyeler.
2. **HTTP:** Gerçek Fiber uygulaması + gerçek servisler + bellek içi repository'ler. Router, middleware sırası, JWT, doğrulama, hata eşleme birlikte test edilir. Veritabanı gerekmez.
3. **Entegrasyon:** Gerçek PostgreSQL (`-tags integration`). SQL'in, kısıtların, kilitlerin ve transaction'ların doğru çalıştığını gösterir.

Identity entegrasyon testleri her test için **geçici bir PostgreSQL şeması** (`itest_<rastgele>`) açar, migration'ları oraya uygular ve test bitince `DROP SCHEMA ... CASCADE` yapar. Böylece testler birbirinden ve geliştirme verisinden izoledir; "sistemde en az bir admin" gibi **global** kurallar, gerçek admin hesabına dokunmadan test edilebilir.

### Yarış testi yazarken dikkat

İlk yazdığımız "iki admin birbirini düşürür" testi (her turda yeni şema, yeni bağlantılar) kusurlu kodda **geçti**: yeni açılan bağlantılar yavaş olduğu için zamanlama yarışı gizledi. Bağlantı havuzu ısıtılıp aynı şemada 300 tur çalıştırılınca yarış ilk turda yakalandı. Ders: bir yarış testinin geçmesi yarışın olmadığını göstermez; testin **kusurlu kodda kırmızıya döndüğünü** görmeden güvenme.

### Testler nasıl veritabansız çalışıyor?

`application` katmanı repository'nin **arayüzüne** bağlı olduğu için testlerde `fakes_test.go` içindeki bellek-içi sahte repository'ler kullanılır. Hızlıdır (milisaniye) ve iş kurallarını izole test eder.

### Henüz olmayanlar

- Entegrasyon testleri şu an çalışan Compose Postgres'ine bağlanıyor; CI'da Testcontainers ile kendi kendine yeten hale getirilebilir.
- Ortak kod (`shared/httpx`, `shared/auth` middleware) yalnızca servislerin HTTP testleri üzerinden dolaylı test ediliyor; kendi birim testleri yok (yalnızca `auth.Verifier`'ın var).
- Uçtan uca testler (birden fazla servis olunca).

Bu eksikler bilerek not edilmiştir; ilerleyen adımlarda eklenecek.

## Sık karşılaşılan sorunlar

| Belirti | Sebep / çözüm |
| --- | --- |
| `port is already allocated` | Başka bir program aynı portu kullanıyor. Postgres için host portu bu yüzden 5433. |
| Compose "set in .env" hatası | `.env` eksik ya da bir değişken boş. |
| Parolayı değiştirdim ama Postgres eski parolayı kullanıyor | Init betiği sadece ilk kurulumda çalışır. `docker compose down -v` ile volume'u sil. |
| `identity` sürekli yeniden başlıyor | `docker compose logs identity`: genelde `JWT_SECRET` 32 karakterden kısa veya veritabanına erişilemiyor. |
