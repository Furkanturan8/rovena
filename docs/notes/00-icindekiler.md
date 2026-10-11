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
| 99 | [Sözlük](99-sozluk.md) | Terimlerin kısa açıklamaları |

## Nasıl okunmalı?

Sırayla 01'den başla. Kod örneklerini görmek için ilgili dosyayı yanında aç. Dosya yolları proje köküne göre yazılmıştır.

Sıradaki işlerin planı: [`docs/todo.md`](../todo.md).

## İlerleme günlüğü

| Aşama | Durum | Not |
| --- | --- | --- |
| Altyapı (Compose, Postgres, RabbitMQ, Nginx) | Tamamlandı | RabbitMQ henüz hiçbir servis tarafından kullanılmıyor (Faz 2'de Payment ile başlayacak) |
| Shared modülü | Tamamlandı | errorx, response, httpx, auth, logging, messaging, outbox |
| Identity servisi | Tamamlandı | Kayıt, giriş, refresh, profil, admin rol yönetimi; birim, HTTP ve Postgres entegrasyon testleri |
| Catalog servisi | Tamamlandı | Kategori, ürün, varyant, görsel; arama/filtre; admin yönetimi; aynı test seti |
| API şemaları (OpenAPI) | Tamamlandı | Identity, Catalog ve Inventory; sözleşme testi |
| Inventory servisi | Tamamlandı | Varyant bazlı stok, rezervasyon yaşam döngüsü, süre aşımı, hareket defteri; birim, HTTP ve eşzamanlılık testleri |
| Mesajlaşma altyapısı (Faz 1) | Tamamlandı | Olay zarfı, RabbitMQ topolojisi (retry, DLQ), Transactional Outbox, idempotent Inbox; gerçek RabbitMQ ve Postgres ile entegrasyon testleri, mutasyon kontrolü |
| Order servisi (Saga, Outbox) | Sırada | Rezervasyon uçları REST olarak hazır; RabbitMQ bu adımda devreye girecek |
| Payment servisi | Planlı | |
| Frontend | Planlı | Önce backend |
