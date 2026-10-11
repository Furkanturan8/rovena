# Rovena Öğrenme Notları

Bu notlar, mikroservis mimarisini hiç bilmeyen biri için yazılmış, projeyle birlikte büyüyen bir rehberdir. Her bölüm "kod ne yapıyor" ve "neden böyle yapıldı" sorularını yanıtlar. Proje ilerledikçe yeni bölümler eklenir, mevcutlar güncellenir.

## Bölümler

| # | Bölüm | Ne öğrenirsin |
| --- | --- | --- |
| 01 | [Mikroservis nedir?](01-mikroservis-nedir.md) | Monolit ile farkı, neden ve ne zaman mikroservis, bu projedeki servisler |
| 02 | [Proje haritası](02-proje-haritasi.md) | Kök dizindeki her klasör ve dosya ne işe yarıyor |
| 03 | [Altyapı](03-altyapi.md) | Docker Compose, PostgreSQL, RabbitMQ, Nginx gateway, `.env` |
| 04 | [Shared modülü](04-shared-modulu.md) | Servislerin ortak kullandığı kod ve neden ayrı modül |
| 05 | [Identity servisi: katmanlar](05-identity-katmanlar.md) | Bir servisin içi: domain, application, transport, repository, infrastructure |
| 06 | [Kimlik doğrulama akışı](06-kimlik-dogrulama.md) | JWT, refresh token, rotasyon, roller, diğer servislerin token'ı doğrulaması |
| 07 | [Bir isteğin yolculuğu](07-istegin-yolculugu.md) | Tarayıcıdan veritabanına ve geri, adım adım |
| 08 | [Çalıştırma ve test](08-calistirma-ve-test.md) | Projeyi ayağa kaldırma, API deneme, testlerin mantığı |
| 09 | [Catalog servisi](09-catalog-servisi.md) | Ürün/varyant modeli, arama ve filtreleme, arşivleme, servis sınırı |
| 10 | [API sözleşmesi: OpenAPI](10-openapi.md) | OpenAPI dosyaları, nasıl okunur, sözleşme testi |
| 11 | [Inventory servisi](11-inventory-servisi.md) | Stok, rezervasyon yaşam döngüsü, overselling'i engelleme, koşullu güncelleme ve kilitler |
| 12 | [Mesajlaşma](12-mesajlasma.md) | Olay zarfı, RabbitMQ topolojisi (retry, DLQ), Transactional Outbox, idempotent tüketici |
| 13 | [Payment servisi](13-payment-servisi.md) | Ödeme durumları, idempotency anahtarı, sağlayıcı arayüzü ve sahte sağlayıcı, sağlayıcı cevap vermezse ne olur, tek ödeme/sipariş |
| 14 | [Order servisi ve checkout Saga](14-order-servisi-ve-saga.md) | Sepet, fiyatı sunucuda belirleme, checkout idempotency, Saga adımları ve telafi, korumalı geçişler, takılan siparişi kurtarma |
| 99 | [Sözlük](99-sozluk.md) | Terimlerin kısa açıklamaları |

## Nasıl okunmalı?

Sırayla 01'den başla. Kod örneklerini görmek için ilgili dosyayı yanında aç. Dosya yolları proje köküne göre yazılmıştır.

Sıradaki işlerin planı: [`docs/todo.md`](../todo.md).

## İlerleme günlüğü

| Aşama | Durum | Not |
| --- | --- | --- |
| Altyapı (Compose, Postgres, RabbitMQ, Nginx) | Tamamlandı | RabbitMQ'yu Payment ve Order servisleri kullanıyor |
| Shared modülü | Tamamlandı | errorx, response, httpx, auth, logging, messaging, outbox, events |
| Identity servisi | Tamamlandı | Kayıt, giriş, refresh, profil, admin rol yönetimi; birim, HTTP ve Postgres entegrasyon testleri |
| Catalog servisi | Tamamlandı | Kategori, ürün, varyant, görsel; arama/filtre; admin yönetimi; aynı test seti |
| API şemaları (OpenAPI) | Tamamlandı | Identity, Catalog ve Inventory; sözleşme testi |
| Inventory servisi | Tamamlandı | Varyant bazlı stok, rezervasyon yaşam döngüsü, süre aşımı, hareket defteri; birim, HTTP ve eşzamanlılık testleri |
| Mesajlaşma altyapısı (Faz 1) | Tamamlandı | Olay zarfı, RabbitMQ topolojisi (retry, DLQ), Transactional Outbox, idempotent Inbox; gerçek RabbitMQ ve Postgres ile entegrasyon testleri, mutasyon kontrolü |
| Payment servisi (Faz 2) | Tamamlandı | `PaymentRequested` tüketir, `PaymentSucceeded/Failed` yayınlar; idempotency anahtarı, sahte sağlayıcı, okuma uçları; birim, HTTP, Postgres ve RabbitMQ entegrasyon testleri, mutasyon kontrolü |
| Order servisi ve checkout Saga (Faz 3) | Tamamlandı | Sepet, checkout (Idempotency-Key), Catalog/Inventory REST + Payment olayları, korumalı durum geçişleri, kurtarma işi; birim, HTTP, Postgres entegrasyon testleri, mutasyon kontrolü, gerçek yığında uçtan uca deneme |
| Frontend | Planlı | Önce backend |
