# Rovena Saga Lab

Rovena'nın **gerçek** checkout Saga'sını (Order = orchestrator, Inventory REST, Payment RabbitMQ, Transactional Outbox, kurtarma işi) animasyonlar, tahmin et-sonra-gör hata senaryoları ve testlerle öğreten interaktif laboratuvar. Derleme adımı ve bağımlılık yok; çalışan bir backend gerekmez.

## Çalıştırma

`../observability-lab/` altındaki ortak dosyaları kullandığı için sunucuyu `frontend/` kökünden başlat:

```bash
cd frontend
python3 -m http.server 5180
# http://localhost:5180/saga-lab/
```

Sekmeye doğrudan gitmek için: `#flow`, `#failure`, `#arch`, `#code`, `#testing`, `#obs`.

## Sekmeler

| Sekme | İçerik |
| --- | --- |
| Saga Flow | Saga'nın çözdüğü problem, Rovena'daki adım/telafi tablosu, 12 kavram, başarılı akışın animasyonu (transaction sınırları, canlı DB değerleri, mesaj içerikleri, ilgili kod) |
| Failure Lab | A ortadaki adım, B kayıp/geciken mesaj, C duplicate teslim, D telafi başarısız, E dual write / outbox. Önce tahmin, sonra animasyon; her senaryoda varyantlar |
| Architecture Lab | Aynı iş: choreography (varsayımsal) ve orchestration (Rovena) animasyonları, karşılaştırma, karar yardımcısı, Rovena değerlendirmesi |
| Code Lab | 15 gerçek kod parçası + 1 öneri (Saga metrikleri); her biri için ne yapar, ne zaman çalışır, başarısız olursa, tekrar gelirse, yeniden başlatınca |
| Testing Lab | 10 test senaryosu, beklenen ve gerçek sonuç, 5 invariant, bu davranışı doğrulayan gerçek Rovena testleri ve son çalıştırma sonuçları |
| Observability Lab | Saga metrik haritası (var olan / eksik), trace context'in mesaj sınırlarını geçişi, 7 soruluk iki hata ayıklama senaryosu (iade takıldı, ödeme bekleyen Saga'lar birikiyor) |

## Gerçek, simüle, varsayımsal

- **Gerçek:** durum modeli (`orders.status`, `reservation_state`, `refund_state`), olay/komut adları, kuyruklar, kod parçaları (kısaltılmış), metrikler, uyarılar, test adları.
- **Simüle:** `js/sim.js` Rovena'nın kod yollarını deterministik olarak modeller; animasyon ve test sonuçları bu modelden gelir. Hiçbir broker'a veya veritabanına bağlanmaz.
- **Varsayımsal (etiketli):** choreography akışı ve "outbox olmadan dual write" varyantı. Rovena'da yoklar.
- **Türetilmiş:** Pending / Running / Completed / Failed / Compensating / Compensated / Requires manual intervention etiketleri. Rovena'da ayrı bir saga durumu kolonu yok; bu etiketler üç durum alanından türetilir.

## Yapı

```
index.html
css/saga.css         (temel tema: ../observability-lab/css/lab.css)
js/sim.js            Rovena Saga simülatörü + invariant'lar
js/player.js         diyagram, transaction çerçeveleri, paket animasyonu, zaman çizelgesi
js/flow.js           Saga Flow
js/failure.js        Failure Lab
js/arch.js           Architecture Lab
js/code.js           Code Lab (kod kütüphanesi)
js/testing.js        Testing Lab
js/obs.js            Observability Lab (senaryo motoru: ../observability-lab/js/lab.js)
js/app.js            sekmeler
```

Ortak dosyalar: `../observability-lab/js/{util,tools,lab}.js`. Simülatörü Node'da da çalıştırabilirsin:

```bash
node -e 'globalThis.RL={}; require("./js/sim.js"); console.log(RL.saga.simulate({commit:"gone"}).state)'
```
