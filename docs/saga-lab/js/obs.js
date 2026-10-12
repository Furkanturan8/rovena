/*
 * Observability Lab: Saga'yı Prometheus/Grafana/Jaeger/RabbitMQ ile incelemek.
 * Senaryo motoru gözlem laboratuvarından (../observability-lab/js/lab.js) yeniden kullanılır; araç pencereleri SİMÜLE veridir,
 * metrik/kuyruk/tablo adları GERÇEK.
 */
(function () {
  const { h } = RL;
  const N = RL.N, sh = RL.shift, T0 = 0.9;
  const g = (fn, noise = 0, seed = 1) => RL.gen(N, fn, noise, seed);
  const flat = (v, n = 0, seed = 1) => g(() => v, n, seed);
  const S = (name, data, color) => ({ name, data, color });
  const gf = (rows) => ({ tool: 'grafana', dashboard: 'Rovena overview', url: 'localhost:3000/d/rovena-overview/rovena-overview', range: 360, rows });

  const METRICS = [
    ['Başarılı / başarısız Saga', 'orders_finished_total{status, code}', 'counter', true, 'sum by (status, code) (increase(orders_finished_total[1h]))'],
    ['Devam eden Saga sayısı', '— (öneri: saga_checkout_in_progress gauge)', 'gauge', false, 'sum by (status) (saga_checkout_in_progress)'],
    ['Saga tamamlanma süresi', '— (öneri: saga_checkout_duration_seconds histogram)', 'histogram', false, 'histogram_quantile(0.95, sum by (le, outcome) (rate(saga_checkout_duration_seconds_bucket[15m])))'],
    ['Compensation sayısı', 'orders_refunds_requested_total{reason} · orders_finished_total{status="cancelled"}', 'counter', true, 'sum by (reason) (increase(orders_refunds_requested_total[1h]))'],
    ['Compensation başarısızlığı', 'saga_recovery_actions_total{action="refund_requested_again"} · RefundNotAnswered', 'counter', true, 'increase(saga_recovery_actions_total{action="refund_requested_again"}[15m])'],
    ['Retry sayısı', 'messaging_events_consumed_total{result="retried"}', 'counter', true, 'sum by (queue) (rate(messaging_events_consumed_total{result="retried"}[5m]))'],
    ['Timeout sayısı', 'saga_recovery_actions_total{action=~"payment_requested_again|payment_timeout"}', 'counter', true, 'sum by (action) (increase(saga_recovery_actions_total[15m]))'],
    ['Consumer hata oranı', 'messaging_events_consumed_total{result}', 'counter', true, 'sum by (queue) (rate(messaging_events_consumed_total{result!="ok"}[5m])) / sum by (queue) (rate(messaging_events_consumed_total[5m]))'],
    ['Kuyruk derinliği', 'rabbitmq_detailed_queue_messages_ready{queue}', 'gauge', true, 'rabbitmq_detailed_queue_messages_ready{queue=~"payment.requests|order.payment-results|.*\\\\.dlq"}'],
  ];

  /* ---------------- Senaryo S1: iade takıldı ---------------- */
  const ORDER = '0192f3a7-5c1e-7d2a-9b8e-4f6a1c2d3e4f';
  const adminView = { tool: 'logs', command: `curl -s -H "Authorization: Bearer $ADMIN_TOKEN" localhost:8080/api/v1/admin/orders/${ORDER} | jq '.data'`, lines: [
    '{', `  "id": "${ORDER}",`, '  "status": "cancelled",', '  "failure_code": "reservation_lost",', '  "reservation_state": "released",', '  "refund_state": "requested",', '  "refund_requests": 5,', '  "payment_requests": 1,', '  "updated_at": "2026-10-09T14:22:41Z"', '}'],
    note: 'Yanıt SİMÜLE ve kısaltılmış; admin uçları siparişin defter alanlarını gösterir (TestAdmin_SeesEveryOrderWithBookkeeping).' };
  const historyView = { tool: 'logs', command: `docker compose exec postgres psql -U orders orders -c "SELECT from_status, to_status, reason, created_at FROM order_status_history WHERE order_id='${ORDER}' ORDER BY id"`, lines: [
    ' from_status      | to_status        | reason                            | created_at', '------------------+------------------+-----------------------------------+--------------------', '                  | pending          | checkout                          | 2026-10-09 14:01:02',
    ' pending          | awaiting_payment | stock reserved, payment requested | 2026-10-09 14:01:02', ' awaiting_payment | paid             | payment succeeded                 | 2026-10-09 14:16:40', ' paid             | cancelled        | reservation_lost                  | 2026-10-09 14:16:40', ' cancelled        | cancelled        | stock released                    | 2026-10-09 14:16:40'],
    note: 'Tablo ve sütun adları gerçek (order_status_history); satırlar SİMÜLE.' };
  const dlqMsg = (n) => `Exchange: (AMQP default)  Routing key: payment.requests.dlq\nHeaders:\n  x-retry-count: 5\n  x-error: refund payment: provider unavailable: context deadline exceeded\n  x-failed-at: 2026-10-0${n}T1${n}:0${n}:12Z\n  traceparent: 00-3c9e1a…-…-01\nPayload: {"type":"RefundRequested","correlation_id":"${ORDER}","payload":{"order_id":"${ORDER}","reason":"${n === 9 ? 'reservation_lost' : 'no_answer'}"}}`;
  const rabbitS1 = { tool: 'rabbit', url: 'localhost:15672/#/queues', tab: 'queues',
    queues: [
      { name: 'order.payment-results', state: 'idle', ready: 0, unacked: 0, incoming: 0.6, deliver: 0.6, ack: 0.6, consumers: 1 },
      { name: 'payment.requests', state: 'running', ready: 0, unacked: 0, incoming: 0.7, deliver: 0.7, ack: 0.7, consumers: 1 },
      { name: 'payment.requests.dlq', state: 'idle', ready: 5, unacked: 0, consumers: 0, hist: { ready: g((t) => Math.min(5, Math.floor(t * 6)), 0, 1), unacked: flat(0) }, consumersList: [], messages: [dlqMsg(9), dlqMsg(9), dlqMsg(9)] },
      { name: 'payment.requests.retry', state: 'idle', ready: 0, unacked: 0, consumers: 0 },
    ],
    overview: { ready: 5, unacked: 0, readyHist: flat(5), unackedHist: flat(0), publish: flat(1.3, 0.2, 1), deliver: flat(1.3, 0.2, 2), ack: flat(1.3, 0.2, 3) }, connections: [], channels: [], exchanges: [], bindings: [],
    queueHint: 'payment.requests.dlq’yu aç ve mesajları oku (Nack/requeue ile: silmeden).' };
  const refundTrace = (i, ago) => ({ id: '3c9e1a' + i + 'f0', service: 'payment', services: ['order', 'payment', 'inventory'], op: 'process payment.requests', dur: 940, ago, err: true, spans: [
    { id: `r${i}a`, svc: 'order', name: 'process order.payment-results', start: 0, dur: 64, tags: { 'messaging.queue': 'order.payment-results', correlation_id: ORDER } },
    { id: `r${i}b`, parent: `r${i}a`, svc: 'order', name: 'POST inventory', start: 4, dur: 18, err: true, tags: { 'http.status_code': 410 }, logs: ['reservation is gone'] },
    { id: `r${i}c`, parent: `r${i}a`, svc: 'order', name: 'db: UPDATE orders (cancel, released, refund requested)', start: 26, dur: 22 },
    { id: `r${i}d`, parent: `r${i}a`, svc: 'order', name: 'publish RefundRequested', start: 160, dur: 6, async: true },
    { id: `r${i}e`, parent: `r${i}d`, svc: 'payment', name: 'process payment.requests', start: 180, dur: 760, err: true, async: true, tags: { 'messaging.retry_count': i, 'event.type': 'RefundRequested' }, logs: ['refund payment: provider unavailable: context deadline exceeded'] },
  ] });
  const jaegerS1 = { tool: 'jaeger', url: 'localhost:16686/search', service: 'payment', tags: 'error=true', traces: [0, 1, 2, 3, 4].map((i) => refundTrace(i, 3 * 24 * 60 - i)) ,
    note: 'Telafi adımları trace’te görünür: cancel + release (DB span’i), publish RefundRequested, Payment’ta refund denemesi. correlation_id = sipariş id’si (span etiketi), Prometheus etiketi DEĞİL.' };
  const dashS1 = (fixed) => gf([{ title: 'Orders and payments', panels: [
    { id: 'refunds', title: 'Refunds', expr: 'sum(increase(orders_refunds_requested_total[1h]))  vs  sum(increase(payments_refunded_total[1h]))', series: [S('istenen', g((t) => 1 + Math.sin(t * 30) * 0.6, 0.2, 1), '#f2cc60'), S('yapılan', g((t) => (t < T0 ? 1 + Math.sin(t * 30) * 0.6 : fixed && t > 0.97 ? 1.6 : 0), 0.15, 2), '#7ee787')] },
    { id: 'recovery', title: 'Recovery job actions per 5 minutes', expr: 'sum by (action) (increase(saga_recovery_actions_total[5m]))', series: [S('refund_requested_again', g((t) => (t < T0 + 0.02 ? 0 : fixed && t > 0.97 ? 0 : 2), 0.3, 3), '#ff7b72'), S('payment_requested_again', flat(0.2, 0.2, 4), '#5ab0ff')] },
  ] }, { title: 'Messaging', panels: [
    { id: 'consumed', title: 'Events consumed per second, by result', expr: 'sum by (queue, result) (rate(messaging_events_consumed_total[1m]))', series: [S('payment.requests ok', g((t) => (t < T0 ? 0.7 : 0.55), 0.05, 5), '#7ee787'), S('payment.requests retried', g((t) => (t < T0 ? 0 : fixed && t > 0.97 ? 0 : 0.15), 0.03, 6), '#ffb454'), S('dead_lettered', g((t) => (t < T0 ? 0 : fixed && t > 0.97 ? 0 : 0.02), 0.005, 7), '#ff7b72')] },
    { id: 'depth', title: 'Queue depth (messages waiting)', expr: 'rabbitmq_detailed_queue_messages_ready', series: [S('payment.requests.dlq', g((t) => (fixed && t > 0.975 ? 0 : t < T0 ? 0 : Math.min(5, Math.floor((t - T0) * 60))), 0, 8), '#ff7b72')] },
  ] }]);

  /* ---------------- Senaryo S2: ödeme bekleyen Saga'lar birikiyor ---------------- */
  const rabbitS2 = { tool: 'rabbit', url: 'localhost:15672/#/queues', tab: 'queues', queues: [
    { name: 'order.payment-results', state: 'idle', ready: 0, unacked: 0, incoming: 0, deliver: 0, ack: 0, consumers: 1 },
    { name: 'payment.requests', state: 'idle', ready: 4120, unacked: 0, incoming: 4.6, deliver: 0, ack: 0, consumers: 0, hist: { ready: g((t) => (t < T0 ? 0 : (t - T0) * 41200), 5, 3), unacked: flat(0) }, consumersList: [] },
    { name: 'payment.requests.dlq', state: 'idle', ready: 0, unacked: 0, consumers: 0 }, { name: 'payment.requests.retry', state: 'idle', ready: 0, unacked: 0, consumers: 0 }],
    overview: { ready: 4120, unacked: 0, readyHist: g((t) => (t < T0 ? 0 : (t - T0) * 41200), 5, 3), unackedHist: flat(0), publish: g((t) => (t < T0 ? 3 : 4.6), 0.3, 1), deliver: g((t) => (t < T0 ? 3 : 0), 0.2, 2), ack: g((t) => (t < T0 ? 3 : 0), 0.2, 4), caption: 'Publish sürüyor, deliver/ack sıfır: tüketen yok. Publish hızı checkout hızından (3/s) BÜYÜK: kurtarma işinin tekrar istekleri ekleniyor.' },
    connections: [{ name: '172.18.0.7:50221 → 172.18.0.3:5672 (order)', user: 'rovena', state: 'running', channels: 2 }], channels: [], exchanges: [], bindings: [] };
  const dashS2 = (fixed) => gf([{ title: 'Orders and payments', panels: [
    { id: 'finished', title: 'Orders finished (last hour)', expr: 'sum by (status, code) (increase(orders_finished_total[1h]))', series: [S('confirmed', g((t) => (t < T0 ? 640 : fixed && t > 0.96 ? 900 : sh(t, T0, 640, 0, 0.03)), 15, 1), '#7ee787'), S('cancelled/payment_timeout', g((t) => (t < T0 + 0.01 ? 2 : fixed && t > 0.96 ? 60 : 180), 4, 2), '#ff7b72')] },
    { id: 'recovery', title: 'Recovery job actions per 5 minutes', expr: 'sum by (action) (increase(saga_recovery_actions_total[5m]))', series: [S('payment_requested_again', g((t) => (t < T0 + 0.006 ? 0 : fixed && t > 0.96 ? 0 : 420), 20, 3), '#f2cc60'), S('payment_timeout', g((t) => (t < T0 + 0.02 ? 0 : fixed && t > 0.96 ? 0 : 75), 6, 4), '#ff7b72')] },
    { id: 'refunds', title: 'Refunds', expr: 'sum by (reason) (increase(orders_refunds_requested_total[1h]))', series: [S('payment_timeout', g((t) => (fixed && t > 0.96 ? sh(t, 0.96, 0, 55, 0.01) : 0), 1, 5), '#c792ea')] },
  ] }, { title: 'Messaging', panels: [
    { id: 'depth', title: 'Queue depth (messages waiting)', expr: 'rabbitmq_detailed_queue_messages_ready', series: [S('payment.requests', g((t) => (t < T0 ? 0 : fixed && t > 0.96 ? Math.max(0, 4120 - (t - 0.96) * 160000) : (t - T0) * 41200), 5, 6), '#ffb454')] },
  ] }]);

  RL.scenarios = [
    { id: 'S1', title: 'İade takıldı (telafi başarısız)', tools: ['Admin API', 'Postgres', 'RabbitMQ', 'Jaeger', 'Grafana'], stages: [
      { phase: 'durum', title: 'Durum: “siparişim iptal oldu, param gelmedi”', body: `<p>Destek ekibi bir sipariş numarası iletiyor: <code>${ORDER}</code>. Müşteri 3 gün önce ödedi, sipariş iptal edildi, iade hâlâ yok.</p><p class="muted">Bu senaryoda yedi soruyu sırayla cevaplayacaksın: durum, son başarılı adım, beklenen mesaj, hatanın türü, telafi gereği, doğrulayan araç, düzeltmenin kanıtı.</p>` },
      { phase: 'belirti', title: 'Belirti: iki uyarı', body: '<p>Prometheus Alerts sayfasında iki kural kırmızı. Bildirim gelmedi: Rovena’da Alertmanager yok, uyarıları birinin açıp görmesi gerekiyor.</p>',
        view: { tool: 'prom', url: 'localhost:9090/alerts', tab: 'alerts', alerts: [{ name: 'RefundNotAnswered', state: 'firing', summary: 'Payment is not answering refund requests; customers are waiting for their money' }, { name: 'DeadLetterQueueNotEmpty', state: 'firing', summary: 'Queue payment.requests.dlq holds 5 messages nobody could process; a person must look' }, { name: 'HighServerErrorRate', state: 'inactive', summary: '' }] } },
      { phase: 'ilk', title: 'Soru 1 · Saga hangi durumda kaldı?', choice: { q: 'Tek bir siparişin Saga durumunu nereden okursun?', opts: [
        { t: 'Order’ın admin ucundan (ya da orders tablosundan): status, reservation_state, refund_state', ok: true, view: adminView, fb: 'Saga durumu Order DB’de kalıcı. cancelled + reservation=released + refund=requested + refund_requests=5 ⇒ telafi yarım kaldı ve kurtarma işi denemeyi bıraktı: <b>Requires manual intervention</b>.' },
        { t: 'Grafana’da order_id etiketiyle filtrele', ok: false, fb: 'Metriklerde sipariş/saga kimliği etiket olarak YOK ve olmamalı: her sipariş yeni bir zaman serisi demek (kardinalite patlaması). Tekil Saga için durum sorgusu, log ve trace kullanılır.' },
        { t: 'RabbitMQ’da siparişin mesajlarını ara', ok: false, fb: 'Kuyruklar veritabanı değildir: işlenmiş mesajlar silinir. Durum orders satırında.' },
      ] } },
      { phase: 'inceleme', title: 'Soru 2 · Son başarılı adım hangisi?', view: historyView, choice: { q: 'order_status_history’ye göre Saga nerede durdu?', opts: [
        { t: 'Ödeme alındı, stok kesinleştirilemedi, sipariş iptal edildi, stok kaydı kapatıldı; iade istendi ama tamamlanmadı', ok: true, fb: 'Son başarılı telafi: “stock released”. Bekleyen telafi: para iadesi (refund_state = requested).' },
        { t: 'Ödeme hiç alınmadı', ok: false, fb: '“awaiting_payment → paid: payment succeeded” satırı var: para çekildi.' },
        { t: 'Sipariş onaylandı ve sonra iptal edildi', ok: false, fb: '“confirmed” satırı yok; paid’den doğrudan reservation_lost ile cancelled’a geçmiş.' },
      ] } },
      { phase: 'inceleme', title: 'Soru 3 · Hangi mesaj bekleniyor?', body: '<p>Order bir komut gönderdi ve cevabını bekliyor. DLQ’ya bak: ne birikmiş?</p>', view: rabbitS1,
        task: { text: 'payment.requests.dlq’yu aç ve “Get Message(s)” ile mesajları oku.', req: ['get-messages:payment.requests.dlq'], hint: 'Tabloda DLQ satırına tıkla.' },
        choice: { q: 'Order hangi mesajı bekliyor?', opts: [
          { t: 'PaymentRefunded (RefundRequested komutunun cevabı)', ok: true, fb: 'DLQ’da 5 RefundRequested var: her biri 5 deneme sonunda “provider unavailable” ile düşmüş. Cevap (PaymentRefunded) hiç üretilmedi.' },
          { t: 'PaymentSucceeded', ok: false, fb: 'O zaten geldi (history: payment succeeded).' },
          { t: 'OrderCancelled', ok: false, fb: 'OrderCancelled’ı Order kendisi yayınlar; kimseden beklemez.' },
        ] } },
      { phase: 'teshis', title: 'Soru 4 · Hata geçici mi, kalıcı mı, belirsiz mi?', body: '<p>Jaeger’da Payment’ın refund denemelerine bak. Sipariş id’si trace’te <code>correlation_id</code> olarak geçer.</p>', view: jaegerS1,
        task: { text: 'Bir trace aç; Order’daki telafi span’lerini (db: UPDATE orders, publish RefundRequested) ve Payment’taki hatalı span’i incele.', req: [/^span:r\de$/], hint: 'Hatalı (kırmızı) “process payment.requests” span’ine tıkla.' },
        choice: { q: 'Bu hata hakkında en doğru değerlendirme?', opts: [
          { t: 'Kalıcı veri hatası: mesaj bozuk', ok: false, fb: 'Payload geçerli; hata metni altyapıyı (provider unavailable, deadline exceeded) gösteriyor.' },
          { t: 'Geçici bir altyapı hatası ama uzun sürmüş; ve SONUÇ BELİRSİZ olabilir: timeout’ta sağlayıcı iadeyi yapmış ama cevabı kaybolmuş olabilir', ok: true, fb: '“deadline exceeded” = cevabı alamadık; iade gerçekleşmiş de olabilir. Elle iade etmeden önce sağlayıcı tarafında bu ödemenin iade durumunu doğrula, yoksa çift iade riski var.' },
          { t: 'Geçici, kendiliğinden düzelecek; beklemek yeterli', ok: false, fb: 'Kurtarma işi 5 denemeden sonra durdu ve DLQ otomatik tüketilmez: kendiliğinden düzelmez.' },
        ] } },
      { phase: 'aksiyon', title: 'Soru 5 · Compensation gerekli mi?', choice: { q: 'Ne yaparsın?', opts: [
        { t: 'Evet. Önce sağlayıcıda iadenin durumunu kontrol et; sağlayıcı düzelince DLQ’daki RefundRequested’i kontrollü yeniden yayınla (Payment iadeyi bir kez yapar)', ok: true, fb: 'Payment iade isteğinde idempotent: tekrar istekler tek iade yapar ve her birine cevap verir (TestRepeatedRefundRequestsRefundOnceAndAnswerEachTime). Kurtarma işi sınırı doldurduğu için kendiliğinden tekrar istemeyecek; mesajı bir insan geri göndermeli.' },
        { t: 'Hayır, sipariş zaten iptal; Saga bitti', ok: false, fb: 'Müşterinin parası alındı ve geri verilmedi: Saga telafi edilmiş sayılamaz.' },
        { t: 'Siparişi elle “refunded” işaretle', ok: false, fb: 'Para hareketi olmadan durumu değiştirmek sorunu gizler; defter ve gerçek ayrışır.' },
      ] }, code: [{ title: 'Komutlar (öneri)', lang: 'c-ok', body: '# 1) DLQ mesajlarını incele (silmeden): Management UI ▸ payment.requests.dlq ▸ Get messages (Nack, requeue)\n# 2) Sağlayıcı düzeldikten sonra mesajı iş kuyruğuna geri gönder: UI ▸ Publish message\n#    (exchange: (AMQP default), routing key: payment.requests, payload: DLQ’daki gövde)\n# 3) Order tarafında refund_state’i izle:\ndocker compose exec postgres psql -U orders orders -c "SELECT refund_state, refund_requests FROM orders WHERE id=\'<order-id>\'"' }] },
      { phase: 'dogrulama', title: 'Soru 6 · Hangi dashboard bunu doğrular?', view: dashS1(false),
        task: { text: '“Refunds” ve “Recovery job actions” panellerine tıkla: istenen ve yapılan iadeler ne zaman ayrıştı?', req: ['panel:refunds', 'panel:recovery'] },
        choice: { q: 'Toplu görünümde problemi en iyi hangi işaret gösterir?', opts: [
          { t: 'İstenen iade (orders_refunds_requested_total) sürerken yapılan iade (payments_refunded_total) sıfıra düştü; refund_requested_again ve retried/dead_lettered yükseldi', ok: true, fb: 'İki sayacın farkı “bekleyen telafi”dir. Sipariş bazında değil, toplu sinyal: kaç müşteri etkilendi, ne zaman başladı.' },
          { t: 'HTTP 5xx oranı', ok: false, fb: 'Telafiler asenkron: HTTP katmanı yeşil kalır.' },
        ] } },
      { phase: 'dogrulama', title: 'Soru 7 · Düzelttiğimi nasıl doğrularım?', body: '<p>Sağlayıcı düzeldi, DLQ’daki mesajlar yeniden yayınlandı. Son 6 saat:</p>', view: dashS1(true), choice: { q: 'Hangisi birlikte doğrular?', opts: [
        { t: 'Siparişte refund_state = refunded; payments_refunded_total arttı; DLQ boş; retried sıfır; RefundNotAnswered kapandı; yeni trace’te refund span’i hatasız', ok: true, fb: 'Tekil (sipariş satırı, trace) ve toplu (sayaçlar, kuyruk, uyarı) kanıt birlikte.' },
        { t: 'DLQ boşaldı', ok: false, fb: 'Purge de DLQ’yu boşaltır: mesajların İŞLENDİĞİNİ göstermez.' },
        { t: 'Payment servisi ayakta', ok: false, fb: 'Servis baştan beri ayaktaydı; sorun sağlayıcıdaydı.' },
      ] } },
    ], summary: '<table><tr><th>Soru</th><th>Nereye bakılır?</th></tr><tr><td>1 Saga hangi durumda?</td><td>orders satırı / admin ucu (status, reservation_state, refund_state)</td></tr><tr><td>2 Son başarılı adım?</td><td>order_status_history</td></tr><tr><td>3 Hangi mesaj bekleniyor?</td><td>Durum modeli + RabbitMQ (DLQ, kuyruklar)</td></tr><tr><td>4 Geçici / kalıcı / belirsiz?</td><td>Jaeger span hatası, x-error, log</td></tr><tr><td>5 Telafi gerekli mi?</td><td>İş kuralı + dış sistemde doğrulama</td></tr><tr><td>6 Hangi pano doğrular?</td><td>Grafana: Refunds, Recovery actions, Messaging</td></tr><tr><td>7 Düzeldi mi?</td><td>refund_state=refunded + sayaçlar + uyarı + yeni trace</td></tr></table>' },

    { id: 'S2', title: 'Ödeme bekleyen Saga’lar birikiyor', tools: ['Postgres', 'Grafana', 'RabbitMQ'], stages: [
      { phase: 'durum', title: 'Durum: kampanya akşamı', body: '<p>Checkout hızı ≈3/sn. Bir süredir müşteriler siparişlerinin “ödeme bekleniyor”da kaldığını yazıyor; bazıları iptal bildirimi almış.</p>' },
      { phase: 'belirti', title: 'Belirti: iptaller artıyor', view: dashS2(false), body: '<p>“Orders finished” panelinde <code>cancelled/payment_timeout</code> yükseliyor, “Recovery job actions”ta <code>payment_requested_again</code> patlamış.</p>', task: { text: 'İki paneli incele (tıkla).', req: ['panel:finished', 'panel:recovery'] } },
      { phase: 'ilk', title: 'Soru 1 · Kaç Saga, hangi durumda?', choice: { q: '“Şu an kaç Saga yolda ve hangi adımda?” sorusunu Rovena’da nasıl cevaplarsın?', opts: [
        { t: 'Prometheus’ta saga_in_progress metriği', ok: false, fb: 'Rovena’da böyle bir metrik YOK (Code Lab ▸ ÖNERİ: saga_checkout_in_progress gauge). Bugün cevap veritabanında.' },
        { t: 'Order DB’de durum başına sayım (ya da admin listesi status filtresiyle)', ok: true, view: { tool: 'logs', command: "docker compose exec postgres psql -U orders orders -c \"SELECT status, count(*) FROM orders WHERE status IN ('pending','awaiting_payment','paid') GROUP BY status\"", lines: [' status           | count', '------------------+-------', ' awaiting_payment |   812', ' pending          |     0', ' paid             |     0'], note: 'Sorgu gerçek şemaya göre; sayılar SİMÜLE.' }, fb: '812 Saga ikinci adımda (ödeme) bekliyor; pending ve paid sıfır: Inventory ve commit tarafı sağlıklı.' },
        { t: 'Jaeger’da trace sayısı', ok: false, fb: 'Trace örneklenebilir ve bellekte tutulur; sayım için değil, tekil inceleme için.' },
      ] } },
      { phase: 'inceleme', title: 'Soru 2-3 · Son başarılı adım ve beklenen mesaj', choice: { q: 'awaiting_payment’taki bir Saga için son başarılı adım ve beklenen mesaj?', opts: [
        { t: 'Stok rezerve edildi ve PaymentRequested outbox’tan yayınlandı; PaymentSucceeded/Failed bekleniyor', ok: true, fb: 'awaiting_payment = reservation held + payment_requests ≥ 1. Cevap order.payment-results kuyruğundan gelmeli.' },
        { t: 'Ödeme alındı, stok onayı bekleniyor', ok: false, fb: 'O “paid” durumu olurdu.' },
      ] } },
      { phase: 'inceleme', title: 'RabbitMQ: mesajlar nerede?', view: rabbitS2, task: { text: 'payment.requests kuyruğunu aç: consumer var mı?', req: ['queue:payment.requests'] },
        choice: { q: 'Gördüğün ne?', opts: [
          { t: 'Consumers = 0, Ready 4.120 ve büyüyor: Payment tüketmiyor; publish hızı checkout’tan yüksek çünkü kurtarma işi aynı siparişler için tekrar istiyor', ok: true, fb: 'Retry/timeout mekanizmaları sorun sırasında yükü ÇOĞALTIR (her sipariş 3 istek). Idempotency key sayesinde çift çekim olmaz ama kuyruk büyür.' },
          { t: 'Mesajlar yönlendirilemiyor', ok: false, fb: 'Ready büyüyor: mesajlar kuyruğa ulaşıyor.' },
        ] } },
      { phase: 'teshis', title: 'Soru 4 · Geçici mi, kalıcı mı, belirsiz mi?', choice: { q: 'Bu durum için doğru değerlendirme?', opts: [
        { t: 'Geçici (consumer yok). Ama timeout’a düşen siparişler iptal edildi: Payment geri gelince bekleyen istekleri işleyecek ve bu siparişlerin bazıları için para çekilecek → geç gelen başarılar iade edilecek', ok: true, fb: 'Belirsizlik burada: iptal edilmiş siparişler için ödeme sonradan alınabilir. Rovena bunu iptal edilmiş siparişe gelen PaymentSucceeded’i iade ederek telafi eder.' },
        { t: 'Kalıcı; mesajları purge et', ok: false, fb: 'Purge 812 Saga’nın ödeme isteğini siler; kurtarma işi sınırı doldurduğu için iptallere düşerler. Veri ve müşteri kaybı.' },
      ] } },
      { phase: 'aksiyon', title: 'Soru 5 · Compensation gerekli mi?', choice: { q: 'Payment’ı ayağa kaldırdın. Telafi tarafında ne izlersin?', opts: [
        { t: 'payment_timeout ile iptal edilmiş siparişlere gelen ödemelerin iade edildiğini: orders_refunds_requested_total{reason="payment_timeout"} ve payments_refunded_total', ok: true, fb: 'Telafi otomatik ama doğrulanmalı. RefundNotAnswered uyarısı kapalı kalmalı.' },
        { t: 'Hiçbir şey: Payment ayağa kalktı, iş bitti', ok: false, fb: 'Bekleyen kuyruk işlenirken iptal edilmiş siparişler için ödemeler alınabilir; iadelerin gerçekleştiği görülmeli.' },
      ] } },
      { phase: 'dogrulama', title: 'Soru 6-7 · Doğrulama', view: dashS2(true), body: '<p>Payment geri döndü. Son 6 saat:</p>', choice: { q: 'Düzeldiğini ne gösterir?', opts: [
        { t: 'Kuyruk eriyor, confirmed yeniden artıyor, payment_requested_again/payment_timeout sıfıra iniyor, geç ödemeler için iadeler artıp sonra duruyor; awaiting_payment sayımı normale döndü', ok: true, fb: 'İş sonucu (confirmed), taşıma (kuyruk), kurtarma (recovery actions) ve telafi (refunds) birlikte.' },
        { t: 'Payment target’ı UP', ok: false, fb: 'Servisin ayakta olması işlerin bittiğini göstermez.' },
      ] } },
    ], summary: '<p><b>Ders:</b> Retry ve timeout mekanizmaları bir arıza sırasında yükü çoğaltır; idempotency bunu güvenli yapar ama ucuz yapmaz. “Kaç Saga yolda?” sorusu için Rovena’da bir metrik yok: ya DB’ye sorulur ya da önerilen gauge eklenir.</p>' },
  ];

  RL.initObs = function (root) {
    const labHost = h('div');
    root.append(
      h('div', { class: 'card' }, [h('h2', null, 'Saga’yı ne ile ölçerim?'), h('p', { class: 'muted' }, 'Prometheus toplu sayılar içindir: kaç Saga bitti, kaçı telafiye düştü, ne kadar sürdü. Tek bir Saga’yı (sipariş) takip etmek için trace (correlation_id = sipariş id’si), log ve DB kullanılır. Sipariş/Saga id’si asla metrik etiketi yapılmaz: her yeni sipariş yeni bir zaman serisi açar.'),
        h('table', { class: 't' }, [h('tr', null, ['İhtiyaç', 'Rovena metriği', 'Tür', 'Var mı?', 'PromQL'].map((x) => h('th', null, x))),
          METRICS.map((m) => h('tr', null, [h('td', null, m[0]), h('td', { class: 'mono' }, m[1]), h('td', null, m[2]), h('td', { class: m[3] ? 'ok-t' : 'warn-t' }, m[3] ? 'var' : 'YOK (öneri)'), h('td', { class: 'mono', style: 'font-size:11.5px' }, m[4])]))]),
        h('p', null, ['Eksik iki metrik için örnek instrumentation: ', h('a', { href: '#', onclick: (e) => { e.preventDefault(); RL.openCode('instrumentation'); } }, 'Code Lab ▸ ÖNERİ: Saga süresi ve devam eden Saga')])]),
      h('div', { class: 'card' }, [h('h2', null, 'Trace context mesaj sınırlarını nasıl geçer?'), RL.code('HTTP:   traceparent başlığı            (Order → Inventory)\noutbox: Envelope.Traceparent          (olay DB’de beklerken iz onunla bekler)\nAMQP:   mesaj başlığında traceparent  (relay yayını → consumer span’i)\nSonuç:  checkout → publish PaymentRequested → process payment.requests → publish PaymentSucceeded\n        → process order.payment-results → POST inventory commit   (TEK trace)', 'c-trace'),
        h('p', { class: 'muted' }, 'Telafiler aynı trace’te görünür: cancel ve release DB span’leri, publish RefundRequested, Payment’ta refund işleme. Kurtarma işinin başlattığı adımlar (tekrar istek, timeout iptali) yeni bir trace açar; sipariş id’si correlation_id olarak log ve zarfta kalır.'),
        h('p', null, [h('b', null, 'Yayın ≠ iş: '), 'RabbitMQ’ya publish edilmesi işin yapıldığını göstermez. Mesaj kuyrukta bekleyebilir, retry’a, DLQ’ya düşebilir. İşin sonucu Saga durumunda (orders satırı) ve orders_finished_total’da görülür.'])]),
      h('div', { class: 'card' }, [h('h2', null, 'Hata ayıklama senaryoları'), h('p', { class: 'muted' }, 'Her senaryo yedi soru üzerinden ilerler: 1) Saga hangi durumda? 2) Son başarılı adım? 3) Hangi mesaj bekleniyor? 4) Geçici / kalıcı / belirsiz? 5) Telafi gerekli mi? 6) Hangi pano/trace doğrular? 7) Düzeldiğini nasıl doğrularım?'), labHost]),
    );
    RL.initLab(labHost);
  };
})();
