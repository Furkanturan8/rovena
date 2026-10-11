# 15 - Uçtan uca testler ve hata enjeksiyonu

Şimdiye kadarki testler tek bir servisi (ya da bir servis + veritabanı + RabbitMQ'yu) sınadı. Hiçbiri "beş servis birlikte çalışırken biri düşerse ne olur?" sorusunu cevaplamadı. Bu bölümdeki paket (`backend/e2e/run.py`) tam bunu yapar: gerçek sipariş verir, gerçek konteynerleri durdurup başlatır ve sonunda para, stok ve sipariş durumunun tutarlı kaldığını denetler.

```bash
make e2e                           # tüm senaryolar, yaklaşık 3 dakika
make e2e SCENARIO="rabbitmq inventory"   # adında bu kelimeler geçenler
```

## Nasıl çalışır?

- `make e2e` Order servisini `backend/docker-compose.e2e.yml` ile **kısa kurtarma süreleriyle** başlatır (ödeme zaman aşımı 2 dakika yerine 15 saniye, kurtarma aralığı 3 saniye) ve bitince normal ayarlara geri döner. Süreler kısalmasaydı bazı senaryolar dakikalar sürerdi.
- Her senaryo kendi ürününü ve stoğunu açar; böylece senaryolar birbirinin sayılarını bozmaz.
- Paket önce gateway üzerinden yeni bir kullanıcı ve admin ile giriş yapar, ardından **yalnızca gerçek API'leri** kullanır. Veritabanına yalnızca sayıları doğrulamak için bakar.
- Bitince (hata alsa bile) durdurduğu her konteyneri geri başlatır ve oluşturduğu her şeyi (kullanıcı, ürün, stok, sipariş, ödeme, rezervasyon) siler.

## Senaryolar

| Senaryo | Ne yapar | Neyi kanıtlar |
| --- | --- | --- |
| `happy_path` | 2 adet sipariş, kart onaylı | Durum geçmişi `pending → awaiting_payment → paid → confirmed`; stok kalıcı olarak 2 düştü; tek ödeme |
| `insufficient_stock` | Stoktan fazlasını ister | `cancelled / out_of_stock`; stok dokunulmamış; ödeme hiç istenmemiş |
| `payment_declined` | Reddedilen kart | `cancelled / card_declined`; stok serbest kaldı; `reservation_state = released` |
| `provider_outage_ends_with_an_answer` | Sağlayıcı hiç cevap vermiyor | Üç denemeden sonra `provider_unavailable` ile iptal; sipariş sonsuza dek beklemiyor |
| `double_checkout_request` | Aynı `Idempotency-Key` ile 8 eşzamanlı istek | Tek sipariş, tek `201`, tek rezervasyon, tek ödeme, stok bir kez düştü |
| `duplicate_events_change_nothing` | Onaylı siparişe 3 kez `PaymentSucceeded`, bir çelişkili `PaymentFailed`, bir tekrar `PaymentRequested` yayınlar | Sipariş `confirmed` kalır; stok, geçmiş, ödeme sayısı ve sağlayıcı çağrı sayısı değişmez |
| `restart_order_mid_saga` | Ödeme servisi kapalıyken sipariş verir, **Order'ı yeniden başlatır**, sonra ödemeyi açar | Sipariş yine tamamlanır; tek ödeme |
| `restart_payment_while_requests_wait` | Ödeme kapalıyken 3 sipariş verir, sonra açar | RabbitMQ mesajları tuttu; üçü de onaylandı |
| `rabbitmq_outage` | **RabbitMQ'yu durdurur**, sipariş verir | Checkout broker olmadan çalışır; `PaymentRequested` outbox'ta bekler; broker dönünce yayınlanır ve sipariş tamamlanır |
| `inventory_down_at_checkout` | Inventory kapalıyken sipariş verir | Sipariş `pending` kalır, istek hata vermez; Inventory dönünce kurtarma işi tamamlar |
| `inventory_down_when_payment_arrives` | Ödeme başarılı, ama stok servisi kapalı | Sipariş onaylanmaz (`paid`'de bekler); Inventory dönünce commit tamamlanır |
| `lost_payment_request_is_asked_again` | `PaymentRequested` mesajını kuyruktan siler | Kurtarma işi ödemeyi yeniden ister; yine tek ödeme |
| `silent_payment_cancels_and_late_success_is_flagged` | Ödeme servisi hiç cevap vermez | 3 istekten sonra `payment_timeout` ile iptal, stok bırakılır. Ödeme servisi sonra açılınca **geç gelen başarı** siparişi canlandırmaz ve `REFUND NEEDED` loglanır |

## Bulgular

İlk çalıştırmada 13 senaryonun hepsi geçti; Faz 1–3'te yazılan koruma katmanları (outbox, korumalı geçişler, kurtarma işi, idempotent Inventory çağrıları) bu hata türlerini zaten karşılıyormuş. Senaryoların boş yere geçmediğinden emin olmak için testler somut durumları denetler (geçmiş listesinin tam hali, `payment_requests >= 2`, log satırının varlığı, stok çiftinin tam değeri) ve zamanlamaya bağlı senaryolar kısa süreleri gerçekten kullanır.

Bilinen ve **bilerek sınanan** eksik: son senaryo, para alınıp siparişin iptal kaldığı durumu gösterir. İade akışı yok; bu Faz 6'da.

## Bu paket neyi kapsamıyor?

- Postgres'in düşmesi (servisler `/readyz` ile bunu bildirir, ama senaryosu yok).
- Gateway arkasındaki hız sınırlayıcı (ayrı testlerde var).
- Yük ve performans.
- Aynı anda birden çok Order kopyası (kurtarma işi tek kopya varsayar).
