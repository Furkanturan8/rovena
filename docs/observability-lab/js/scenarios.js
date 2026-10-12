/*
 * Mod 2 senaryoları A ve B + ortak yardımcılar.
 * Tüm sayılar SİMÜLE (deterministik üretilir). Metrik adları, PromQL, kuyruk adları ve uyarılar GERÇEK:
 * backend/infrastructure/{prometheus,grafana}, backend/shared/{messaging,outbox,telemetry}.
 */
(function () {
  const N = RL.N, sh = RL.shift;
  const g = (fn, noise = 0, seed = 1) => RL.gen(N, fn, noise, seed);
  const flat = (v, noise = 0, seed = 1) => g(() => v, noise, seed);
  const S = (name, data, color) => ({ name, data, color });
  const panel = (id, title, expr, series, o) => Object.assign({ id, title, expr, series }, o || {});
  const row = (title, panels) => ({ title, panels });
  const gf = (rows, extra) => Object.assign({ tool: 'grafana', dashboard: 'Rovena overview', url: 'localhost:3000/d/rovena-overview/rovena-overview', rows }, extra || {});
  const T0 = 0.905; // olayın başladığı an (≈ son 34 dk)
  RL.sim = { N, sh, g, flat, S, panel, row, gf, T0 };

  const EX = {
    rps: 'sum by (job) (rate(http_server_request_duration_seconds_count[1m]))',
    err: 'sum by (job) (rate(http_server_request_duration_seconds_count{http_response_status_code=~"5.."}[1m]))',
    p95: 'histogram_quantile(0.95, sum by (job, http_route, le) (rate(http_server_request_duration_seconds_bucket[5m])))',
    peer: 'histogram_quantile(0.95, sum by (peer, le) (rate(http_client_request_duration_seconds_bucket[5m])))',
    consumed: 'sum by (queue, result) (rate(messaging_events_consumed_total[1m]))',
    depth: 'rabbitmq_detailed_queue_messages_ready',
    outbox: 'outbox_pending_events',
    age: 'outbox_oldest_pending_age_seconds',
    handler: 'histogram_quantile(0.95, sum by (queue, le) (rate(messaging_handler_duration_seconds_bucket[5m])))',
    published: 'sum by (type, result) (rate(messaging_events_published_total[1m]))',
  };
  RL.EX = EX;

  const sc = (RL.scenarios = []);

  /* =========================================================================================
   * A: API isteği neden yavaşladı?
   * ========================================================================================= */
  function dashA(fixed) {
    const rec = fixed ? 0.962 : 2;
    const slow = (lo, hi, seed) => g((t) => sh(t, 0.92, lo, hi, 0.03) + (fixed ? sh(t, rec, 0, lo - hi, 0.02) : 0), 0.02 * hi, seed);
    return gf([row('Traffic and errors', [
      panel('rps', 'Requests per second, by service', EX.rps, [S('order', flat(3.2, 0.3, 1), '#5ab0ff'), S('catalog', flat(6.1, 0.5, 2), '#ffb454'), S('inventory', flat(3.5, 0.3, 3), '#7ee787')]),
      panel('err', '5xx responses per second', EX.err, [S('order', flat(0, 0, 1), '#5ab0ff'), S('catalog', flat(0, 0, 2), '#ffb454'), S('inventory', flat(0, 0, 3), '#7ee787')], { yMax: 1 }),
      panel('p95route', 'p95 latency, by route', EX.p95, [S('order /api/v1/checkout', slow(0.42, 2.8, 4), '#ff7b72'), S('order /api/v1/orders', flat(0.05, 0.01, 5), '#5ab0ff'), S('catalog /api/v1/products', flat(0.04, 0.01, 6), '#ffb454')], { unit: 's', thr: [{ y: 1, label: 'SlowRequests > 1s' }] }),
      panel('client', 'Calls from Order to Catalog and Inventory (p95)', EX.peer, [S('inventory', slow(0.18, 2.4, 7), '#7ee787'), S('catalog', flat(0.03, 0.005, 8), '#ffb454')], { unit: 's' }),
    ])], { range: 60, markers: [] });
  }

  const slowSpans = [
    { id: 'a-root', svc: 'order', name: 'POST /api/v1/checkout', start: 0, dur: 2940, tags: { 'http.route': '/api/v1/checkout', 'http.status_code': 201 } },
    { id: 'a-cat', parent: 'a-root', svc: 'order', name: 'POST catalog', start: 8, dur: 44 },
    { id: 'a-cat-s', parent: 'a-cat', svc: 'catalog', name: 'POST /api/v1/internal/variants/lookup', start: 12, dur: 36 },
    { id: 'a-cat-db', parent: 'a-cat-s', svc: 'catalog', name: 'db: SELECT variants', start: 15, dur: 24, tags: { 'db.system': 'postgresql' } },
    { id: 'a-inv', parent: 'a-root', svc: 'order', name: 'POST inventory', start: 60, dur: 2760 },
    { id: 'a-inv-s', parent: 'a-inv', svc: 'inventory', name: 'POST /api/v1/internal/reservations', start: 66, dur: 2748 },
    { id: 'a-db', parent: 'a-inv-s', svc: 'inventory', name: 'db: SELECT stock_levels … FOR UPDATE', start: 70, dur: 2690, tags: { 'db.system': 'postgresql', 'db.statement': 'SELECT … FROM stock_levels WHERE variant_id IN (?) FOR UPDATE', note: 'Değerler değil, yer tutucular kaydedilir' } },
    { id: 'a-ins', parent: 'a-inv-s', svc: 'inventory', name: 'db: INSERT reservations', start: 2765, dur: 40, tags: { 'db.system': 'postgresql' } },
    { id: 'a-odb', parent: 'a-root', svc: 'order', name: 'db: INSERT orders', start: 2830, dur: 18 },
    { id: 'a-obx', parent: 'a-root', svc: 'order', name: 'db: INSERT outbox_events', start: 2850, dur: 12 },
    { id: 'a-pub', parent: 'a-root', svc: 'order', name: 'publish PaymentRequested', start: 3120, dur: 8, async: true },
    { id: 'a-pay', parent: 'a-pub', svc: 'payment', name: 'process payment.requests', start: 3140, dur: 330, async: true },
  ];
  const fastSpans = slowSpans.map((x) => {
    const m = { 'a-root': [0, 330], 'a-inv': [60, 120], 'a-inv-s': [66, 108], 'a-db': [70, 22], 'a-ins': [96, 20], 'a-odb': [190, 18], 'a-obx': [210, 12], 'a-pub': [420, 8], 'a-pay': [440, 330] }[x.id];
    return m ? { ...x, start: m[0], dur: m[1] } : x;
  });
  const traceList = (spans, durs, svc) => durs.map(([d, ago], i) => ({ id: 'a1b2c3d4e5f6' + i + (svc || ''), service: 'order', op: 'POST /api/v1/checkout', dur: d, ago, spans: spans.map((x) => ({ ...x })) }));
  const jaegerA = (fixed) => ({
    tool: 'jaeger', url: 'localhost:16686/search', service: 'order', min: fixed ? '' : '', autoSearch: true,
    traces: fixed
      ? traceList(fastSpans, [[330, 2], [341, 3], [318, 5], [352, 6]]).concat(traceList(slowSpans, [[2910, 21], [3010, 24]]))
      : traceList(slowSpans, [[2940, 1], [2810, 2], [3080, 4], [2720, 5], [2990, 7], [3150, 9], [2660, 12], [2870, 18]]).concat(traceList(fastSpans.map((x) => ({ ...x })), [[455, 44], [431, 51], [468, 58]], 'f'))
        .concat([{ id: 'ordersx01', service: 'order', op: 'GET /api/v1/orders', dur: 58, ago: 3, spans: [{ id: 'g1', svc: 'order', name: 'GET /api/v1/orders', start: 0, dur: 58 }] }]),
  });

  const promA = () => ({
    tool: 'prom', url: 'localhost:9090/graph', queries: [
      { id: 'rate', expr: 'rate(http_server_request_duration_seconds_count[5m])', rows: [{ labels: { job: 'order', http_route: '/api/v1/checkout', http_request_method: 'POST' }, value: '3.18 /s' }, { labels: { job: 'catalog', http_route: '/api/v1/products', http_request_method: 'GET' }, value: '5.9 /s' }], verdict: { type: 'meh', text: 'Trafik normal: yavaşlamanın sebebi yük artışı değil. Bu bilgi değerli ama hangi route ve hangi bağımlılık sorusunu cevaplamıyor.' } },
      { id: 'avg', expr: 'http_server_request_duration_seconds_sum / http_server_request_duration_seconds_count', rows: [{ labels: { job: 'order', http_route: '/api/v1/checkout' }, value: '0.61 s', cls: 'warn-t' }], verdict: { type: 'bad', text: 'Yanıltıcı: sum/count süreç başladığından beri ORTALAMA verir; saatlerce normal geçen dönem yeni yavaşlığı sulandırır (0,6 sn “fena değil” gibi görünür) ve ortalama p95’i göstermez. Güncel durum için rate(...[5m]) kullan.' } },
      { id: 'route', expr: 'histogram_quantile(0.95, sum by (http_route, le) (rate(http_server_request_duration_seconds_bucket{job="order"}[5m])))', rows: [{ labels: { http_route: '/api/v1/checkout' }, value: '2.81 s', cls: 'bad-t' }, { labels: { http_route: '/api/v1/orders' }, value: '0.06 s' }, { labels: { http_route: '/api/v1/cart' }, value: '0.04 s' }], verdict: { type: 'good', text: 'Doğru sorgu: yavaşlayan tek route /api/v1/checkout. Diğerleri normal; sorun Order’ın tamamı değil, bu akış. Bucket’lardan p95 tahmini için sum by (…, le) şart: le etiketini toplamdan düşürürsen histogram_quantile anlamsız sonuç verir.' } },
      { id: 'peer', expr: 'histogram_quantile(0.95, sum by (peer, le) (rate(http_client_request_duration_seconds_bucket[5m])))', rows: [{ labels: { peer: 'inventory' }, value: '2.42 s', cls: 'bad-t' }, { labels: { peer: 'catalog' }, value: '0.03 s' }], verdict: { type: 'good', text: 'Checkout’un iki bağımlılığından yalnızca Inventory yavaş (2,4 sn). Süre Order’ın kendi kodunda değil, Inventory çağrısında: iç içe span’lere bakma zamanı.' } },
    ],
  });

  const rabbitIdle = () => ({
    tool: 'rabbit', url: 'localhost:15672/#/queues', tab: 'queues',
    queues: [
      { name: 'order.payment-results', state: 'idle', ready: 0, unacked: 0, incoming: 0.02, deliver: 0.02, ack: 0.02, consumers: 1 },
      { name: 'order.payment-results.dlq', state: 'idle', ready: 0, unacked: 0, consumers: 0 },
      { name: 'order.payment-results.retry', state: 'idle', ready: 0, unacked: 0, consumers: 0 },
      { name: 'payment.requests', state: 'running', ready: 0, unacked: 0, incoming: 3.1, deliver: 3.1, ack: 3.1, consumers: 1 },
      { name: 'payment.requests.dlq', state: 'idle', ready: 0, unacked: 0, consumers: 0 },
      { name: 'payment.requests.retry', state: 'idle', ready: 0, unacked: 0, consumers: 0 },
    ],
    overview: { ready: 0, unacked: 0, readyHist: flat(0), unackedHist: flat(0.2, 0.2, 3), publish: flat(3.1, 0.4, 1), deliver: flat(3.1, 0.4, 2), ack: flat(3.1, 0.4, 4), caption: 'Her şey sakin: publish ≈ deliver ≈ ack.' },
    connections: [], channels: [], exchanges: [], bindings: [],
  });

  sc.push({
    id: 'A', title: 'API isteği neden yavaşladı?', tools: ['Grafana', 'Prometheus', 'Jaeger'],
    stages: [
      { phase: 'durum', title: 'Durum: sıradan bir öğleden sonra', body: '<p>Rovena’da müşteriler sipariş veriyor; trafik her zamanki gibi (≈3 checkout/sn). Kimse bir değişiklik yaptığını hatırlamıyor.</p><p class="muted">Bu senaryoda bir endpoint’in yanıt süresinin neden arttığını bulacaksın: <b>Grafana → Prometheus → Jaeger</b> sırasıyla.</p>' },
      { phase: 'belirti', title: 'Belirti: “ödeme sayfası çok geç açılıyor”', body: '<p>Destek ekibi şunu iletiyor: <i>“Sipariş verirken sayfa uzun süre dönüyor, bazı müşteriler vazgeçiyor.”</i> Hata mesajı yok; sadece yavaşlık. Prometheus Alerts’te <code>SlowRequests</code> (p95 &gt; 1 sn, 5 dk) <b>pending</b>.</p>' },
      { phase: 'ilk', title: 'İlk kontrol: hangi aracı açarsın?', choice: {
        q: 'Belirti “yavaşlık”. İlk olarak hangi aracı açarsın?',
        opts: [
          { t: 'Grafana: Rovena overview panosu (gecikme, hata, trafik)', ok: true, fb: 'Yavaşlık ve hata belirtilerinde ilk bakış her zaman “trafik, hata, gecikme” üçlüsüdür; hangi route ve hangi servis olduğunu dashboard gösterir.' },
          { t: 'RabbitMQ Management ▸ Queues', ok: false, fb: 'Checkout’un HTTP yanıtı, ödeme kuyruğuna bakılmadan döner. Kuyruklar sakin (aşağıda); bu ekran HTTP gecikmesini açıklayamaz.', view: rabbitIdle() },
          { t: 'Jaeger’da rastgele bir trace aç', ok: false, fb: 'Çalışır ama verimsiz: hangi operasyonun yavaşladığını bilmeden “tipik” bir örnek seçersin. Önce metrikle daralt, sonra örnek trace aç.' },
          { t: 'docker compose logs order | tail', ok: false, fb: 'Log “ne oldu” sorusunu cevaplar; “ne kadar, nerede” için değil. Hata yoksa log sessizdir ve yavaşlık hakkında hiçbir şey söylemez.' },
        ],
        explain: 'Sıra: <b>belirti → metrik (genel resim) → trace (tek örnek) → log (neden)</b>. Metrik seni doğru trace’e götürür.',
      } },
      { phase: 'inceleme', title: 'Grafana: dört panelde ne değişti?', body: '<p>Panolardaki her grafik bir <b>PromQL</b> sorgusudur (panel başlığındaki “‹/› sorgu”ya tıkla). Zaman aralığını 1 saat bırak: olay ve öncesi aynı ekranda.</p>',
        view: dashA(false), qa: { look: 'Trafik, 5xx, route başına p95 ve Order’ın bağımlılık çağrılarının p95’i.', source: 'Hepsi Prometheus’tan; Prometheus bunları Order/Catalog/Inventory’nin /metrics uçlarından 10 sn’de bir toplamış.', meaning: 'Trafik düz ve 5xx sıfırsa sorun yük ya da hata değil, <b>gecikme</b>. p95 yükselmesi “yavaş isteklerin %5’i değil, daha fazlası” demektir.', next: 'Hangi route ve hangi bağımlılık?' },
        task: { text: '“p95 latency, by route” ve “Calls from Order to Catalog and Inventory (p95)” panellerine tıkla. Hangi route, hangi bağımlılık?', req: ['panel:p95route', 'panel:client'], hint: 'Kırmızı çizgi eşik (1 sn). Yalnızca bir çizgi sınırı aşıyor.' } },
      { phase: 'inceleme', title: 'Prometheus: aynı soruyu elle sor', body: '<p>Grafana paneli arka planda sorguyu Prometheus’a gönderir. Kök nedeni bulurken <b>Graph</b> sekmesinde aynı sorguları kendin çalıştırırsın. Aşağıdaki sorgulardan hangisi sana <i>hangi route</i> ve <i>hangi bağımlılığın</i> yavaş olduğunu söyler? Hepsini deneyebilirsin: yanlış olanlar da öğreticidir.</p>',
        code: [{ title: 'Kavramlar', lang: 'c-metric', body: 'rate(x_count[5m])       # son 5 dk’daki saniyelik ortalama artış (counter → hız)\nhistogram_quantile(0.95, …)  # bucket’lardan p95 tahmini\nsum by (route, le) (…)   # le (bucket sınırı) etiketi toplamda KALMALI', explain: '<b>Counter</b> yalnızca artar, ham değeri anlamsızdır: <code>rate()</code> ya da <code>increase()</code> ile kullan. <b>Histogram</b> süre dağılımıdır: <code>_bucket</code>, <code>_sum</code>, <code>_count</code> serilerinden p95 hesaplanır.' }],
        view: promA(), task: { text: 'p95’i route’a göre ve bağımlılığa (peer) göre veren iki sorguyu çalıştır.', req: ['query:route', 'query:peer'], hint: 'İkisi de histogram_quantile kullanıyor; biri http_server_, diğeri http_client_ metriği.' } },
      { phase: 'inceleme', title: 'Jaeger: yavaş isteğin trace’ini aç', body: '<p>Artık biliyorsun: yavaş olan <code>order · POST /api/v1/checkout</code> ve içinde Inventory çağrısı. Jaeger’da <b>Service = order</b>, <b>Min Duration = 1s</b> yazıp “Find Traces” de; yavaş bir trace aç ve <b>en uzun span</b>’i bul.</p><p class="muted">Span çubuğuna tıklayınca ayrıntıları, <b>self time</b>’ı (çocuklar hariç kendi süresi) ve etiketleri görürsün.</p>',
        view: jaegerA(false), qa: { look: 'Bir isteğin servisler arasındaki span ağacı; çubuk uzunluğu = süre.', source: 'Span’leri servisler OpenTelemetry (OTLP) ile Jaeger’a itti; veritabanı sorguları da ayrı span.', meaning: 'Bir span’in süresinin çoğu <b>çocuklarında</b> geçiyorsa suçlu aşağıdadır; çocuk yoksa/azsa süre span’in kendisinde (self time) geçiyordur.', next: 'Süre nerede geçiyor?' },
        task: { text: 'Yavaş bir trace aç ve Inventory’nin veritabanı span’ine (db: SELECT stock_levels … FOR UPDATE) tıkla.', req: ['span:a-db'], hint: 'Min Duration alanına 1s yaz, Find Traces’e bas. Waterfall’da en uzun mor-mavi çubuğa bak.' } },
      { phase: 'teshis', title: 'Teşhis: 2,9 saniyenin çoğu nerede geçti?', choice: {
        q: 'Trace’e göre HTTP isteğinin süresi nereden geliyor?',
        opts: [
          { t: 'Order handler kodunun kendisi (iş mantığı, JSON işleme)', ok: false, fb: 'Order kök span’inin <b>self time</b>’ı yaklaşık 100 ms: toplamın %4’ü. Süre çocuklarında.' },
          { t: 'Catalog’a yapılan fiyat sorgusu', ok: false, fb: 'Catalog çağrısı yalnızca 44 ms sürdü; metrikte de peer=catalog normal (0,03 sn).' },
          { t: 'Inventory’nin stok satırlarını kilitleyen veritabanı sorgusu (FOR UPDATE)', ok: true, fb: 'Inventory span’inin 2,75 saniyesinin 2,69’u tek bir DB span’inde: sorgu kilit bekliyor ya da çok yavaş. HTTP katmanı ve Order sağlıklı.' },
          { t: 'Asenkron ödeme akışı (RabbitMQ + Payment)', ok: false, fb: '<code>publish PaymentRequested</code> ve Payment span’leri trace’te kök span <b>bittikten sonra</b> başlıyor (mor çubuklar). HTTP yanıtı onları beklemez; kullanıcının beklediği süreye dahil değiller.' },
        ],
        explain: '<b>Üç ayrım:</b> (1) <i>Handler/self time</i>: span kısa çocuk içeriyor, kendisi uzun. (2) <i>DB</i>: uzun “db:” span’i. (3) <i>Asenkron</i>: kök span’den sonra başlayan span’ler, HTTP süresine dahil değil.',
      } },
      { phase: 'aksiyon', title: 'Aksiyon: önce ne kontrol edersin?', choice: {
        q: 'Süre Inventory’deki “SELECT … FOR UPDATE” sorgusunda. Geliştirici olarak ilk adımın?',
        opts: [
          { t: 'Inventory reserve sorgusunu ve transaction’ı incele: kilit bekleyen oturumlar, EXPLAIN ANALYZE, index, transaction süresi', ok: true, fb: 'Doğru. Kilit çekişmesi (aynı varyant üzerinde yarışan rezervasyonlar, uzun açık kalan transaction) ve eksik index klasik nedenlerdir.' },
          { t: 'RabbitMQ prefetch’ini artır', ok: false, fb: 'Kuyruk bu yavaşlığın parçası değil; consumer ayarı HTTP gecikmesini düzeltmez.' },
          { t: 'Order’ın Inventory timeout’unu kısalt', ok: false, fb: 'Semptomu hata oranına çevirir: yavaş istekler bu kez 5xx olur. Kök nedeni (DB) çözmez.' },
          { t: 'Prometheus scrape aralığını 2 sn’ye indir', ok: false, fb: 'Daha sık ölçmek nedeni değiştirmez; sorunu bulduk, ölçüm değil çözüm lazım.' },
        ],
        explain: 'İpucu: aynı anda iki oturum aynı satırı kilitlerse biri bekler; bu bekleme DB span’i olarak görünür.',
      }, code: [{ title: 'Postgres tarafında bakacağın yerler (Inventory DB)', lang: 'c-ok', body: "SELECT pid, state, wait_event_type, wait_event, now()-xact_start AS xact_age, left(query,80)\n  FROM pg_stat_activity WHERE datname='inventory' AND state <> 'idle' ORDER BY xact_age DESC;\nSELECT * FROM pg_locks WHERE NOT granted;       -- kimin neyi beklediği\nEXPLAIN (ANALYZE, BUFFERS) <rezervasyon sorgusu>;" }] },
      { phase: 'dogrulama', title: 'Doğrulama: düzeltmenin işe yaradığını nasıl anlarsın?', body: '<p>Düzeltme yayına alındı. Aşağıda “şimdi” ekranını görüyorsun (zaman aralığı: son 15 dk). Bir kanıt yetmez; birden fazla bağımsız sinyalin uyuşması gerekir.</p>',
        view: () => Object.assign(dashA(true), { range: 15 }), choice: {
          q: 'Hangisi “düzeldi” demek için en sağlam kanıttır?',
          opts: [
            { t: 'Deploy “başarılı” göründü', ok: false, fb: 'Deploy’un bitmesi düzeltmenin işe yaradığını göstermez.' },
            { t: 'Ben denedim, sayfa hızlı açıldı', ok: false, fb: 'Tek örnek, tek kullanıcı: p95 en yavaş %5’i ölçer, senin denemen onu yakalamaz.' },
            { t: 'p95 (route) ve p95 (peer=inventory) eşiğin altına indi, yeni trace’lerde DB span’i kısa, SlowRequests uyarısı kapandı', ok: true, fb: 'Üç bağımsız sinyal birlikte: metrik (genel), trace (tek örnek), alert (kural). <code>[5m]</code> pencereli sorgular eski yavaş örnekleri bir süre taşır; birkaç dakika bekle.' },
          ],
        } },
    ],
    summary: '<table><tr><th>Soru</th><th>Araç</th></tr><tr><td>Hangi route / servis yavaş, yük artmış mı, hata var mı?</td><td>Grafana (Prometheus sorguları)</td></tr><tr><td>Hangi bağımlılık yavaş? Tek bir sorguyla nasıl ölçerim?</td><td>Prometheus: <code>histogram_quantile(… http_client_…)</code></td></tr><tr><td>Bu yavaş istekte süre nereye gitti: handler, DB, asenkron?</td><td>Jaeger waterfall + self time</td></tr></table><p><b>Özet:</b> yavaşlıkta sıra <i>Grafana → Prometheus → Jaeger</i>. HTTP gecikmesine yalnızca isteğin <b>senkron</b> yolu dahildir; yanıttan sonra çalışan span’ler dahil değildir.</p>',
  });

  /* =========================================================================================
   * B: RabbitMQ kuyruğu büyüyor
   * ========================================================================================= */
  function rabbitB(fixed) {
    const grow = g((t) => (fixed ? (t < 0.94 ? Math.max(0, (t - T0) / (0.94 - T0)) * 412 : Math.max(0, 412 * (1 - (t - 0.94) / 0.045))) : Math.max(0, (t - T0) / (1 - T0)) * 412), 1, 5);
    const one = flat(1, 0, 1);
    const q = {
      name: 'payment.requests', state: 'running', ready: fixed ? 6 : 412, unacked: fixed ? 3 : 1, incoming: 3.1, deliver: fixed ? 6.4 : 0.2, ack: fixed ? 6.4 : 0.2, consumers: fixed ? 3 : 1,
      features: 'D', hist: { ready: grow, unacked: fixed ? flat(3, 0.3, 2) : one, rates: [S('incoming', flat(3.1, 0.3, 6), '#7ee787'), S('deliver', g((t) => (t < T0 ? 3.1 : fixed ? sh(t, 0.94, 0.2, 6.4, 0.02) : 0.2), 0.15, 7), '#ffb454'), S('ack', g((t) => (t < T0 ? 3.1 : fixed ? sh(t, 0.94, 0.2, 6.4, 0.02) : 0.2), 0.15, 8), '#c792ea')] },
      consumersList: Array.from({ length: fixed ? 3 : 1 }, (_, i) => ({ channel: `172.18.0.${9 + i}:5143${i} (1)`, prefetch: 1 })),
      arguments: 'durable: true',
      messages: ['Exchange: rovena.events   Routing key: PaymentRequested\nProperties: delivery_mode 2, message_id 0192f…\nPayload: {"event_id":"0192f…","type":"PaymentRequested","version":1,"correlation_id":"…","traceparent":"00-…","payload":{"order_id":"…","amount_cents":3000}}'],
    };
    return {
      tool: 'rabbit', url: 'localhost:15672/#/queues', tab: 'queues',
      queues: [
        { name: 'order.payment-results', state: 'idle', ready: 0, unacked: 0, incoming: 0.01, deliver: 0.01, ack: 0.01, consumers: 1, hist: { ready: flat(0), unacked: flat(0) }, consumersList: [{ channel: '172.18.0.7:49812 (1)', prefetch: 1 }] },
        { name: 'order.payment-results.dlq', state: 'idle', ready: 0, unacked: 0, consumers: 0 }, { name: 'order.payment-results.retry', state: 'idle', ready: 0, unacked: 0, consumers: 0 },
        q, { name: 'payment.requests.dlq', state: 'idle', ready: 0, unacked: 0, consumers: 0 }, { name: 'payment.requests.retry', state: 'idle', ready: 0, unacked: 0, consumers: 0 },
      ],
      overview: { ready: q.ready, unacked: q.unacked, readyHist: grow, unackedHist: one, publish: flat(3.1, 0.3, 1), deliver: g((t) => (t < T0 ? 3.1 : 0.2), 0.2, 2), ack: g((t) => (t < T0 ? 3.1 : 0.2), 0.2, 3), caption: 'Publish 3/sn sürerken deliver ve ack düştü: aradaki fark kuyruğa birikiyor.' },
      connections: [{ name: '172.18.0.7:49812 → 172.18.0.3:5672', user: 'rovena', state: 'running', channels: 2 }, { name: '172.18.0.9:51340 → 172.18.0.3:5672', user: 'rovena', state: 'running', channels: 2 }],
      channels: [{ name: '172.18.0.9:51340 (1)', prefetch: 1, unacked: 1, consumers: 1 }, { name: '172.18.0.7:49812 (1)', prefetch: 1, unacked: 0, consumers: 1 }],
      exchanges: [{ name: '(AMQP default)', type: 'direct', features: 'D' }, { name: 'rovena.events', type: 'topic', features: 'D' }],
      bindings: [['PaymentRequested', 'payment.requests'], ['RefundRequested', 'payment.requests'], ['PaymentSucceeded', 'order.payment-results'], ['PaymentFailed', 'order.payment-results'], ['PaymentRefunded', 'order.payment-results']],
      queueHint: 'Bir kuyruğa tıkla: consumer’lar, prefetch ve grafikler.',
    };
  }
  function dashB(fixed) {
    const rec = 0.94;
    const grow = g((t) => (fixed ? (t < rec ? Math.max(0, (t - T0) / (rec - T0)) * 412 : Math.max(0, 412 * (1 - (t - rec) / 0.045))) : Math.max(0, (t - T0) / (1 - T0)) * 412), 1, 5);
    const hand = g((t) => (t < T0 ? 0.35 : fixed ? (t < rec ? sh(t, T0, 0.35, 4.8, 0.02) : sh(t, rec, 4.8, 0.38, 0.02)) : sh(t, T0, 0.35, 4.8, 0.02)), 0.03, 11);
    const ok = g((t) => (t < T0 ? 3.1 : fixed ? (t < rec ? 0.2 : sh(t, rec, 0.2, 6.4, 0.02)) : 0.2), 0.12, 12);
    return gf([row('Messaging', [
      panel('consumed', 'Events consumed per second, by result', EX.consumed, [S('payment.requests ok', ok, '#7ee787'), S('payment.requests retried', flat(0, 0, 2), '#ffb454')]),
      panel('depth', 'Queue depth (messages waiting)', EX.depth, [S('payment.requests', grow, '#ffb454'), S('order.payment-results', flat(0), '#5ab0ff'), S('payment.requests.dlq', flat(0), '#ff7b72')], { thr: [{ y: 100, label: 'QueueBacklog > 100 (5 dk)' }] }),
      panel('outbox', 'Outbox: events waiting to be published', EX.outbox, [S('order', flat(0, 0, 1), '#5ab0ff'), S('payment', flat(0, 0, 2), '#7ee787')], { yMax: 50 }),
      panel('handler', 'Handler duration (p95)', EX.handler, [S('payment.requests', hand, '#ff7b72'), S('order.payment-results', flat(0.04, 0.005, 4), '#5ab0ff')], { unit: 's' }),
    ]), row('Traffic and errors', [
      panel('p95route', 'p95 latency, by route', EX.p95, [S('order /api/v1/checkout', flat(0.21, 0.02, 3), '#5ab0ff')], { unit: 's' }),
    ])], { range: 60 });
  }
  const promB = () => ({
    tool: 'prom', url: 'localhost:9090/graph', queries: [
      { id: 'ready', expr: 'rabbitmq_detailed_queue_messages_ready{queue="payment.requests"}', rows: [{ labels: { queue: 'payment.requests' }, value: '412', cls: 'bad-t' }], verdict: { type: 'meh', text: '<b>Gauge</b>: anlık değer, rate() GEREKMEZ. 412 mesaj bekliyor; ama tek değer “büyüyor mu, sabit mi” bilgisini vermez.' } },
      { id: 'deriv', expr: 'deriv(rabbitmq_detailed_queue_messages_ready{queue="payment.requests"}[10m])', rows: [{ labels: { queue: 'payment.requests' }, value: '+2.8 mesaj/sn', cls: 'bad-t' }], verdict: { type: 'good', text: 'Gauge’ların eğimi için <code>deriv()</code>: kuyruk saniyede ≈2,8 mesaj büyüyor (giriş 3,1 − çıkış 0,2 ≈ 2,9). Eğim pozitifse tüketim girişten yavaş. (Counter’lar için <code>rate()</code>, gauge’lar için <code>deriv()</code>.)' } },
      { id: 'handler95', expr: 'histogram_quantile(0.95, sum by (queue, le) (rate(messaging_handler_duration_seconds_bucket[5m])))', rows: [{ labels: { queue: 'payment.requests' }, value: '4.81 s', cls: 'bad-t' }, { labels: { queue: 'order.payment-results' }, value: '0.04 s' }], verdict: { type: 'good', text: 'Payment’ın handler’ı bir mesaj için 4,8 sn harcıyor (normal 0,35 sn). prefetch=1 olduğu için bir consumer’ın hızı ≈ 1 / 4,8 ≈ 0,2 mesaj/sn: tam olarak RabbitMQ’da gördüğün deliver hızı.' } },
      { id: 'consumed', expr: 'sum by (queue, result) (rate(messaging_events_consumed_total[5m]))', rows: [{ labels: { queue: 'payment.requests', result: 'ok' }, value: '0.20 /s' }, { labels: { queue: 'order.payment-results', result: 'ok' }, value: '0.01 /s' }], verdict: { type: 'meh', text: 'Tüketim hızı düşük ama hata (retried/dead_lettered) yok: tüketici <i>çalışıyor</i>, yavaş. Hata olsaydı result="retried" görünürdü.' } },
      { id: 'consumerscount', expr: 'rabbitmq_detailed_queue_consumers{queue="payment.requests"}', rows: [], verdict: { type: 'bad', text: 'Bu metrik Rovena’da <b>yok</b>: prometheus.yml yalnızca <code>queue_coarse_metrics</code> ailesini çekiyor. Consumer sayısı şimdilik yalnızca RabbitMQ Management UI’da. Eklemek istersen <code>family: [queue_coarse_metrics, queue_consumer_count]</code> yeterli (Rehber ▸ “Eksik metrikler”).' } },
    ],
  });
  const jaegerB = () => {
    const spans = [
      { id: 'b-root', svc: 'payment', name: 'process payment.requests', start: 0, dur: 4750, tags: { 'messaging.queue': 'payment.requests', 'messaging.retry_count': 0 } },
      { id: 'b-sel', parent: 'b-root', svc: 'payment', name: 'db: SELECT payments', start: 3, dur: 6 },
      { id: 'b-ins', parent: 'b-root', svc: 'payment', name: 'db: INSERT payments', start: 4720, dur: 8 },
      { id: 'b-pub', parent: 'b-root', svc: 'payment', name: 'publish PaymentSucceeded', start: 4735, dur: 4 },
    ];
    return { tool: 'jaeger', url: 'localhost:16686/search', service: 'payment', traces: [[4750, 1], [4810, 2], [4690, 3]].map(([d, ago], i) => ({ id: 'bb77aa' + i + 'cd', service: 'payment', op: 'process payment.requests', dur: d, ago, spans: spans.map((x) => ({ ...x })) })).concat([{ id: 'bb77aa9old', service: 'payment', op: 'process payment.requests', dur: 330, ago: 55, spans: spans.map((x) => ({ ...x, dur: x.id === 'b-root' ? 330 : x.dur, start: x.id === 'b-ins' ? 300 : x.id === 'b-pub' ? 315 : x.start })) }]) };
  };

  sc.push({
    id: 'B', title: 'RabbitMQ kuyruğu büyüyor', tools: ['RabbitMQ UI', 'Grafana', 'Prometheus', 'Jaeger'],
    stages: [
      { phase: 'durum', title: 'Durum: kampanya günü', body: '<p>Kampanya başladı; saniyede ≈3 sipariş geliyor. Order servisi checkout’u hızlıca tamamlıyor ve ödeme isteğini RabbitMQ’ya bırakıyor. Payment servisi bu isteği kuyruktan alıp işler.</p>' },
      { phase: 'belirti', title: 'Belirti: “ödedim ama sipariş hâlâ ödeme bekliyor”', body: '<p>Müşteriler ödeme yaptıklarını ama siparişin <code>awaiting_payment</code>’ta kaldığını söylüyor. Order API’si <code>201</code> dönüyor, hata oranı sıfır. Prometheus Alerts: <code>QueueBacklog</code> <b>pending</b>.</p>' },
      { phase: 'ilk', title: 'İlk kontrol', choice: { q: 'Belirti: işler tamamlanmıyor, API sağlıklı görünüyor. Nereden başlarsın?', opts: [
        { t: 'RabbitMQ Management UI ▸ Queues', ok: true, fb: 'İyi seçim: tek ekranda Ready, Unacked, consumer sayısı ve giriş/çıkış hızları. UI anlıktır; geçmişi Grafana gösterir.', view: rabbitB(false) },
        { t: 'Grafana ▸ Messaging satırı (Queue depth, Handler duration)', ok: true, fb: 'Bu da doğru: geçmiş eğilimi gösterir (ne zaman başladı, nasıl büyüyor). RabbitMQ UI ise o anki consumer’ları ve prefetch’i gösterir. İkisini birlikte kullanacağız.' },
        { t: 'Grafana ▸ HTTP latency ve 5xx panelleri', ok: false, fb: 'API sağlıklı; bu paneller yeşil kalır ve seni “sorun yok” sonucuna götürür. Sorun HTTP yanıtından <b>sonraki</b> asenkron yolda.' },
        { t: 'Order servisini yeniden başlat', ok: false, fb: 'Neden bilinmeden yeniden başlatma, mesajların nerede takıldığını göstermez. Kuyruk sağlam; teşhis önce.' },
      ] } },
      { phase: 'inceleme', title: 'RabbitMQ UI: kuyruğu aç', body: '<p><b>Queues</b> sekmesi her kuyruğun <i>Ready</i> (bekleyen), <i>Unacked</i> (teslim edilmiş ama ack edilmemiş), <i>incoming</i> (publish), <i>deliver/get</i> ve <i>ack</i> hızlarını gösterir. <code>payment.requests</code>’i aç ve hızlara, consumer’lara bak.</p>',
        view: rabbitB(false), qa: { look: 'Queues tablosu ve payment.requests ayrıntıları.', source: 'Doğrudan RabbitMQ broker’ı: uygulamaya sormadan kendi sayaçları.', meaning: 'Ready yükseliyor = giriş &gt; çıkış. Unacked = şu an işlenen mesajlar (prefetch=1 olduğu için en çok consumer sayısı kadar).', next: 'Ready neden yükseliyor? Consumer yok mu, yavaş mı, ack mi etmiyor?' },
        task: { text: '“payment.requests” kuyruğuna tıkla ve Consumers, deliver/ack hızlarına bak.', req: ['queue:payment.requests'], hint: 'Tablodaki satıra tıkla.' } },
      { phase: 'teshis', title: 'Teşhis 1: hangi arıza deseni?', body: '<p>Gördüğün: <code>Ready 412 · Unacked 1 · Consumers 1 · incoming 3.1/s · deliver 0.2/s · ack 0.2/s</code></p>', choice: {
        q: 'Bu rakamlar hangi durumu gösteriyor?',
        opts: [
          { t: 'Consumer durmuş', ok: false, fb: 'Durmuş olsaydı Consumers = <b>0</b>, deliver ve ack = 0 olurdu. Burada 1 consumer var ve mesaj alıyor.' },
          { t: 'Consumer çalışıyor ama girişten yavaş', ok: true, fb: 'Giriş 3,1/s, çıkış 0,2/s. Aradaki ≈2,9/s fark kuyruğa birikiyor. Unacked = 1 (prefetch=1: bir consumer aynı anda en çok 1 mesaj tutar).' },
          { t: 'Consumer mesajları ack etmiyor', ok: false, fb: 'Ack etmeyen consumer’da deliver ≈ ack olmazdı: <b>Unacked takılı kalır, ack hızı 0</b>. Burada ack ≈ deliver ≈ 0,2/s: işleniyor ama yavaş.' },
          { t: 'Exchange mesajı yönlendiremiyor', ok: false, fb: 'Yönlendirilemeseydi kuyrukta mesaj <b>olmazdı</b>; “incoming” hızı da 3,1/s’i göstermezdi. Mesajlar kuyruğa ulaşıyor.' },
        ],
        explain: '<table class="t"><tr><th>Durum</th><th>Ready</th><th>Unacked</th><th>Consumers</th><th>deliver</th><th>ack</th></tr><tr><td>Consumer durmuş</td><td>↑</td><td>0</td><td class="bad-t">0</td><td>0</td><td>0</td></tr><tr><td>Consumer yavaş</td><td>↑</td><td>1 (prefetch)</td><td>≥1</td><td>düşük &gt;0</td><td>düşük &gt;0</td></tr><tr><td>Ack etmiyor</td><td>↑</td><td class="warn-t">takılı</td><td>≥1</td><td>0</td><td class="bad-t">0</td></tr><tr><td>Yönlendirme yok</td><td>0</td><td>0</td><td>≥1</td><td>0</td><td>0</td></tr></table>',
      } },
      { phase: 'inceleme', title: 'Grafana: geçmiş ve handler süresi', body: '<p>RabbitMQ UI anlık görüntüyü verdi. Şimdi geçmişe bak: kuyruk <i>ne zaman</i> büyümeye başladı ve consumer’ın <i>handler süresi</i> ne yaptı?</p>',
        view: dashB(false), qa: { look: 'Queue depth, tüketim sonuçları ve handler p95.', source: 'Kuyruk derinliği RabbitMQ’nun :15692 uç noktasından (Prometheus çeker); handler süresi Payment’ın kendi histogramından.', meaning: 'Handler p95 sıçradıysa tüketici yavaşlamıştır. Outbox panelleri sıfırsa Order yayınlıyor, sorun aşağı yönde.', next: 'Handler neden yavaşladı?' },
        task: { text: '“Handler duration (p95)” ve “Queue depth” panellerine tıkla; ikisi aynı anda mı değişmiş?', req: ['panel:handler', 'panel:depth'] } },
      { phase: 'inceleme', title: 'Prometheus: gauge, counter ve histogram farkı', body: '<p>Kuyruk derinliği bir <b>gauge</b> (yukarı/aşağı), consumed bir <b>counter</b> (sürekli artan), handler süresi bir <b>histogram</b>. Her biri farklı sorgu tarzı ister. Aşağıdan <b>eğimi</b> ve <b>handler p95</b>’ini veren sorguları çalıştır.</p>', view: promB(),
        task: { text: 'Kuyruğun büyüme hızını (deriv) ve handler p95’ini çalıştır.', req: ['query:deriv', 'query:handler95'], hint: 'Gauge’ın eğimi: deriv(). Süre dağılımı: histogram_quantile.' } },
      { phase: 'inceleme', title: 'Jaeger: handler’ın içi', body: '<p>Handler 4,8 sn sürüyor. <code>payment · process payment.requests</code> trace’lerini aç ve süre <i>nerede</i> geçiyor bak.</p>', view: jaegerB(),
        qa: { look: 'Consumer span’i ve çocukları.', source: 'Consumer, mesajın traceparent’ine bağlanan bir span açar; DB sorguları çocuk span’dir.', meaning: 'Çocuk span’ler kısaysa süre span’in <b>self time</b>’ında: yani handler kodunun içinde (dış çağrı bekleme, uyku, hesaplama).', next: 'Dış bir sağlayıcı yavaşsa trace’te nasıl görünür?' },
        task: { text: 'Bir trace aç ve kök span’in self time’ına bak (span’e tıkla).', req: [/^span:b-root$/] } },
      { phase: 'aksiyon', title: 'Aksiyon', choice: { q: 'Handler’ın süresi, payment sağlayıcısını çağıran kodda geçiyor. Doğru aksiyon?', opts: [
        { t: 'Sağlayıcı gecikmesini/timeout’unu (PAYMENT_CHARGE_TIMEOUT) kontrol et; Payment’ın kopya (consumer) sayısını artır', ok: true, fb: 'Prefetch=1 ile her kopya aynı anda 1 mesaj işler: 3 kopya ≈ 3 kat çıkış. Birikme sürerken <b>hem</b> kök neden (sağlayıcı) <b>hem</b> kapasite (kopya) ele alınır.' },
        { t: 'payment.requests kuyruğunu purge et (boşalt)', ok: false, fb: 'Her mesaj bir <b>sipariş ödemesi</b>: purge siparişleri ödeme bekler halde bırakır ve veriyi siler. Mesajlar kayıp olarak sayılır.' },
        { t: 'Prefetch’i 100 yap', ok: false, fb: 'Kod değişikliği gerektirir ve sağlayıcı zaten yavaşken eşzamanlı çağrıyı artırmak onu daha da yorabilir; sıra garantisi de bozulur. Önce kopya sayısı ve sağlayıcı.' },
        { t: 'Order servisini yeniden başlat', ok: false, fb: 'Order sağlıklı; tıkanıklık aşağıda (Payment tüketimi).' },
      ] }, code: [{ title: 'Birikme aritmetiği', lang: 'c-amqp', body: 'çıkış hızı = consumer_sayısı × (1 / handler_süresi)\nşimdi : 1 × (1/4.8) = 0.21 /s    giriş: 3.1 /s    ⇒ +2.9 /s birikir\n3 kopya, süre 0.4 s: 3 × 2.5 = 7.5 /s ⇒ -4.4 /s, 412 mesaj ≈ 94 sn’de erir' }] },
      { phase: 'dogrulama', title: 'Doğrulama', body: '<p>Sağlayıcı sorunu giderildi ve Payment 3 kopyaya çıkarıldı. Aşağıda son 1 saat; kuyruğun erimeye başladığı anı ve handler süresinin normale döndüğünü gör.</p>', view: () => dashB(true), choice: { q: 'Düzeldiğini hangi kanıt gösterir?', opts: [
        { t: 'Unacked sıfır oldu', ok: false, fb: 'Unacked zaten prefetch yüzünden hep küçüktü (≤ consumer sayısı); iyileşmeyi göstermez.' },
        { t: 'Consumer sayısı 3’e çıktı', ok: false, fb: 'Yalnızca bir <i>aksiyon</i>. Sonuç değil: kuyruk gerçekten boşalıyor mu?' },
        { t: 'Ready eğimi negatif, deliver ≥ incoming, handler p95 normal, QueueBacklog uyarısı kapandı', ok: true, fb: 'Üç sinyal: kuyruk (Ready↓), akış (deliver ≥ incoming) ve neden (handler p95 normal). Uyarı <code>for: 5m</code> kuralı ile bir süre sonra kapanır.' },
      ] } },
    ],
    summary: '<table><tr><th>Belirti</th><th>Bakılacak</th></tr><tr><td>Ready ↑, Consumers = 0</td><td>Tüketici servisi çökmüş/dağıtımda: servis ve <code>up</code> metriği</td></tr><tr><td>Ready ↑, Unacked sabit, ack = 0</td><td>Handler takılmış (deadlock, sonsuz bekleme): trace + log</td></tr><tr><td>Ready ↑, deliver/ack düşük &gt; 0, handler p95 ↑</td><td>Yavaş consumer: dış bağımlılık, kapasite</td></tr><tr><td>Ready ↑, retried ↑</td><td>Hata → Senaryo C</td></tr><tr><td>Kuyruk boş ama iş bitmiyor</td><td>Mesaj hiç gelmiyor → Senaryo E (outbox)</td></tr></table>',
  });
})();
