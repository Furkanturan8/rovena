# Rovena: Backend Yol Haritası

İşaretler: `[x]` bitti, `[ ]` yapılacak. Bir faz bitince ilgili kutuları işaretle, `docs/notes/00-icindekiler.md` ilerleme günlüğünü güncelle.

## Faz 0: Temel servisler (bitti)

- [x] Altyapı: Compose, Postgres (servis başına DB), RabbitMQ, Nginx gateway
- [x] `shared/` modülü (errorx, response, httpx, auth, logging)
- [x] Identity, Catalog, Inventory servisleri (birim, HTTP ve gerçek Postgres entegrasyon testleriyle)
- [x] OpenAPI şemaları (3 servis) ve sözleşme testi
- [x] Hibrit checkout kararı: stok rezervasyonu REST, ödeme RabbitMQ (`architecture.md` bölüm 7)

## Faz 1: Mesajlaşma altyapısı (`shared/`)

- [ ] Olay zarfı: `event_id`, tip, sürüm, zaman, `correlation_id` (+ request id taşıma)
- [ ] RabbitMQ bağlantısı ve topoloji: exchange, kuyruklar, retry (gecikmeli) kuyruğu, dead-letter kuyruğu
- [ ] Transactional Outbox: outbox tablosu şablonu + yayıncı (yayınlanmamışları gönderir, işaretler)
- [ ] Idempotent tüketici: `processed_events` kaydı, tekrar teslimde tek etki
- [ ] Testler: gerçek RabbitMQ ile (yayın, tekrar teslim, zehirli mesaj DLQ'ya düşer, yayıncı çökmesi)
- [ ] Not bölümü: mesajlaşma, outbox, idempotent tüketici

## Faz 2: Payment servisi

- [ ] Alan modeli: ödeme denemesi ve durumları; `PaymentRequested` tüketir, `PaymentSucceeded/Failed` yayınlar (outbox)
- [ ] Sahte sağlayıcı arayüzü: tutar/test kartına göre başarı, hata, gecikme (gerçek sağlayıcı ileride takılabilir)
- [ ] Idempotency anahtarı: aynı istek iki kez gelirse tek ödeme
- [ ] Okuma uçları (sipariş/ödeme durumu), admin listesi; OpenAPI + sözleşme testi
- [ ] Testler (birim, HTTP, entegrasyon) + mutasyon kontrolü + not bölümü

## Faz 3: Order servisi ve checkout Saga

- [ ] Sepet: ekle/çıkar/güncelle; fiyat ve ad **Catalog'dan sunucuda** alınır (istemci fiyatına güvenilmez)
- [ ] Checkout: idempotency anahtarı, bekleyen sipariş + ürün/fiyat snapshot'ı
- [ ] Saga: Inventory'de rezerve et (REST) → `PaymentRequested` (outbox) → sonuca göre commit/release → onay/iptal
- [ ] Sipariş durum geçmişi; müşteri uçları (kendi siparişleri) ve admin uçları
- [ ] Kurtarma işi: takılı kalan siparişi bul, eksik adımı tekrar dene ya da iptal edip rezervasyonu bırak
- [ ] Gateway'e Order/Payment rotaları; OpenAPI + sözleşme testi
- [ ] Testler (birim, HTTP, entegrasyon, eşzamanlılık) + mutasyon kontrolü + not bölümü

## Faz 4: Uçtan uca dayanıklılık

- [ ] E2E senaryolar: başarılı sipariş, yetersiz stok, ödeme hatası, çift checkout isteği, tekrarlanan olay
- [ ] Hata enjeksiyonu: servis yeniden başlatma, RabbitMQ kesintisi, yarıda kalan Saga'nın toparlanması
- [ ] Tek `make` komutuyla çalışan E2E paketi; sonuçları nota işle

## Faz 5: Gözlemlenebilirlik ve CI

- [ ] OpenTelemetry: HTTP ve mesaj başlıklarında trace taşıma, temel metrikler (gecikme, hata, kuyruk birikimi, yeniden deneme)
- [ ] CI (GitHub Actions): build, vet, test; entegrasyon testleri Testcontainers ile kendi kendine yeten hale
- [ ] Linter (`golangci-lint`) ve `go.mod` bağımlılık kontrolü

## Faz 6: Cilalama ve borçlar

- [ ] Refresh token için kısa "grace period" (eşzamanlı iki sekme oturumu kapatmasın)
- [ ] Gateway: 404 ve 502/504 için de JSON hata gövdesi
- [ ] `shared/httpx` ve `shared/auth` middleware için kendi birim testleri
- [ ] Yeni Catalog varyantı için Inventory'de stok kaydı otomatik açılsın mı? (olay ile) Karar ver
- [ ] Servisler arası JWT: HS256 yerine asimetrik imza (RS256/EdDSA) değerlendir
- [ ] Sonra: React frontend (ayrı plan)

## Her fazın bitiş ölçütü

1. `make vet`, `make test`, `make test-integration` yeşil
2. Yeni uçlar için `docs/openapi/*.yaml` ve `docs/openapi/check_contract.py` güncel
3. Önemli kurallar için mutasyon kontrolü (testler bozuk kodda kırmızıya dönüyor)
4. `docs/notes/` bölümü yazıldı, ilerleme günlüğü ve sözlük güncellendi
