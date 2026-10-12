# 19 - Saga laboratuvarı

[14](14-order-servisi-ve-saga.md) checkout Saga'sını kodla anlattı. Bu bölüm onu **görerek ve bozarak** öğretir: `frontend/saga-lab/` (başlatma: `cd frontend && python3 -m http.server 5180`, sonra `/saga-lab/`).

## Rovena'daki Saga, tek paragrafta

Order servisi **orchestrator**'dır. Checkout siparişi `pending` yazar, Inventory'de stoğu **senkron REST** ile rezerve eder, `awaiting_payment` durumuna geçerken `PaymentRequested` komutunu aynı transaction'da **outbox**'a yazar. Payment komutu RabbitMQ'dan alır, sağlayıcıyı **aynı idempotency key** ile çağırır, sonucu (`PaymentSucceeded/Failed`) yine outbox ile yayınlar. Order sonuca göre ya stoğu commit edip `confirmed` olur ya da **telafi** eder: stoğu bırakır (Release), parayı iade ettirir (`RefundRequested`). Cevabı gelmeyen adımları **kurtarma işi** yakalar: aynı key ile tekrar sorar, sınır dolunca iptal eder; geç gelen ödeme iade edilir.

## Laboratuvardan çıkan dersler

- **Telafi ≠ rollback.** Commit edilmiş bir adım geri alınmaz; etkisi yeni bir işlemle giderilir (iade bir para hareketi, release bir stok hareketi). Önceki durum birebir geri gelmez: sipariş `cancelled` olarak kalır.
- **Neyi telafi edeceğin adıma bağlı.** Stok yoksa telafi edilecek bir şey yok (Failed). Kart reddinde yalnızca stok. Rezervasyon kaybında hem stok hem para. Ters sıra varsayılandır ama kural değildir.
- **Timeout "başarısız" değil, "bilinmiyor" demektir.** Rovena aynı key ile yeniden sorar, sonra karar verir ve kararı geri dönüşlü tutar.
- **Idempotency üç katmanda:** iş anahtarı (`order:<id>`), işleme kaydı (`Inbox`), korumalı durum geçişi (`apply` + `FOR UPDATE`).
- **Outbox dual-write'ı çözer**, "en az bir kez"i çözmez: tüketiciler yine idempotent olmalı.
- **Telafi de başarısız olabilir.** Rovena sınırlı tekrar eder, sonra `REFUND STUCK` loglar; `refund_state = requested` kalır. Ayrı bir "manuel müdahale" durumu ve bildirim (Alertmanager) yok.

## Gözlemlenebilirlikte eksikler (öneri, uygulanmadı)

- Devam eden Saga sayısı (gauge) ve Saga tamamlanma süresi (histogram) metrikleri yok. Örnek kod: Saga Lab ▸ Code Lab ▸ "ÖNERİ".
- Sipariş/Saga id'si metrik etiketi olmaz; tekil takip `correlation_id` (= sipariş id'si), trace ve `order_status_history` ile yapılır.

## Testler

Laboratuvardaki 10 test senaryosu, simülatörün son durumunu 5 invariant'la denetler ve her biri aynı davranışı doğrulayan gerçek testlere bağlanır (`service_test.go`, `integration_test.go`, `e2e/run.py`). 2026-10-12'de `make test` ve izole entegrasyon testleri (shared messaging/outbox, payment ve order repository) geçti. E2E servisleri durdurup başlattığı için çalıştırılmadı.
