# 18 - Gözlemlenebilirlik laboratuvarı

[16](16-gozlemlenebilirlik-ve-ci.md) araçları anlattı. Bu bölüm onları **kullanmayı** öğretir: `frontend/observability-lab/` altında çalışan bir backend gerektirmeyen interaktif bir laboratuvar. Başlatma: `cd frontend/observability-lab && python3 -m http.server 5180`.

## Üç mod

1. **Sistem nasıl çalışıyor?** Bir checkout'un HTTP isteğinden RabbitMQ'ya, consumer'a, metriğe ve trace'e yolculuğu; adım adım, kontrol edilebilir animasyon. Üç varyant: mutlu yol, retry + DLQ, kuyruk birikmesi.
2. **Sorun nasıl tespit edilir?** Beş senaryo (yavaş API, büyüyen kuyruk, hatalı mesaj, hata oranı sıçraması, "HTTP yeşil ama iş bitmiyor"). Her biri belirtiden doğrulamaya yedi aşama; cevap önce sana bırakılır.
3. **Araç rehberi.** Grafana kavramları, Prometheus `/targets`, counter/gauge/histogram, PromQL kataloğu, Jaeger ve RabbitMQ UI gezintisi.

## Dört aracın farkı (kısa)

| Araç | Soru | Veri |
| --- | --- | --- |
| Prometheus | Ne kadar, ne zaman, nerede? | Sayısal zaman serileri; servislerden **çeker** (pull) |
| Grafana | Bunu nasıl gösteririm? | Hiçbiri; Prometheus/Jaeger'a sorar |
| Jaeger | Bu tek istekte süre nereye gitti? | Span ağaçları; servisler OTLP ile **iter** (push) |
| RabbitMQ | Mesaj şu an nerede, kim tüketiyor? | Mesajların kendisi + broker sayaçları |

## Önemli dersler

- **HTTP başarısı asenkron işin başarısı değildir.** Checkout `201` döner; ödeme ve onay sonra gelir (Senaryo E).
- **Boş kuyruk sağlıklı demek değildir.** Mesaj hiç gelmiyor olabilir; outbox metrikleri ve bağlantılar da bakılır.
- **Prefetch=1** olduğundan `Unacked` en çok consumer sayısı kadardır; birikme aritmetiği: eğim = giriş − çıkış, çıkış = consumer × 1/handler süresi.
- **Geçici hata:** aynı mesajın denemeleri arasında sonuç değişir. **Kalıcı hata:** her denemede aynı hata, mesaj DLQ'ya düşer (`x-error`).
- **Metrik "kaç"ı, trace/log "neden"i söyler.** Yüksek kardinaliteli etiket (event_id) metrikte olmaz.

## Projede olmayanlar (laboratuvarda varmış gibi gösterilmedi)

- Alertmanager yok; uyarılar yalnızca görünür.
- Consumer sayısı metriği Prometheus'a çekilmiyor (`queue_coarse_metrics` ailesi). Eklemek için `family: [queue_coarse_metrics, queue_consumer_count]`; uygulanmadı.
- Gerçek frontend yok; laboratuvar bağımsızdır ve `frontend/` altındaki ilk uygulamadır.

## Gerçek ve simüle

Mimari, metrik adları, sorgular ve kuyruk adları gerçek; grafik ve sayılar simüle. Her araç penceresinde "SİMÜLE VERİ" rozeti var. Ayrıntı ve dosya haritası: `frontend/observability-lab/README.md`.
