/*
 * Araç rehberi: Grafana / Prometheus / Jaeger / RabbitMQ arayüzlerini nasıl kullanacağın + PromQL oyun alanı.
 * Sorgu kataloğu Rovena'daki GERÇEK metrik adlarıyla yazıldı; demo serileri SİMÜLE.
 */
(function () {
  const { h, clear } = RL;
  const { g, flat, S, sh, T0 } = RL.sim;

  /* ----------- Metrik kataloğu (gerçek) ----------- */
  const METRICS = [
    ['http_server_request_duration_seconds', 'histogram', 'http_route, http_request_method, http_response_status_code', true, 'Her servisin gelen isteklerinin süresi. Üç seri: _bucket, _sum, _count. Hız, hata oranı ve gecikmenin kaynağı.'],
    ['http_client_request_duration_seconds', 'histogram', 'peer, sonuç (200, 409, error)', true, 'Order’ın Catalog/Inventory çağrıları. Yavaşlıkta “hangi bağımlılık?” sorusu.'],
    ['messaging_events_published_total', 'counter', 'type, result', true, 'Broker’a yayınlanan olaylar. result="error" yayın hatası.'],
    ['messaging_events_consumed_total', 'counter', 'queue, type, result (ok | retried | dead_lettered)', true, 'Tüketim sonuçları. retried ve dead_lettered hata işareti.'],
    ['messaging_handler_duration_seconds', 'histogram', 'queue, type', true, 'Bir olayın handler’da harcadığı süre.'],
    ['outbox_pending_events', 'gauge', 'job', true, 'Yayınlanmayı bekleyen olay sayısı (Order, Payment).'],
    ['outbox_oldest_pending_age_seconds', 'gauge', 'job', true, 'En eski bekleyen olayın yaşı: “takıldı mı?” göstergesi.'],
    ['orders_finished_total', 'counter', 'status (confirmed | cancelled), code', true, 'Sonuçlanan siparişler; iptal nedeni code etiketinde.'],
    ['saga_recovery_actions_total', 'counter', 'action', true, 'Kurtarma işinin onardıkları. Normalde ≈ 0.'],
    ['payments_completed_total', 'counter', 'status, failure_code', true, 'Tamamlanan ödemeler.'],
    ['rabbitmq_detailed_queue_messages_ready', 'gauge', 'queue', true, 'Kuyrukta bekleyen (teslim edilmemiş) mesajlar.'],
    ['rabbitmq_detailed_queue_messages_unacked', 'gauge', 'queue', true, 'Teslim edilmiş ama ack bekleyen mesajlar.'],
    ['rabbitmq_detailed_queue_consumers', 'gauge', 'queue', false, 'Kuyruk başına consumer sayısı. Şu an çekilmiyor: yalnızca Management UI’da var.'],
    ['go_goroutines / go_memstats_alloc_bytes', 'gauge', 'job', true, 'Go çalışma zamanı: sızıntı ve yük işaretleri.'],
    ['up', 'gauge (Prometheus üretir)', 'job, instance', true, 'Hedefin son scrape’i başarılı mı (1/0). Uygulamanın kendi metriği değildir.'],
  ];

  /* ----------- PromQL sorgu kataloğu ----------- */
  const noisy = (base, seed, n = 0.05) => g(() => base, base * n, seed);
  const QUERIES = [
    { id: 'rps', t: 'İstek hızı (throughput)', expr: 'sum by (job) (rate(http_server_request_duration_seconds_count[1m]))', measures: 'Servis başına saniyede gelen istek sayısı.', when: 'Yük arttı mı? Gecikme artışı yükle mi ilgili?', read: 'Ani düşüş = trafik kesildi ya da servis cevap vermiyor; ani yükseliş = kampanya/bot/tekrar eden istek.', unit: '/s', series: () => [S('order', noisy(3.2, 1), '#5ab0ff'), S('catalog', noisy(6.1, 2), '#ffb454')] },
    { id: 'err', t: '5xx hata oranı (alarm kuralındaki sorgu)', expr: 'sum by (job) (rate(http_server_request_duration_seconds_count{http_response_status_code=~"5.."}[5m]))\n  / sum by (job) (rate(http_server_request_duration_seconds_count[5m]))', measures: 'Servis başına istekler arasında 5xx oranı (0-1).', when: 'Hata arttı mı, hangi serviste? HighServerErrorRate bunu kullanır (&gt; 0,05).', read: '0,05 = istekler %5’i hata. Hız değil <b>oran</b> olduğu için trafik azken de anlamlıdır.', unit: '', series: () => [S('catalog', g((t) => sh(t, 0.92, 0, 0.31, 0.01), 0.01, 3), '#ff7b72'), S('order', g((t) => sh(t, 0.92, 0, 0.59, 0.01), 0.01, 4), '#ffb454')], thr: [{ y: 0.05, label: '0.05' }] },
    { id: 'p95', t: 'Route başına p95 gecikme', expr: 'histogram_quantile(0.95, sum by (job, http_route, le)\n  (rate(http_server_request_duration_seconds_bucket[5m])))', measures: 'İsteklerin %95’inin bu süreden kısa sürdüğü değer.', when: 'Yavaşlık şikâyeti; SLO takibi.', read: 'p95 ortalamadan farklı olarak yavaş kuyruğu gösterir. le, toplamda kalmalı; yoksa sonuç anlamsız.', unit: 's', series: () => [S('/api/v1/checkout', g((t) => sh(t, 0.92, 0.42, 2.8, 0.03), 0.04, 5), '#ff7b72'), S('/api/v1/orders', noisy(0.05, 6, 0.1), '#5ab0ff')], thr: [{ y: 1, label: '1s' }] },
    { id: 'avg', t: 'Ortalama süre (dikkat!)', expr: 'rate(http_server_request_duration_seconds_sum[5m])\n  / rate(http_server_request_duration_seconds_count[5m])', measures: 'Son 5 dk’daki ortalama istek süresi.', when: 'Kaba bir genel bakış için; asıl yavaşlık tespiti için değil.', read: 'Ortalama, yavaş isteklerin etkisini gizler (az sayıda 10 sn’lik istek ortalamayı pek oynatmaz). Rate’siz <code>_sum/_count</code> ise süreç başladığından beri ortalama verir: yanıltıcı.', unit: 's', series: () => [S('avg', g((t) => sh(t, 0.92, 0.45, 0.9, 0.03), 0.02, 7), '#f2cc60'), S('p95', g((t) => sh(t, 0.92, 0.6, 2.8, 0.03), 0.04, 8), '#ff7b72')] },
    { id: 'peer', t: 'Bağımlılık başına p95 (Order → Catalog/Inventory)', expr: 'histogram_quantile(0.95, sum by (peer, le)\n  (rate(http_client_request_duration_seconds_bucket[5m])))', measures: 'Order’ın dış çağrılarının p95 süresi.', when: 'Checkout yavaş: süre bağımlılıkta mı?', read: 'Bir peer yükseliyorsa kök neden o servistedir (Jaeger’da o servisin span’lerine bak).', unit: 's', series: () => [S('inventory', g((t) => sh(t, 0.92, 0.18, 2.4, 0.03), 0.04, 9), '#7ee787'), S('catalog', noisy(0.03, 10, 0.1), '#ffb454')] },
    { id: 'consumed', t: 'Tüketim sonuçları', expr: 'sum by (queue, result) (rate(messaging_events_consumed_total[1m]))', measures: 'Kuyruk ve sonuç başına saniyede işlenen olay.', when: 'Mesaj işleme sağlığı: hata var mı, DLQ’ya gidiyor mu?', read: 'ok baskınsa sağlıklı. retried yükseliyorsa geçici/kalıcı hata; dead_lettered &gt; 0 ise bir insan bakmalı.', unit: '/s', series: () => [S('payment.requests ok', noisy(3.4, 11), '#7ee787'), S('retried', g((t) => (t < T0 ? 0 : 0.35), 0.05, 12), '#ffb454'), S('dead_lettered', g((t) => (t < 0.93 ? 0 : 0.02), 0.004, 13), '#ff7b72')] },
    { id: 'depth', t: 'Kuyruk derinliği', expr: 'rabbitmq_detailed_queue_messages_ready{queue="payment.requests"}', measures: 'Payment kuyruğunda bekleyen mesaj sayısı (GAUGE: rate gerekmez).', when: 'Birikme şüphesi; QueueBacklog (&gt;100, 5 dk).', read: 'Tek değer değil <b>eğim</b> önemli: yükselen eğim = giriş &gt; çıkış. Düz sıfır her zaman iyi demek değildir (bkz. Senaryo E).', unit: '', series: () => [S('payment.requests', g((t) => Math.max(0, (t - T0) / (1 - T0)) * 412, 1, 14), '#ffb454')], thr: [{ y: 100, label: '100' }] },
    { id: 'deriv', t: 'Kuyruk büyüme hızı', expr: 'deriv(rabbitmq_detailed_queue_messages_ready{queue="payment.requests"}[10m])', measures: 'Gauge’ın saniyedeki eğimi (mesaj/sn).', when: 'Birikme sürüyor mu, eriyor mu?', read: 'Pozitif = büyüyor, negatif = eriyor, ≈ 0 = dengede. Counter’larda <code>rate()</code>, gauge’larda <code>deriv()</code>.', unit: '/s', series: () => [S('eğim', g((t) => (t < T0 ? 0 : 2.8), 0.15, 15), '#c792ea')] },
    { id: 'handler', t: 'Handler p95 süresi', expr: 'histogram_quantile(0.95, sum by (queue, le)\n  (rate(messaging_handler_duration_seconds_bucket[5m])))', measures: 'Bir olayın işlenme süresi (p95).', when: 'Consumer yavaşladı mı? prefetch=1 ⇒ hız ≈ 1 / süre.', read: 'Yükselmesi, aynı kuyruk için çıkış hızının düştüğünü gösterir; sebebi trace’te aranır.', unit: 's', series: () => [S('payment.requests', g((t) => sh(t, T0, 0.35, 4.8, 0.02), 0.04, 16), '#ff7b72')] },
    { id: 'outbox', t: 'Outbox birikimi ve yaşı', expr: 'outbox_pending_events\noutbox_oldest_pending_age_seconds', measures: 'Yayınlanmayı bekleyen olay sayısı ve en eskisinin yaşı.', when: 'API başarılı ama iş bitmiyor (Senaryo E); OutboxBacklog / OutboxEventStuck.', read: 'Sayı küçük ama yaş büyüyorsa tek bir olay takılmış demektir; ikisi birlikte büyüyorsa yayın yolu kopuk.', unit: '', series: () => [S('pending', g((t) => (t < T0 ? 0 : ((t - T0) / (1 - T0)) * 1042), 3, 17), '#ff7b72')] },
    { id: 'orders', t: 'Sonuçlanan siparişler', expr: 'sum by (status, code) (increase(orders_finished_total[1h]))', measures: 'Son 1 saatte confirmed/cancelled sipariş sayısı, iptal nedeniyle.', when: 'İşin gerçek sonucu: ödeme ve onay akışı çalışıyor mu?', read: 'cancelled/payment_timeout artışı = Payment ya da mesajlaşma sorunu. <code>increase()</code> pencere toplamı verir, <code>rate()</code> saniyelik hız.', unit: '', series: () => [S('confirmed', g((t) => 380 + 40 * Math.sin(t * 20), 8, 18), '#7ee787'), S('cancelled', g((t) => (t < T0 ? 6 : 6 + (t - T0) * 600), 1, 19), '#ff7b72')] },
    { id: 'saga', t: 'Kurtarma işi', expr: 'sum by (action) (increase(saga_recovery_actions_total[5m]))', measures: 'Takılı siparişler için kurtarma işinin yaptığı işlemler.', when: 'Bir bağımlılık sağlıksız mı? Normalde 0’a yakın olmalı.', read: 'Artış kendi başına bir arıza değil, <b>başka bir arızanın belirtisidir</b> (Payment, RabbitMQ ya da Inventory sorunu).', unit: '', series: () => [S('payment_requested_again', g((t) => (t < T0 ? 0 : 6), 0.5, 20), '#f2cc60')] },
  ];

  /* ----------- Yardımcı çizimler ----------- */
  function queryDemo(q, range) {
    const w = h('div');
    w.append(RL.code(q.expr, 'c-warn'), RL.lineChart({ series: q.series(), unit: q.unit, range: range || 60, thresholds: q.thr, height: 140 }),
      h('dl', { class: 'kv', style: 'margin-top:8px' }, [
        h('dt', null, 'Ne ölçer?'), h('dd', { html: q.measures }), h('dt', null, 'Ne zaman?'), h('dd', { html: q.when }), h('dt', null, 'Nasıl yorumlanır?'), h('dd', { html: q.read }),
      ]), h('div', { class: 'note-sim' }, 'Grafik SİMÜLE; sorgu ve metrik adı gerçek.'));
    w.querySelectorAll('dd').forEach((d) => (d.style.fontFamily = 'var(--sans)'));
    return w;
  }

  function counterDemo() {
    const raw = g((t) => t * 900, 4, 31), rate = raw.map((v, i, a) => (i < 5 ? 0 : (v - a[i - 5]) / 300));
    return h('div', { class: 'compare' }, [
      h('div', null, [h('b', null, 'Counter (ham)'), RL.lineChart({ series: [S('http_server_request_duration_seconds_count', raw, '#7ee787')], height: 110, range: 120 }), h('p', { class: 'muted' }, 'Yalnızca artar (süreç yeniden başlarsa sıfırlanır). Ham değerin kendisi anlamsızdır: “1842” ne zamandan beri?')]),
      h('div', null, [h('b', null, 'rate(…[5m])'), RL.lineChart({ series: [S('rate', rate.map((v) => v * 60), '#5ab0ff')], height: 110, range: 120, unit: '/s' }), h('p', { class: 'muted' }, 'Saniyelik ortalama artış: grafiğin anlamlı hali. <code>increase(…[1h])</code> aynı artışı pencere toplamı olarak verir.')]),
    ]);
  }
  function gaugeDemo() {
    return h('div', null, [RL.lineChart({ series: [S('rabbitmq_detailed_queue_messages_ready', g((t) => 8 + 6 * Math.sin(t * 25), 2, 32), '#ffb454')], height: 110, range: 120 }), h('p', { class: 'muted' }, 'Yukarı-aşağı oynar: anlık durumu ölçer (kuyruk derinliği, goroutine sayısı, outbox bekleyen). rate() uygulanmaz; eğim için <code>deriv()</code>.')]);
  }
  function histogramDemo() {
    const b = [['0.05', 120], ['0.1', 480], ['0.25', 1710], ['0.5', 1810], ['1', 1830], ['2.5', 1840], ['+Inf', 1842]], tot = 1842, W = 360, H = 120;
    const svg = RL.s('svg', { viewBox: `0 0 ${W} ${H + 20}`, style: 'width:100%;max-width:520px' });
    b.forEach(([le, c], i) => {
      const bw = W / b.length - 6, x = i * (W / b.length) + 3, bh = (c / tot) * (H - 10);
      svg.append(RL.s('rect', { x, y: H - bh, width: bw, height: bh, rx: 3, fill: le === '0.5' ? '#7ee787' : '#2b4d7a' }), RL.s('text', { x: x + bw / 2, y: H + 12, 'text-anchor': 'middle', class: 'ax' }, 'le=' + le), RL.s('text', { x: x + bw / 2, y: H - bh - 3, 'text-anchor': 'middle', class: 'ax' }, c));
    });
    return h('div', null, [svg, h('p', { class: 'muted' }, ['Bucket’lar <b>kümülatif</b>tir: “le=0.25” = süresi ≤ 0,25 sn olan istek sayısı (1710/1842 = %92,8). p95’i (%95 = 1750. istek) <code>histogram_quantile</code> bu bucket’lar arasında interpolasyonla hesaplar (≈ 0,26-0,5 sn). Bu yüzden toplamda <code>le</code> etiketi kalmalı ve doğruluk bucket sınırlarına bağlıdır.'])]);
  }

  /* ----------- Sekmeler ----------- */
  RL.initGuide = function (root) {
    const tabs = [['four', 'Dört araç, dört soru'], ['grafana', 'Grafana'], ['prom', 'Prometheus & PromQL'], ['jaeger', 'Jaeger'], ['rabbit', 'RabbitMQ UI'], ['gaps', 'Eksikler & genişletme']];
    const bar = h('div', { class: 'gtabs' }), body = h('div');
    root.append(bar, body);
    let cur = 'four';
    function draw() {
      clear(bar); tabs.forEach(([k, l]) => bar.append(h('button', { class: k === cur ? 'on' : '', onclick: () => { cur = k; draw(); } }, l)));
      clear(body).append(({ four, grafana, prom, jaeger, rabbit, gaps })[cur]());
    }

    function four() {
      const card = (name, color, q, store, ui, never) => h('div', { class: 'card' }, [h('h3', { style: 'color:' + color }, name), h('p', null, [h('b', null, 'Cevapladığı soru: '), q]), h('p', null, [h('b', null, 'Neyi tutar: '), store]), h('p', null, [h('b', null, 'Rovena’da: '), ui]), h('p', { class: 'muted' }, [h('b', null, 'Tutmaz: '), never])]);
      return h('div', null, [
        h('div', { class: 'gcols' }, [
          card('Prometheus', 'var(--metric)', 'Ne kadar? Ne zaman? Hangi servis / route / kuyruk?', 'Sayısal zaman serileri (sayaç, gauge, histogram). Toplu özet, ucuz.', ':9090. Servisleri 10 sn’de bir çeker (pull). Uyarı kurallarını değerlendirir.', 'Tek bir isteğin hikâyesini, mesaj içeriğini.'),
          card('Grafana', '#f2994a', 'Bunu bir bakışta nasıl gösteririm?', 'Hiçbir şey. Datasource’lara (Prometheus, Jaeger) sorar ve çizer.', ':3000, “Rovena overview” JSON’dan provision edilir. Anonim admin (yalnızca geliştirme).', 'Kendi verisini; uygulamadan doğrudan metrik toplamaz.'),
          card('Jaeger', 'var(--trace)', 'Bu tek istekte süre nereye gitti? Hangi servis hata verdi?', 'Span ağaçları (trace). Bellekte; yeniden başlayınca silinir.', ':16686. Servisler OTLP ile :4318’e iter (push).', 'İstek hızı, hata oranı gibi toplu sayıları.'),
          card('RabbitMQ', 'var(--amqp)', 'Mesaj şu an nerede? Kim tüketiyor? Birikiyor mu?', 'Mesajların kendisi (kuyruklarda) ve broker’ın sayaçları.', ':15672 yönetim, :15692 Prometheus metrikleri. rovena.events topic exchange + 6 kuyruk.', 'Bir mesajın hangi HTTP isteğinden doğduğunu (ama başlıkta traceparent taşır).'),
        ]),
        h('div', { class: 'card' }, [h('h3', null, 'Belirtiye göre sıra'), h('table', { class: 't' }, [
          h('tr', null, ['Belirti', 'Sıra'].map((x) => h('th', null, x))),
          [['API yavaş', 'Grafana (hangi route) → Prometheus (p95, peer) → Jaeger (span) → log'], ['5xx arttı', 'Grafana → Prometheus (oran, route) → Jaeger (hata span’i) → log'], ['Kuyruk büyüyor', 'RabbitMQ UI + Grafana queue depth → handler p95 → Jaeger (consumer span’i)'], ['Mesaj hatası', 'Grafana (result) → log/Jaeger (neden) → RabbitMQ DLQ'], ['HTTP yeşil, iş bitmiyor', 'Outbox (Prometheus/Grafana) → Jaeger → RabbitMQ bağlantıları']].map((r) => h('tr', null, [h('td', null, r[0]), h('td', null, r[1])])),
        ])]),
        h('div', { class: 'card' }, [h('h3', null, 'Aynı olay, iki farklı kayıt'), h('div', { class: 'compare' }, [
          h('div', null, [h('b', { class: 'c-metric' }, 'Prometheus'), RL.code('http_server_request_duration_seconds_count\n  {job="order", http_route="/api/v1/checkout"}\n12:00:10 → 1840\n12:00:20 → 1842', 'c-metric')]),
          h('div', null, [h('b', { class: 'c-trace' }, 'Jaeger'), RL.code('trace 5b8a… (212 ms)\n order  POST /checkout     212\n  order  POST inventory     74\n  order  db: INSERT outbox   9', 'c-trace')]),
        ])]),
        h('div', { class: 'row-actions' }, [h('button', { class: 'btn', onclick: () => RL.show('flow') }, '← Mod 1’de akışı gör'), h('button', { class: 'btn', onclick: () => RL.show('lab') }, 'Mod 2’de senaryoları çöz →')]),
      ]);
    }

    function grafana() {
      const sel = h('select', { 'aria-label': 'Sorgu seç', onchange: () => show() }, QUERIES.map((q) => h('option', { value: q.id }, q.t)));
      const rng = h('select', { 'aria-label': 'Time range', onchange: () => show() }, [[15, 'Last 15 minutes'], [60, 'Last 1 hour'], [360, 'Last 6 hours']].map(([v, l]) => h('option', { value: v, selected: v === 60 }, l)));
      const out = h('div');
      function show() { clear(out); const q = QUERIES.find((x) => x.id === sel.value); out.append(h('h5', { style: 'margin:0 0 6px' }, q.t), queryDemo(q, +rng.value)); }
      show();
      return h('div', null, [
        h('div', { class: 'gcols' }, [
          ['Dashboard', 'Panolar topluluğu. Rovena’da “Rovena overview”: <code>backend/infrastructure/grafana/dashboards/rovena-overview.json</code>. Dosya <i>provision</i> edilir (<code>disableDeletion</code>): UI’dan değil JSON’dan düzenle.'],
          ['Panel', 'Tek bir görsel + <b>bir ya da birkaç sorgu</b>. Panel başlığı ▸ Edit ▸ Query’de PromQL’i görürsün. “Requests per second” bir <code>rate()</code> sorgusudur.'],
          ['Datasource', 'Verinin nereden geldiği: <b>Prometheus</b> (varsayılan, <code>http://prometheus:9090</code>) ve <b>Jaeger</b>. Grafana verinin kendisini tutmaz; datasource’u yanlış seçersen panel “No data” gösterir.'],
          ['Değişken (variable)', 'Pano üstündeki açılır liste (örn. <code>$job</code>); sorguda <code>{job="$job"}</code> olarak kullanılır. Aynı panoyu servis servis gezmeni sağlar. Rovena panosunda şimdilik değişken yok: eklemek için dashboard ayarlarında Variables.'],
          ['Time range', 'Sağ üstteki aralık. Yakın sorun için 15 dk–1 sa, “ne zaman başladı” için 6 sa. <b>Aralık, sorgunun rate penceresinden küçük olmamalı</b>.'],
          ['Eşik / uyarı çizgisi', 'Panelde kırmızı kesikli çizgi (örn. 1 sn). Uyarı <b>kuralları</b> ise Prometheus’ta (<code>alerts.yml</code>) yaşar; Alertmanager yok.'],
        ].map(([t, d]) => h('div', { class: 'qcard' }, [h('b', null, t), h('div', { class: 'meta', html: d })]))),
        h('div', { class: 'card' }, [h('h3', null, 'Panel oluşturma pratiği'), h('p', { class: 'muted' }, 'Grafana’da yeni panel: ① datasource = Prometheus ② Metric/PromQL ③ time range ④ görselleştirme (Time series). Aşağıda sorgu ve aralığı seç, paneli gör ve yorumla.'),
          h('div', { class: 'gf-head' }, [h('span', { class: 'pill' }, 'Datasource: Prometheus'), sel, h('span', { class: 'pill' }, 'Time range'), rng]), out]),
        h('div', { class: 'card' }, [h('h3', null, 'Gerçek panoyu açmak için'), RL.code('cd backend && make up\n# http://localhost:3000  ▸ Dashboards ▸ Rovena ▸ Rovena overview', 'c-ok')]),
      ]);
    }

    function prom() {
      const sel = h('select', { 'aria-label': 'Sorgu', onchange: () => run() }, QUERIES.map((q) => h('option', { value: q.id }, q.t)));
      const out = h('div');
      function run() { clear(out).append(queryDemo(QUERIES.find((x) => x.id === sel.value))); }
      run();
      const targets = RL.tools.window({ tool: 'prom', url: 'localhost:9090/targets', tab: 'targets', queries: null, targets: [
        ['identity', 'identity:8081'], ['catalog', 'catalog:8082'], ['inventory', 'inventory:8083'], ['payment', 'payment:8085'], ['order', 'order:8086'], ['rabbitmq', 'rabbitmq:15692/metrics/detailed'],
      ].map(([job, ep]) => ({ job, endpoint: ep + (job === 'rabbitmq' ? '' : '/metrics'), up: true, last: '4.2s ago', dur: '3ms' })) });
      return h('div', null, [
        h('div', { class: 'card' }, [h('h3', null, '1 · /targets: önce veri geliyor mu?'), h('p', null, 'Bir panel boşsa ilk adım Prometheus ▸ Status ▸ Targets. Hedef <b>DOWN</b> ise o servisin sayıları yoktur (ServiceDown uyarısı bu yüzden). Rovena’da beş servis + RabbitMQ’nun kuyruk sayıları çekilir:'), targets]),
        h('div', { class: 'card' }, [h('h3', null, '2 · Üç metrik türü'), h('div', { class: 'qcard' }, [h('b', null, 'Counter'), counterDemo()]), h('div', { class: 'qcard', style: 'margin-top:10px' }, [h('b', null, 'Gauge'), gaugeDemo()]), h('div', { class: 'qcard', style: 'margin-top:10px' }, [h('b', null, 'Histogram'), histogramDemo()]),
          h('table', { class: 't', style: 'margin-top:10px' }, [h('tr', null, ['Soru', 'Kullan'].map((x) => h('th', null, x))), [['Saniyede kaç? (counter)', '<code>rate(x_total[5m])</code>'], ['Son 1 saatte toplam kaç? (counter)', '<code>increase(x_total[1h])</code>'], ['Şu an kaç? (gauge)', '<code>x</code> ya da <code>max_over_time(x[10m])</code>'], ['Gauge büyüyor mu?', '<code>deriv(x[10m])</code>'], ['p95 süre (histogram)', '<code>histogram_quantile(0.95, sum by (le) (rate(x_bucket[5m])))</code>']].map((r) => h('tr', null, [h('td', null, r[0]), h('td', { html: r[1] })]))])]),
        h('div', { class: 'card' }, [h('h3', null, '3 · /graph: sorgu oyun alanı'), h('p', { class: 'muted' }, 'Rovena’nın gerçek metrikleriyle kurulmuş sorgu kataloğu. Her biri neyi ölçtüğünü, ne zaman kullanılacağını ve nasıl yorumlanacağını açıklar. Gerçek ortamda aynı sorguları http://localhost:9090/graph’a yapıştırabilirsin.'), h('div', { class: 'gf-head' }, [h('span', { class: 'pill' }, 'Expression'), sel]), out]),
        h('div', { class: 'card' }, [h('h3', null, '4 · Rovena metrik kataloğu'), h('table', { class: 't' }, [h('tr', null, ['Metrik', 'Tür', 'Etiketler', 'Mevcut mu?', 'Neden faydalı'].map((x) => h('th', null, x))), METRICS.map((m) => h('tr', null, [h('td', { class: 'mono' }, m[0]), h('td', null, m[1]), h('td', { class: 'muted' }, m[2]), h('td', { class: 'avail ' + (m[3] ? 'yes' : 'no') }, m[3] ? 'var' : 'YOK'), h('td', null, m[4])]))]),
          h('p', { class: 'muted', style: 'margin-top:8px' }, 'OpenTelemetry adları (messaging.events.consumed) Prometheus’a noktalar altçizgi, counter’lara _total, süre histogramlarına _seconds eklenerek gelir.')]),
      ]);
    }

    function jaeger() {
      const sp = (id, parent, svc, name, start, dur, extra) => Object.assign({ id, parent, svc, name, start, dur }, extra || {});
      const spans = [sp('j1', null, 'order', 'POST /api/v1/checkout', 0, 212), sp('j2', 'j1', 'order', 'POST catalog', 8, 38), sp('j3', 'j2', 'catalog', 'POST /api/v1/internal/variants/lookup', 12, 31), sp('j4', 'j1', 'order', 'POST inventory', 60, 74), sp('j5', 'j4', 'inventory', 'POST /api/v1/internal/reservations', 66, 69), sp('j6', 'j1', 'order', 'db: INSERT orders / outbox_events', 190, 20),
        sp('j7', 'j1', 'order', 'publish PaymentRequested', 420, 6, { async: true }), sp('j8', 'j7', 'payment', 'process payment.requests', 440, 318, { async: true, tags: { event_id: '0192f…', 'messaging.retry_count': 0 } }), sp('j9', 'j8', 'payment', 'publish PaymentSucceeded', 740, 5, { async: true }), sp('j10', 'j9', 'order', 'process order.payment-results', 770, 41, { async: true }), sp('j11', 'j10', 'inventory', 'POST …/reservations/:id/commit', 790, 22, { async: true })];
      const win = RL.tools.window({ tool: 'jaeger', url: 'localhost:16686/search', service: 'order', traces: [{ id: '5b8a91c20f3d', service: 'order', services: ['order', 'catalog', 'inventory', 'payment'], op: 'POST /api/v1/checkout', dur: 812, ago: 2, spans }] });
      return h('div', null, [
        h('div', { class: 'gcols' }, [
          ['1 · Servis seç', 'Sol üstte <b>Service</b>: order, catalog, inventory, payment, identity. Trace, o servisin en az bir span’ini içeren akışlardır.'],
          ['2 · Operation seç', '“POST /api/v1/checkout”, “process payment.requests”, “publish PaymentRequested”… <i>all</i> bırakırsan hepsi.'],
          ['3 · Filtrele', '<b>Tags:</b> <code>error=true</code> hatalı trace’ler; <b>Min Duration:</b> <code>1s</code> yavaşlar; Lookback: zaman aralığı.'],
          ['4 · Waterfall oku', 'Her satır bir span; çubuk başlangıç ve süre. Üstteki kök, altındakiler çocukları. <b>Self time</b> = kendi süresi − çocukların süresi.'],
          ['5 · Hatalı span', 'Kırmızı span/<code>error=true</code> etiketi. Span ayrıntısında exception log’u ve etiketler (db.statement, http.status_code).'],
          ['6 · Trace ID ile log', 'Servis logları <code>trace_id=…</code> taşır: Jaeger’da trace’i bul, <code>docker compose logs order | grep &lt;trace_id&gt;</code> ile aynı akışın loglarını çek.'],
        ].map(([t, d]) => h('div', { class: 'qcard' }, [h('b', null, t), h('div', { class: 'meta', html: d })]))),
        h('div', { class: 'card' }, [h('h3', null, 'Gerçek bir checkout’un trace’i (kod tabanındaki akış)'), h('p', { class: 'muted' }, 'Mor çubuklar HTTP yanıtından <b>sonra</b> çalışan asenkron adımlardır. “Find Traces”e bas, trace’i aç, span’lere tıkla.'), win,
          RL.code('# aynı akışın logları\ndocker compose logs order payment inventory | grep 5b8a91c20f3d', 'c-ok')]),
      ]);
    }

    function rabbit() {
      const q = (name, o) => Object.assign({ name, state: 'idle', ready: 0, unacked: 0, consumers: 0 }, o);
      const spec = { tool: 'rabbit', url: 'localhost:15672', tab: 'overview',
        queues: [q('order.payment-results', { state: 'running', incoming: 0.5, deliver: 0.5, ack: 0.5, consumers: 1 }), q('order.payment-results.dlq'), q('order.payment-results.retry'), q('payment.requests', { state: 'running', incoming: 0.5, deliver: 0.5, ack: 0.5, consumers: 1 }), q('payment.requests.dlq'), q('payment.requests.retry')],
        overview: { ready: 0, unacked: 0, readyHist: flat(0), unackedHist: flat(0.3, 0.3, 2), publish: flat(0.5, 0.1, 1), deliver: flat(0.5, 0.1, 3), ack: flat(0.5, 0.1, 4), caption: 'Sağlıklı: publish ≈ deliver ≈ ack, Ready ≈ 0.' },
        connections: [{ name: '172.18.0.7:50221 → 172.18.0.3:5672 (order)', user: 'rovena', state: 'running', channels: 2 }, { name: '172.18.0.9:51340 → 172.18.0.3:5672 (payment)', user: 'rovena', state: 'running', channels: 2 }],
        channels: [{ name: '172.18.0.7:50221 (1)', prefetch: 1, unacked: 0, consumers: 1 }, { name: '172.18.0.9:51340 (1)', prefetch: 1, unacked: 0, consumers: 1 }],
        exchanges: [{ name: '(AMQP default)', type: 'direct', features: 'D' }, { name: 'rovena.events', type: 'topic', features: 'D' }],
        bindings: [['PaymentRequested', 'payment.requests'], ['RefundRequested', 'payment.requests'], ['PaymentSucceeded', 'order.payment-results'], ['PaymentFailed', 'order.payment-results'], ['PaymentRefunded', 'order.payment-results']] };
      return h('div', null, [
        h('div', { class: 'gcols' }, [
          ['Overview', 'Toplam Ready/Unacked ve global publish / deliver / ack hızları. Sağlıklıyken üç hız birbirine yakındır.'],
          ['Connections / Channels', 'Hangi servis bağlı? Channel başına prefetch (Rovena: 1) ve unacked. Bir servis burada <b>yoksa</b> broker’a ulaşamıyordur (Senaryo E).'],
          ['Exchanges / Bindings', '<code>rovena.events</code> (topic) ve bağlı kuyruklar: olay tipi (routing key) → kuyruk. Bir olay yönlendirilemiyorsa ilk bakılacak yer.'],
          ['Queues', 'Ready (bekliyor), Unacked (alındı, ack bekliyor), Total; incoming / deliver / ack hızları; Consumers.'],
          ['Ready / Unacked', 'Ready ↑ ⇒ tüketen yok ya da yavaş. Unacked ↑ (takılı) ⇒ handler bitmiyor. prefetch=1 ⇒ Unacked ≤ consumer sayısı.'],
          ['Retry & DLQ', '<code>.retry</code> TTL’li bekleme kuyruğudur (tüketici yok), <code>.dlq</code> insan bekleyen mesajlar. Mesajın başlıklarında x-retry-count, x-error, x-failed-at.'],
        ].map(([t, d]) => h('div', { class: 'qcard' }, [h('b', null, t), h('div', { class: 'meta', html: d })]))),
        h('div', { class: 'card' }, [h('h3', null, 'Sağlıklı bir Rovena broker’ı'), h('p', { class: 'muted' }, 'Sekmelerde gez; kuyruğa tıklayıp ayrıntıları aç.'), RL.tools.window(spec)]),
        h('div', { class: 'card' }, [h('h3', null, 'Giriş'), RL.code('make up   # http://localhost:15672  (kullanıcı/şifre: backend/.env → RABBITMQ_USER / RABBITMQ_PASSWORD)', 'c-ok')]),
      ]);
    }

    function gaps() {
      return h('div', null, [
        h('div', { class: 'card' }, [h('h3', null, 'Projede bugün OLMAYAN şeyler (eğitimde varmış gibi gösterilmedi)'), h('ul', null, [
          h('li', { html: '<b>Alertmanager yok:</b> uyarılar yalnızca Prometheus ▸ Alerts ve Grafana’da görünür; kimseye bildirim gitmez (docs/todo.md: açık madde).' }),
          h('li', { html: '<b>Consumer sayısı metriği yok:</b> prometheus.yml yalnızca <code>queue_coarse_metrics</code> ailesini çekiyor. Consumers sütunu yalnızca Management UI’da.' }),
          h('li', { html: '<b>Frontend yok:</b> bu laboratuvar, <code>frontend/</code> altında ilk (ve bağımsız) arayüz. Gerçek mağaza arayüzü ayrı plan.' }),
          h('li', { html: '<b>Jaeger ve Prometheus verisi kalıcı değil:</b> yeniden başlatınca eski trace/zaman serisi kaybolur.' }),
          h('li', { html: '<b>Sağlayıcı çağrısı için ayrı span yok olabilir:</b> yoksa “process payment.requests” span’inin self time’ı uzun görünür; dış çağrılar için ayrı span eklemek teşhisi kolaylaştırır.' }),
        ])]),
        h('div', { class: 'card' }, [h('h3', null, 'Consumer sayısını Grafana’da görmek (öneri, uygulanmadı)'), RL.code('# backend/infrastructure/prometheus/prometheus.yml\n  - job_name: rabbitmq\n    metrics_path: /metrics/detailed\n    params:\n      family: [queue_coarse_metrics, queue_consumer_count]   # ← eklenecek\n    static_configs: [{targets: [\'rabbitmq:15692\']}]', 'c-warn'), RL.code('# sonra kullanılabilecek sorgu ve uyarı\nrabbitmq_detailed_queue_consumers{queue="payment.requests"} == 0', 'c-metric'), h('p', { class: 'muted' }, 'Bu değişiklik, çalışan ortamı etkilediği için bilerek yapılmadı; Prometheus yeniden başlatılır.')]),
        h('div', { class: 'card' }, [h('h3', null, 'Gerçek veriye geçiş'), h('p', null, 'Araç pencereleri (js/tools.js) bir <b>spec</b> (veri) alıp çizer; spec’i şu an js/scenarios*.js üretiyor. Gerçek Prometheus’a bağlamak için ilgili spec üreten kod yerine <code>GET http://localhost:9090/api/v1/query_range</code> sonuçlarını aynı şekle (seri: {name, data[]}) dönüştürmek yeterlidir; çizim kodu değişmez. Gerçek ve simüle veri bugün, her pencerede “SİMÜLE VERİ” rozetiyle ayrılır.')]),
      ]);
    }

    draw();
  };
})();
