# Rovena Observability Lab

Prometheus, Grafana, Jaeger ve RabbitMQ'yu Rovena'nın gerçek akışları üzerinden öğreten, bağımlılıksız (vanilla HTML/CSS/JS, derleme adımı yok) interaktif laboratuvar. Çalışan bir backend gerektirmez.

## Çalıştırma

```bash
cd frontend/observability-lab
python3 -m http.server 5180
# http://localhost:5180
```

`index.html` doğrudan çift tıklayarak da açılır (ES modülü kullanılmaz). Doğrudan bir moda gitmek için: `#flow`, `#lab`, `#guide`.

## Modlar

| Mod | İçerik |
| --- | --- |
| **Mod 1 · Sistem nasıl çalışıyor?** | Checkout'un yolculuğu: HTTP, outbox, RabbitMQ, consumer, metrik (pull), trace (push), Grafana. Başlat/duraklat, ileri/geri, hız, yeniden başlat, bileşene tıklayıp açıklama. Üç varyant: mutlu yol, hata + retry + DLQ, kuyruk birikmesi. |
| **Mod 2 · Sorun nasıl tespit edilir?** | 5 senaryo, her biri Durum → Belirti → İlk kontrol → İnceleme → Teşhis → Aksiyon → Doğrulama. Cevap hemen verilmez; aracı sen seçer, paneli/sorguyu/span'i sen bulursun. |
| **Araç rehberi & PromQL** | Dört aracın farkı, Grafana kavramları, `/targets`, counter/gauge/histogram, `rate`/`increase`/`histogram_quantile`, 11 sorguluk katalog, Jaeger ve RabbitMQ UI gezintisi, projede olmayan şeyler. |

## Senaryolar

| Kod | Senaryo | Nasıl başlatılır |
| --- | --- | --- |
| A | API isteği neden yavaşladı? | Mod 2 ▸ soldan **A** (konsol: `RL.startScenario('A')`) |
| B | RabbitMQ kuyruğu büyüyor | Mod 2 ▸ **B** |
| C | Bir mesaj işlenirken hata oluşuyor | Mod 2 ▸ **C** |
| D | Hata oranı aniden yükseldi | Mod 2 ▸ **D** |
| E | Servis çalışıyor ama işler tamamlanmıyor | Mod 2 ▸ **E** |

İlerleme tarayıcıda (`localStorage`) tutulur; "İlerlemeyi sıfırla" ile silinir.

## Gerçek ve simüle veri

- **Gerçek:** mimari, metrik adları, PromQL sorguları, kuyruk/exchange/routing key adları, uyarı kuralları, arayüz adresleri. Kaynak: `backend/docker-compose.yml`, `backend/infrastructure/{prometheus,grafana,nginx}`, `backend/shared/{messaging,outbox,telemetry}`.
- **Simüle:** tüm sayılar ve grafikler. Her araç penceresinde "SİMÜLE VERİ" rozeti var. Veri `RL.gen` ile deterministik üretilir (her açılışta aynı).

Backend'e hiçbir değişiklik yapılmadı.

## Yapı

```
index.html
css/lab.css            koyu tema
js/util.js             DOM yardımcıları, deterministik seri üreteci, çizgi grafik (SVG)
js/tools.js            Grafana / Prometheus / Jaeger / RabbitMQ / terminal pencereleri (spec alıp çizer)
js/architecture.js     Mod 1: düğümler, kenarlar, adımlar, bileşen rehberi (gerçek mimari)
js/flow.js             Mod 1: animasyon motoru ve kontroller
js/lab.js              Mod 2: senaryo motoru (aşama, seçim, görev)
js/scenarios.js        Senaryo A, B + ortak veri yardımcıları
js/scenarios2.js       Senaryo C, D, E
js/guide.js            Araç rehberi, metrik kataloğu, PromQL oyun alanı
js/app.js              Mod geçişi
```

## Gerçek metriklere bağlamak

`js/tools.js` pencereleri bir **spec** (veri) alır; spec'i bugün `js/scenarios*.js` üretir. Gerçek Prometheus'a geçmek için aynı şekli (`{name, data[]}` serileri) `GET /api/v1/query_range` sonucundan üretmek yeterlidir; çizim kodu değişmez.
