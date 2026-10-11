/* Mod 2 senaryoları C, D, E. Yardımcılar scenarios.js'teki RL.sim / RL.EX'ten gelir. Sayılar SİMÜLE, adlar GERÇEK. */
(function () {
  const { sh, g, flat, S, panel, row, gf, T0 } = RL.sim, EX = RL.EX;
  const sc = RL.scenarios;
  const rq = (o) => Object.assign({ state: 'idle', ready: 0, unacked: 0, consumers: 0 }, o);

  /* =========================================================================================
   * C: Bir mesaj işlenirken hata oluşuyor
   * ========================================================================================= */
  const dlqMsg = (id, n) => `Properties: message_id=${id}\nHeaders:\n  x-retry-count: 5\n  x-error: decode PaymentRequested payload: invalid UUID length: 5\n  x-failed-at: 2026-10-12T09:1${n}:03Z\n  traceparent: 00-9a1f0c${n}…-…-01\nPayload: {"event_id":"${id}","type":"PaymentRequested","payload":{"order_id":"1234${n}", "amount_cents":3000}}`;
  function rabbitC() {
    const dlq = g((t) => (t < 0.93 ? 0 : t < 0.955 ? 1 : t < 0.975 ? 2 : 3), 0, 1);
    return {
      tool: 'rabbit', url: 'localhost:15672/#/queues', tab: 'queues',
      queues: [
        rq({ name: 'order.payment-results', ready: 0, incoming: 0.01, deliver: 0.01, ack: 0.01, consumers: 1 }), rq({ name: 'order.payment-results.dlq' }), rq({ name: 'order.payment-results.retry' }),
        rq({ name: 'payment.requests', state: 'running', ready: 0, incoming: 3.4, deliver: 3.4, ack: 3.4, consumers: 1 }),
        rq({ name: 'payment.requests.dlq', ready: 3, features: 'D', hist: { ready: dlq, unacked: flat(0) }, consumersList: [], messages: [dlqMsg('0192b7a1', 4), dlqMsg('0192b7a2', 5), dlqMsg('0192b7a3', 6)], arguments: 'durable: true' }),
        rq({ name: 'payment.requests.retry', state: 'running', ready: 1, incoming: 0.4, consumers: 0, arguments: 'x-message-ttl: 5000, x-dead-letter-exchange: "", x-dead-letter-routing-key: payment.requests' }),
      ],
      overview: { ready: 4, unacked: 0, readyHist: flat(1, 1, 3), unackedHist: flat(0), publish: flat(3.4, 0.3, 1), deliver: flat(3.4, 0.3, 2), ack: flat(3.4, 0.3, 4), caption: 'Genel akış normal görünür: hata, birkaç mesajda gizli kalır.' },
      connections: [], channels: [], exchanges: [], bindings: [],
      queueHint: 'payment.requests.dlq’ya tıkla, sonra “Get Message(s)”.',
    };
  }
  function dashC(fixed) {
    const rec = 0.972;
    const burst = (t, seed) => (t < T0 ? 0 : fixed && t > rec ? 0 : 0.18 + 0.2 * Math.abs(Math.sin(t * 900 + seed)));
    const dlqv = g((t) => (t < 0.93 ? 0 : t < 0.955 ? 1 : t < 0.975 ? 2 : 3), 0, 1);
    return gf([row('Messaging', [
      panel('consumed', 'Events consumed per second, by result', EX.consumed, [S('payment.requests ok', flat(3.4, 0.3, 1), '#7ee787'), S('payment.requests retried', g((t) => burst(t, 1), 0.01, 2), '#ffb454'), S('payment.requests dead_lettered', g((t) => (t < 0.93 ? 0 : fixed && t > rec ? 0 : 0.02), 0.004, 3), '#ff7b72')]),
      panel('depth', 'Queue depth (messages waiting)', EX.depth, [S('payment.requests.dlq', dlqv, '#ff7b72'), S('payment.requests', flat(0.3, 0.3, 4), '#5ab0ff'), S('payment.requests.retry', g((t) => burst(t, 2) * 5, 0.2, 5), '#ffb454')]),
      panel('handler', 'Handler duration (p95)', EX.handler, [S('payment.requests', flat(0.34, 0.04, 6), '#5ab0ff')], { unit: 's' }),
      panel('published', 'Events published per second, by type', EX.published, [S('PaymentRequested ok', flat(3.4, 0.3, 7), '#7ee787')]),
    ])], { range: 60 });
  }
  const logsC = () => ({
    tool: 'logs', command: 'docker compose logs --since 30m payment | grep -E "handler failed|dead-letter"',
    lines: [
      '09:01:12 level=WARN msg="event handler failed" queue=payment.requests event_id=0192a1c4 type=PaymentRequested trace_id=7d21aa01 error="dial tcp 172.18.0.4:5432: connect: connection refused"',
      '09:01:17 level=WARN msg="event handler failed" queue=payment.requests event_id=0192a1c4 type=PaymentRequested trace_id=7d21aa01 error="dial tcp 172.18.0.4:5432: connect: connection refused"',
      '09:01:22 (retry #2) event_id=0192a1c4 → işlendi, ack (log yok: hata olmadı)',
      '09:14:03 level=WARN msg="event handler failed" queue=payment.requests event_id=0192b7a1 type=PaymentRequested trace_id=9a1f0c04 error="decode PaymentRequested payload: invalid UUID length: 5"',
      '09:14:08 level=WARN msg="event handler failed" queue=payment.requests event_id=0192b7a1 type=PaymentRequested trace_id=9a1f0c04 error="decode PaymentRequested payload: invalid UUID length: 5"',
      '  … (aynı satır 09:14:13, :18, :23 için tekrar)',
      '09:14:28 level=ERROR msg="message moved to dead-letter queue" queue=payment.requests message_id=0192b7a1 error="decode PaymentRequested payload: invalid UUID length: 5"',
    ],
    note: 'Log satırları SİMÜLE; alan adları (queue, event_id, type, trace_id, error) gerçek log çıktısından.',
  });
  function jaegerC() {
    const mk = (id, ev, n, ok, ago, mode) => {
      const dur = ok ? 330 : mode === 'db' ? 38 : 3;
      const spans = [{ id: id, svc: 'payment', name: 'process payment.requests', start: 0, dur, err: !ok, tags: { event_id: ev, 'messaging.queue': 'payment.requests', 'messaging.retry_count': n }, logs: ok ? [] : [mode === 'db' ? 'dial tcp 172.18.0.4:5432: connect: connection refused' : 'decode PaymentRequested payload: invalid UUID length: 5'] }];
      if (mode === 'db' || ok) spans.push({ id: id + 'd', parent: id, svc: 'payment', name: 'db: SELECT payments', start: 2, dur: ok ? 6 : 31, err: !ok, logs: ok ? [] : ['dial tcp 172.18.0.4:5432: connect: connection refused'] });
      return { id: 'c-' + id, service: 'payment', op: 'process payment.requests', dur, ago, err: !ok, spans };
    };
    const a = [mk('ca0', '0192a1c4', 0, false, 15, 'db'), mk('ca1', '0192a1c4', 1, false, 15, 'db'), mk('ca2', '0192a1c4', 2, true, 14)];
    const b = [0, 1, 2, 3, 4, 5].map((n) => mk('cb' + n, '0192b7a1', n, false, 9, 'decode'));
    const okOnes = [3, 4, 5, 6].map((i) => mk('cok' + i, '0192ff0' + i, 0, true, i));
    return { tool: 'jaeger', url: 'localhost:16686/search', service: 'payment', tags: 'error=true', traces: a.concat(b, okOnes) };
  }

  sc.push({
    id: 'C', title: 'Bir mesaj işlenirken hata oluşuyor', tools: ['Grafana', 'RabbitMQ UI', 'Jaeger', 'Log'],
    stages: [
      { phase: 'durum', title: 'Durum: sistem çoğunlukla sağlıklı', body: '<p>Payment consumer’ı dakikada yüzlerce ödeme isteğini sorunsuz işliyor (<code>payment.requests</code>). Fakat bu sabah birkaç mesajda işleme hata veriyor.</p>' },
      { phase: 'belirti', title: 'Belirti: iki uyarı birden', body: '<p>Prometheus ▸ Alerts: <code>HandlerRetriesRising</code> (retried hızı &gt; 0,1/sn, 5 dk) ve <code>DeadLetterQueueNotEmpty</code> (DLQ’da mesaj var, 1 dk). Müşteri şikâyeti yok henüz; yalnızca birkaç sipariş ödeme bekliyor.</p><p class="muted">Rovena’da Alertmanager yok: uyarılar Prometheus’un Alerts sayfasında ve Grafana’da görünür, kimseye bildirim gitmez.</p>' },
      { phase: 'ilk', title: 'İlk kontrol', choice: { q: 'Mesajlar işlenirken hata. Önce nereye bakarsın?', opts: [
        { t: 'Grafana ▸ Messaging satırı (events consumed by result, queue depth)', ok: true, fb: 'Kaç mesaj, hangi kuyrukta, hangi sonuçla (ok / retried / dead_lettered) ve ne zaman başladığı tek bakışta görünür.' },
        { t: 'RabbitMQ UI ▸ payment.requests.dlq', ok: true, fb: 'Doğrudan hatalı mesajın kendisini görmek için en hızlı yol. Ama “ne zaman başladı, kaç tane” için Grafana’yı da kullan.', view: rabbitC() },
        { t: 'Order servisinin HTTP hata oranı', ok: false, fb: 'Hata bir <b>consumer</b>’da; HTTP katmanında görünmez. Order’ın API’si hâlâ 201 döner.' },
        { t: 'Payment servisini yeniden başlat', ok: false, fb: 'Mesajlar bozuksa yeniden başlatma onları düzeltmez; ayrıca nedenin izini silersin.' },
      ] } },
      { phase: 'inceleme', title: 'Grafana: tüketim sonuçları', body: '<p><code>messaging_events_consumed_total</code> bir <b>counter</b>; <code>result</code> etiketi üç değer alır: <code>ok</code>, <code>retried</code> (yeniden denenecek), <code>dead_lettered</code> (DLQ’ya gitti).</p>', view: dashC(false),
        qa: { look: 'Sonuç etiketine göre saniyede tüketilen olay ve kuyruk derinlikleri.', source: 'Sayaç Payment’ın broker katmanından; kuyruk derinliği RabbitMQ’dan.', meaning: 'ok sürerken retried ve dead_lettered’ın sıfırdan sapması “bazı mesajlar hata veriyor”; tüm tüketim çökmemiş. DLQ derinliği yalnızca artar: kimse tüketmiyor.', next: 'Hangi mesajlar, neden?' },
        task: { text: '“Events consumed per second, by result” ve “Queue depth” panellerine tıkla; retried ne zaman başladı, DLQ kaça çıktı?', req: ['panel:consumed', 'panel:depth'] } },
      { phase: 'inceleme', title: 'Log mu, metrik mi?', choice: { q: '“Hangi event_id, hangi hata mesajıyla, kaç kez denendi?” sorusunun cevabı nerede?', opts: [
        { t: 'Metrikte: events_consumed_total etiketleri', ok: false, fb: 'Metrik <b>kaç</b> ve <b>hangi kuyruk/tip/sonuç</b> sorularına cevap verir. Yüksek kardinalite (event_id) etiketi olamaz: Prometheus belleği sınırsız büyür.' },
        { t: 'Loglarda ve Jaeger’daki span’de (event_id, trace_id, hata metni)', ok: true, view: logsC(), fb: 'Log/trace tek <b>olay</b>ın ayrıntısını tutar: event_id, hata metni, retry sayısı. Metrik seni <i>nereye</i> bakacağına yönlendirir, log/trace <i>ne olduğunu</i> söyler.' },
        { t: 'RabbitMQ Queues sayılarında', ok: false, fb: 'Sayılar hatalı mesajların sayısını verir, hata nedenini değil (DLQ’daki mesaj başlığında x-error vardır, ama o da tek mesajın nedeni).' },
      ], explain: '<p><b>Metrik</b> = toplu, ucuz, uyarı için ideal. <b>Log</b> = tek olayın metni. <b>Trace</b> = tek akışın zaman çizgisi. Üçü <code>trace_id</code> ile bağlanır.</p>' } },
      { phase: 'inceleme', title: 'RabbitMQ: DLQ’daki mesajı oku', body: '<p>DLQ’ya düşen mesajda gerçek hata ve payload saklıdır: <code>x-error</code>, <code>x-failed-at</code>, <code>x-retry-count</code>. Management UI’da <b>Queues ▸ payment.requests.dlq ▸ Get messages</b>.</p><p class="muted">Dikkat: “Ack mode”u <i>Nack message requeue true</i> yap; “Automatic ack” mesajı kuyruktan <b>siler</b>.</p>', view: rabbitC(),
        qa: { look: 'DLQ’daki üç mesaj ve başlıkları.', source: 'Doğrudan RabbitMQ; başlıkları Payment’ın broker katmanı yazmış.', meaning: 'x-retry-count = MaxRetries’a ulaşmış (5): her denemede başarısız. Payload’daki order_id UUID değil: veri hatası, tekrar denemek işe yaramaz.', next: 'Aynı olaylar Jaeger’da nasıl görünüyor?' },
        task: { text: 'payment.requests.dlq’yu aç ve “Get Message(s)”e bas.', req: ['get-messages:payment.requests.dlq'], hint: 'Tablodan DLQ satırına tıkla, aşağıda Get Message(s).' } },
      { phase: 'inceleme', title: 'Jaeger: aynı event’in tüm denemeleri', body: '<p>Her deneme ayrı bir trace: <code>process payment.requests</code> span’i, etiketlerinde <code>event_id</code> ve <code>messaging.retry_count</code>. Mesaj <b>aynı trace_id’ye</b> bağlı kalır. Filtre: Service = payment, Tags = <code>error=true</code>. İki farklı event’in izini karşılaştır.</p>', view: jaegerC(),
        task: { text: '<b>0192a1c4</b> olayından (ca…) bir span ve <b>0192b7a1</b> olayından (cb…) bir span aç; retry_count ve hata metnine bak.', req: [/^span:ca\d$/, /^span:cb\d$/], hint: 'Başlık “Trace c-ca0 …” gibi görünür; kök span’e tıkla. Ayrıntıda event_id ve retry_count var.' } },
      { phase: 'teshis', title: 'Teşhis: geçici mi, kalıcı mı?', choice: { q: 'İki olayın deneme geçmişi: 0192a1c4 (2 hata, sonra başarı), 0192b7a1 (6 deneme, hepsi hata, sonra DLQ). Sonuç?', opts: [
        { t: '0192a1c4 geçici (altyapı: DB bağlantısı), 0192b7a1 kalıcı (veri/kod: geçersiz order_id)', ok: true, fb: 'Geçici hata <b>kendiliğinden</b> düzelir ve farklı deneme farklı sonuç verir. Kalıcı hata aynı girdiyle <b>her denemede aynı</b> hatayı verir: bekleme işe yaramaz.' },
        { t: 'İkisi de geçici, retry sayısını artırmak çözer', ok: false, fb: 'Aynı bozuk girdiyle 20 deneme de aynı hatayı verir; sadece gecikme ve yük üretirsin.' },
        { t: 'İkisi de kalıcı, DLQ’ya hemen göndermeliyiz', ok: false, fb: '0192a1c4 üçüncü denemede <b>başarılı</b> oldu; retry mekanizmasının amacı tam olarak buydu.' },
      ], explain: '<p>Değerlendirme soruları: (1) Aynı mesaj tekrar denenince sonuç değişiyor mu? (2) Hata metni altyapıyı (connection refused, timeout) mi, veriyi/kodu mu (decode, validation) gösteriyor? (3) Başka mesajlar etkileniyor mu?</p>' } },
      { phase: 'teshis', title: 'Teşhis: tekrar tekrar işleme riski', choice: { q: 'Bir mesaj retry ile birden fazla kez işlenirse (ya da ack yolunda bağlantı kopup yeniden teslim edilirse) ne olur?', opts: [
        { t: 'RabbitMQ “tam bir kez” teslim garantisi verir, sorun olmaz', ok: false, fb: 'Garanti <b>en az bir kez</b> (at-least-once). Tam bir kez teslim dağıtık sistemlerde pratikte yoktur.' },
        { t: 'Handler idempotent olmalı: aynı olay iki kez gelse de etkisi tek (örn. Payment ödemenin idempotency key’ine bakar)', ok: true, fb: 'Doğru. Rovena’da Payment, ödemenin idempotency key’i ile aynı ödemeyi iki kez çekmeyi önler; olay zarfındaki <code>event_id</code> de tekrar tespitinde kullanılır (inbox/<code>processed_events</code>).' },
        { t: 'Mesaj kaybolur, o yüzden retry tehlikelidir', ok: false, fb: 'Broker mesajı önce retry/DLQ kuyruğuna yayınlar, ancak onaylandıktan sonra orijinali ack eder; en kötü durumda <b>iki kopya</b> olur, kaybolmaz. Bu yüzden idempotency şart.' },
      ] } },
      { phase: 'aksiyon', title: 'Aksiyon', choice: { q: 'DLQ’daki üç mesajın nedeni üretici tarafındaki geçersiz order_id. Ne yaparsın?', opts: [
        { t: 'Üreticideki hatayı düzelt, etkilenen siparişlerin durumunu kontrol et, düzeltilmiş olayı kontrollü şekilde yeniden yayınla (ya da siparişi kurtarma işine bırak)', ok: true, fb: 'Önce kök neden (kötü payload’ı kim üretti?), sonra veri kurtarma. DLQ’daki mesaj kanıttır; işi bitmeden silme.' },
        { t: 'DLQ’yu purge et, uyarı kapansın', ok: false, fb: 'Uyarıyı susturursun, sorunu değil; üstelik mesajlar ödeme istekleri.' },
        { t: 'MaxRetries’ı 50 yap', ok: false, fb: 'Kalıcı hata için fayda sağlamaz; yalnızca gecikme ve yük ekler.' },
      ] }, code: [{ title: 'Kalıcı hatayı tekrar denemeden DLQ’ya göndermek (örnek)', lang: 'c-ok', body: '// Tekrar denemenin faydası olmayan (geçersiz veri) hatalar:\nif err := env.DecodePayload(&p); err != nil {\n    return messaging.Permanent(err) // retry kuyruğuna girmeden doğrudan DLQ\n}\n// Geçici hatalar (DB/ağ) ise düz error dönmeli: retry hakkı kullanılır.', explain: 'shared/messaging bu ayrımı destekler (<code>messaging.Permanent</code>). Hangi hatanın hangisi sayılacağı handler yazarının kararıdır.' }] },
      { phase: 'dogrulama', title: 'Doğrulama', view: () => dashC(true), body: '<p>Düzeltme yayında; son 15 dk görünümü.</p>', choice: { q: 'Hangisi normale döndüğünü gösterir?', opts: [
        { t: 'retried ve dead_lettered hızı sıfıra döndü; ok hızı sürüyor; DLQ derinliği artmıyor (mesajlar yeniden yayınlandı ve işlendi)', ok: true, fb: 'DLQ sayısı kendiliğinden düşmez: kimse tüketmiyor. Asıl işaret, yeni hata gelmemesi ve yeniden yayınlanan mesajın trace’inde error olmaması.' },
        { t: 'DLQ derinliği kendiliğinden 0’a düştü', ok: false, fb: 'Düşmez; otomatik tüketici yok. Düşüş olsa bile purge/yeniden yayınlamayla olur.' },
        { t: 'HandlerRetriesRising uyarısı yok', ok: false, fb: 'Uyarı <code>for: 5m</code> ile gecikmeli kapanır; tek başına yeterli kanıt değil.' },
      ] } },
    ],
    summary: '<table><tr><th>Soru</th><th>Kaynak</th></tr><tr><td>Kaç mesaj, hangi kuyrukta, ne zaman başladı?</td><td>Grafana / Prometheus: <code>messaging_events_consumed_total{result}</code></td></tr><tr><td>Hangi event, hangi hata, kaç deneme?</td><td>Log (<code>event_id</code>, <code>trace_id</code>) + Jaeger span etiketleri</td></tr><tr><td>Hatalı mesajın kendisi / başlıkları</td><td>RabbitMQ ▸ DLQ ▸ Get messages (<code>x-error</code>)</td></tr></table><p><b>Geçici mi kalıcı mı?</b> Aynı mesajın denemeleri arasında sonuç değişiyor mu, hata altyapı mı veri mi?</p>',
  });

  /* =========================================================================================
   * D: Hata oranı aniden yükseldi
   * ========================================================================================= */
  function dashD(fixed) {
    const rec = 0.972, dep = 0.92;
    const err = (peak, seed) => g((t) => (t < dep ? 0 : fixed && t > rec ? 0 : sh(t, dep, 0, peak, 0.01)), peak * 0.05, seed);
    return gf([row('Traffic and errors', [
      panel('rps', 'Requests per second, by service', EX.rps, [S('order', flat(3.2, 0.3, 1), '#5ab0ff'), S('catalog', flat(6.1, 0.5, 2), '#ffb454'), S('inventory', flat(3.5, 0.3, 3), '#7ee787')], { markers: [{ at: dep, label: 'deploy' }] }),
      panel('err', '5xx responses per second', EX.err, [S('catalog', err(1.9, 4), '#ffb454'), S('order', err(1.9, 5), '#5ab0ff'), S('inventory', flat(0, 0, 6), '#7ee787')], { markers: [{ at: dep, label: 'deploy' }] }),
      panel('p95route', 'p95 latency, by route', EX.p95, [S('order /api/v1/checkout', g((t) => (t < dep ? 0.42 : fixed && t > rec ? 0.42 : 0.3), 0.02, 7), '#5ab0ff'), S('catalog …/variants/lookup', g((t) => (t < dep ? 0.03 : fixed && t > rec ? 0.03 : 0.02), 0.004, 8), '#ffb454')], { unit: 's', thr: [{ y: 1, label: 'SlowRequests > 1s' }] }),
    ])], { range: 60 });
  }
  const promD = () => ({
    tool: 'prom', url: 'localhost:9090/graph', queries: [
      { id: 'up', expr: 'up', rows: ['identity', 'catalog', 'inventory', 'payment', 'order', 'rabbitmq'].map((j) => ({ labels: { job: j }, value: '1' })), verdict: { type: 'meh', text: 'Tüm hedefler ayakta: bu bir <b>kesinti değil</b>. up yalnızca /metrics’in cevap verip vermediğini söyler; servis cevap verip 5xx dönüyor olabilir (ServiceDown tetiklenmez).' } },
      { id: 'ratio', expr: 'sum by (job) (rate(http_server_request_duration_seconds_count{http_response_status_code=~"5.."}[5m])) / sum by (job) (rate(http_server_request_duration_seconds_count[5m]))', rows: [{ labels: { job: 'order' }, value: '0.59', cls: 'bad-t' }, { labels: { job: 'catalog' }, value: '0.31', cls: 'bad-t' }, { labels: { job: 'inventory' }, value: '0' }, { labels: { job: 'payment' }, value: '0' }, { labels: { job: 'identity' }, value: '0' }], verdict: { type: 'good', text: 'Alarm kuralı da bu oranı kullanır (&gt;0,05). Hata yalnızca <b>order</b> ve <b>catalog</b>’da; inventory/payment/identity temiz: arıza sistem geneli değil, iki servis arasında.' } },
      { id: 'route', expr: 'sum by (job, http_route, http_response_status_code) (rate(http_server_request_duration_seconds_count{http_response_status_code=~"5.."}[5m]))', rows: [{ labels: { job: 'catalog', http_route: '/api/v1/internal/variants/lookup', http_response_status_code: '500' }, value: '1.91 /s', cls: 'bad-t' }, { labels: { job: 'order', http_route: '/api/v1/checkout', http_response_status_code: '500' }, value: '1.90 /s', cls: 'bad-t' }], verdict: { type: 'good', text: 'Tek bir catalog route’u hata veriyor ve <b>aynı hızda</b> order checkout hatası var (1,91 ≈ 1,90): Order’ın hatası, Catalog hatasının <i>yansıması</i>. Kök neden Catalog’da.' } },
      { id: 'inc', expr: 'sum by (job) (increase(http_server_request_duration_seconds_count{http_response_status_code=~"5.."}[15m]))', rows: [{ labels: { job: 'catalog' }, value: '1716', cls: 'warn-t' }, { labels: { job: 'order' }, value: '1710', cls: 'warn-t' }], verdict: { type: 'meh', text: '<code>increase()</code> pencerede TOPLAM artış verir (son 15 dk’da ≈1.7k başarısız istek: “kaç müşteri etkilendi”). <code>rate()</code> aynı bilgiyi saniye başına verir. Hangisini seçtiğin soruya bağlı.' } },
    ],
  });
  function jaegerD() {
    const okTrace = (i, ago) => ({ id: 'd-ok' + i, service: 'order', services: ['order', 'catalog', 'inventory'], op: 'POST /api/v1/checkout', dur: 210 + i * 6, ago, spans: [
      { id: `d-o${i}`, svc: 'order', name: 'POST /api/v1/checkout', start: 0, dur: 210 + i * 6, tags: { 'http.status_code': 201 } },
      { id: `d-o${i}c`, parent: `d-o${i}`, svc: 'order', name: 'POST catalog', start: 8, dur: 38 },
      { id: `d-o${i}cs`, parent: `d-o${i}c`, svc: 'catalog', name: 'POST /api/v1/internal/variants/lookup', start: 12, dur: 31, tags: { 'http.status_code': 200 } },
      { id: `d-o${i}cd`, parent: `d-o${i}cs`, svc: 'catalog', name: 'db: SELECT variants', start: 15, dur: 24 },
      { id: `d-o${i}i`, parent: `d-o${i}`, svc: 'order', name: 'POST inventory', start: 60, dur: 74 },
    ] });
    const errTrace = (i, ago) => ({ id: 'd-er' + i, service: 'order', services: ['order', 'catalog'], op: 'POST /api/v1/checkout', dur: 48, ago, err: true, spans: [
      { id: `d-e${i}-root`, svc: 'order', name: 'POST /api/v1/checkout', start: 0, dur: 48, err: true, tags: { 'http.status_code': 500 } },
      { id: `d-e${i}-cat`, parent: `d-e${i}-root`, svc: 'order', name: 'POST catalog', start: 6, dur: 30, err: true, tags: { 'http.status_code': 500 } },
      { id: `d-e${i}-cats`, parent: `d-e${i}-cat`, svc: 'catalog', name: 'POST /api/v1/internal/variants/lookup', start: 9, dur: 25, err: true, tags: { 'http.status_code': 500 } },
      { id: `d-e${i}-db`, parent: `d-e${i}-cats`, svc: 'catalog', name: 'db: SELECT variants', start: 12, dur: 19, err: true, tags: { 'db.system': 'postgresql' }, logs: ['ERROR: column v.sku_code does not exist (SQLSTATE 42703)'] },
    ] });
    return { tool: 'jaeger', url: 'localhost:16686/search', service: 'catalog', tags: 'error=true', traces: [0, 1, 2, 3, 4, 5].map((i) => errTrace(i, 1 + i)).concat([0, 1, 2, 3].map((i) => okTrace(i, 24 + i * 4))) };
  }

  sc.push({
    id: 'D', title: 'Hata oranı aniden yükseldi', tools: ['Grafana', 'Prometheus', 'Jaeger'],
    stages: [
      { phase: 'durum', title: 'Durum: Cuma 16:20', body: '<p>Bir süre önce bir dağıtım yapıldı (kimse hangi servis olduğundan emin değil). Müşteriler ödeme sayfasında hata görüyor.</p>' },
      { phase: 'belirti', title: 'Belirti: mobil uygulamada “bir hata oluştu”', body: '<p>Destek yoğun. Prometheus Alerts’te <code>HighServerErrorRate</code> (5xx oranı &gt;%5, 5 dk) <b>firing</b>. <code>SlowRequests</code> ise sessiz: yavaşlık yok, hata var.</p>' },
      { phase: 'ilk', title: 'İlk kontrol', choice: { q: 'Hata oranı arttı. Nereden başlarsın?', opts: [
        { t: 'Grafana: hata oranı, istek hızı ve gecikme birlikte', ok: true, fb: 'Üçlü birlikte bakılınca hatanın yükle mi, yavaşlıkla mı, dağıtımla mı ilişkili olduğu görülür; panelde deploy anı da işaretlenebilir.' },
        { t: 'RabbitMQ Management UI', ok: false, fb: 'Hata HTTP katmanında (5xx). Kuyruklar bu sorunun ilk şüphelisi değil.' },
        { t: 'Önce Jaeger’da hatalı trace ara', ok: false, fb: 'Mümkün ama hangi serviste arayacağını bilmiyorsun. Önce metrikle “hangi servis/route” daraltılır.' },
      ] } },
      { phase: 'inceleme', title: 'Grafana: error rate, request rate, latency', body: '<p>Üç sinyali birlikte yorumla. İstek hızı sabit, hata arttı, gecikme <i>düştü</i>. Neden?</p>', view: dashD(false),
        qa: { look: 'İstek hızı, 5xx hızı ve route gecikmesi; mor çizgi deploy anı.', source: 'Prometheus (10 sn scrape); 5xx paneli http_response_status_code=~"5.." filtresi uygulayan bir sorgu.', meaning: 'İstek hızı sabit ⇒ hata yük kaynaklı değil. 5xx deploy’dan hemen sonra başladı ⇒ değişiklikle ilişkili. Gecikme düştü çünkü hata <b>hızlı</b> dönüyor: bu yüzden SlowRequests susar.', next: 'Hangi servis ve route?' },
        task: { text: '“5xx responses per second” ve “Requests per second” panellerine tıkla; hatanın başladığı an ile deploy işareti arasındaki ilişkiye bak.', req: ['panel:err', 'panel:rps'] } },
      { phase: 'inceleme', title: 'Prometheus: sorunu daralt', body: '<p>Aşağıdaki dört sorgudan <b>hata oranını servis başına</b> ve <b>route başına</b> veren ikisini çalıştır. Diğer ikisi de öğretici.</p>', view: promD(),
        task: { text: 'Servis başına hata oranını (ratio) ve route/status başına 5xx hızını (route) çalıştır.', req: ['query:ratio', 'query:route'], hint: 'İlki iki sorgunun oranı (5xx ÷ tüm istekler); ikincisi sum by (job, http_route, http_response_status_code).' } },
      { phase: 'inceleme', title: 'Jaeger: hatalı ve sağlıklı trace’i karşılaştır', body: '<p>Service = <b>catalog</b>, Tags = <code>error=true</code> ile hatalı trace’leri bul, bir hata span’ini aç. Sonra <b>Tags kutusunu temizleyip</b> Find Traces de ve dağıtımdan önceki bir sağlıklı trace’i aç. Fark nedir?</p>', view: jaegerD(),
        task: { text: 'Bir hatalı trace’in veritabanı span’ini (db: SELECT variants) aç, sonra Tags’i temizleyip sağlıklı bir trace aç.', req: [/^span:d-e\d-db$/, /^trace:d-ok/], hint: 'Hatalı trace id’leri d-er…, sağlıklı olanlar d-ok… ile başlar.' } },
      { phase: 'teshis', title: 'Teşhis', choice: { q: 'Bulgular: catalog /internal/variants/lookup 500; order checkout 500 (aynı hızda); span’de “column v.sku_code does not exist”. Sonuç?', opts: [
        { t: 'Catalog’a ait yeni sürüm şema ile uyumsuz (kod yeni kolonu okuyor, migration henüz uygulanmamış); Order sadece yansıtıyor', ok: true, fb: 'Hata tek serviste (Catalog) ve tek route’ta başlıyor; ikisinin hata hızı aynı çünkü Order’ın her başarısız checkout’u bir başarısız Catalog çağrısına karşılık geliyor.' },
        { t: 'Order servisinin kodunda bug var', ok: false, fb: 'Order’ın span’i, alt çağrıdan gelen hatayı iletiyor; asıl hata Catalog’un DB span’inde.' },
        { t: 'Gateway (Nginx) bozuk', ok: false, fb: 'Gateway’de hata olsaydı hata metni ve span’ler Catalog DB’sine uzanmazdı; ayrıca diğer servisler (inventory, identity) temiz.' },
        { t: 'Veritabanı çökmüş', ok: false, fb: 'Çökmüş olsaydı tüm Catalog route’ları ve <code>up</code> etkilenirdi; sorgu yalnızca belirli bir kolon yüzünden hata veriyor.' },
      ] } },
      { phase: 'aksiyon', title: 'Aksiyon', choice: { q: 'Müşteriler şu an hata alıyor. Önceliğin?', opts: [
        { t: 'Önce etkiyi durdur: Catalog’u önceki sürüme al ya da eksik migration’ı uygula; sonra “neden migrate, deploy’dan sonra çalıştı?” incele', ok: true, fb: 'Önce kanamayı durdur, sonra kök neden. Compose’ta migrate servisi uygulamadan önce tamamlanmalıdır (<code>depends_on: service_completed_successfully</code>); dağıtım sırası kontrol edilir.' },
        { t: 'Order’a otomatik retry ekle', ok: false, fb: 'Kalıcı hatayı tekrar denemek yükü artırır, hatayı çözmez ve gerçek sorunu gizler.' },
        { t: 'HighServerErrorRate eşiğini %5’ten %60’a çıkar', ok: false, fb: 'Alarmı susturmak sorunu çözmez.' },
      ] }, code: [{ title: 'Kontrol listesi', lang: 'c-ok', body: 'docker compose ps catalog-migrate catalog      # migrate "exited 0" mı?\ndocker compose logs catalog-migrate | tail     # hangi sürüm uygulanmış?\n# geri alma: önceki imaj etiketine dön, sonra migration sırasını düzelt' }] },
      { phase: 'dogrulama', title: 'Doğrulama', view: () => Object.assign(dashD(true), { range: 15 }), body: '<p>Geri alma yapıldı. Aşağıda son 15 dk.</p>', choice: { q: 'Normale döndüğünü söyleyebilmek için ne gerekir?', opts: [
        { t: '5xx oranı birkaç dakika boyunca ≈0 kalıyor, yeni checkout trace’leri hatasız, uyarı pending/resolved', ok: true, fb: '<code>rate(...[5m])</code> pencere kullanır: eski hatalar birkaç dakika sorguda kalır. Alarm da <code>for: 5m</code> ile gecikmeli kapanır. Tek bir anlık sıfır yetmez.' },
        { t: 'Grafikte tek bir nokta sıfır göründü', ok: false, fb: 'Gürültü olabilir; sürdürülebilir olduğunu birkaç dakika izle.' },
        { t: 'Deploy aracı “başarılı” dedi', ok: false, fb: 'Araç dağıtımın bittiğini söyler; sağlığı değil.' },
      ] } },
    ],
    summary: '<table><tr><th>Soru</th><th>Araç</th></tr><tr><td>Hata yükle mi, yavaşlıkla mı, değişiklikle mi ilişkili?</td><td>Grafana: rps + 5xx + latency birlikte, deploy işareti</td></tr><tr><td>Hangi servis / route?</td><td>Prometheus: <code>sum by (job, http_route) (rate(…5xx…))</code> ve oran sorgusu</td></tr><tr><td>Neden?</td><td>Jaeger: hatalı vs sağlıklı trace; hata span’i ve log</td></tr></table><p><b>İpucu:</b> iki serviste aynı hata hızı çoğunlukla “biri diğerinin yansıması” demektir: kök, en altta hata veren servistir.</p>',
  });

  /* =========================================================================================
   * E: Servis çalışıyor ama işler tamamlanmıyor
   * ========================================================================================= */
  function dashE(fixed) {
    const rec = 0.962;
    const pend = g((t) => (t < T0 ? 0 : fixed && t >= rec ? Math.max(0, 1042 * (1 - (t - rec) / 0.01)) : Math.min(1042, ((t - T0) / (rec - T0)) * 1042)), 3, 1);
    const pendBroken = g((t) => (t < T0 ? 0 : Math.min(1042, ((t - T0) / (1 - T0)) * 1042)), 3, 1);
    const P = fixed ? pend : pendBroken;
    const age = g((t) => (t < T0 ? 0.4 : fixed && t >= rec ? Math.max(0.4, 348 * (1 - (t - rec) / 0.008)) : Math.min(348, ((t - T0) / (rec - T0)) * 348)), 1, 2);
    const ageB = g((t) => (t < T0 ? 0.4 : Math.min(348, ((t - T0) / (1 - T0)) * 348)), 1, 2);
    return gf([row('Traffic and errors', [
      panel('rps', 'Requests per second, by service', EX.rps, [S('order', flat(3.2, 0.3, 1), '#5ab0ff'), S('catalog', flat(6.1, 0.5, 2), '#ffb454'), S('inventory', flat(3.5, 0.3, 3), '#7ee787')]),
      panel('err', '5xx responses per second', EX.err, [S('order', flat(0, 0, 1), '#5ab0ff')], { yMax: 1 }),
      panel('p95route', 'p95 latency, by route', EX.p95, [S('order /api/v1/checkout', flat(0.21, 0.02, 4), '#5ab0ff')], { unit: 's' }),
    ]), row('Messaging', [
      panel('consumed', 'Events consumed per second, by result', EX.consumed, [S('payment.requests ok', g((t) => (t < T0 ? 3.2 : fixed && t >= rec ? (t < rec + 0.02 ? 12 : 3.2) : 0), 0.15, 5), '#7ee787'), S('payment.requests retried', flat(0), '#ffb454')]),
      panel('depth', 'Queue depth (messages waiting)', EX.depth, [S('payment.requests', flat(0, 0, 6), '#ffb454'), S('order.payment-results', flat(0, 0, 7), '#5ab0ff')], { yMax: 10 }),
      panel('outbox', 'Outbox: events waiting to be published', EX.outbox, [S('order', fixed ? P : pendBroken, '#ff7b72'), S('payment', flat(0, 0, 9), '#7ee787')], { thr: [{ y: 20, label: 'OutboxBacklog > 20' }] }),
      panel('age', 'Outbox: age of the oldest waiting event', EX.age, [S('order', fixed ? age : ageB, '#ff7b72')], { unit: 's', thr: [{ y: 120, label: 'OutboxEventStuck > 120s' }] }),
      panel('published', 'Events published per second, by type', EX.published, [S('PaymentRequested ok', g((t) => (t < T0 ? 3.2 : fixed && t >= rec ? 3.2 : 0), 0.2, 10), '#7ee787'), S('PaymentRequested error', g((t) => (t < T0 ? 0 : fixed && t >= rec ? 0 : 0.5), 0.05, 11), '#ff7b72')]),
    ]), row('Orders and payments', [
      panel('recovery', 'Recovery job actions per 5 minutes', 'sum by (action) (increase(saga_recovery_actions_total[5m]))', [S('payment_requested_again', g((t) => (t < T0 + 0.01 ? 0 : fixed && t >= rec ? 0 : 6), 0.5, 12), '#f2cc60')]),
    ])], { range: 60 });
  }
  const promE = () => ({
    tool: 'prom', url: 'localhost:9090/graph', queries: [
      { id: 'depth', expr: 'rabbitmq_detailed_queue_messages_ready{queue="payment.requests"}', rows: [{ labels: { queue: 'payment.requests' }, value: '0' }], verdict: { type: 'meh', text: '<b>Boş kuyruk ≠ sağlıklı akış.</b> Mesajlar hiç GELMİYOR olabilir. Bu sorgu yalnızca RabbitMQ’ya ulaşan mesajları görür.' } },
      { id: 'outbox', expr: 'outbox_pending_events', rows: [{ labels: { job: 'order' }, value: '1042', cls: 'bad-t' }, { labels: { job: 'payment' }, value: '0' }], verdict: { type: 'good', text: 'Order’ın outbox’ında 1.042 olay yayınlanmayı bekliyor. Olaylar yazıldı (iş kaybolmadı) ama RabbitMQ’ya gidemiyor. OutboxBacklog (&gt;20) uyarısı bu yüzden.' } },
      { id: 'age', expr: 'outbox_oldest_pending_age_seconds', rows: [{ labels: { job: 'order' }, value: '348 s', cls: 'bad-t' }], verdict: { type: 'good', text: 'En eski olay 5,8 dakikadır bekliyor: “sayı büyüyor” ile “takılı kaldı” arasındaki farkı yaş verir (OutboxEventStuck &gt;120 s, kritik).' } },
      { id: 'pubErr', expr: 'sum by (type, result) (rate(messaging_events_published_total[5m]))', rows: [{ labels: { type: 'PaymentRequested', result: 'ok' }, value: '0 /s' }, { labels: { type: 'PaymentRequested', result: 'error' }, value: '0.52 /s', cls: 'bad-t' }], verdict: { type: 'good', text: 'Başarılı yayın sıfır, hata hızı 0,52/sn: relay her turda yayınlamayı deniyor ve broker’a ulaşamıyor.' } },
      { id: 'rec', expr: 'sum by (action) (increase(saga_recovery_actions_total[15m]))', rows: [{ labels: { action: 'payment_requested_again' }, value: '31', cls: 'warn-t' }], verdict: { type: 'meh', text: 'Kurtarma işi “ödeme cevabı gelmedi” diye PaymentRequested’i tekrar istiyor (en çok 3 kez). Yeni olay da aynı outbox yolundan geçeceği için kök nedeni çözmez; sadece belirti.' } },
    ],
  });
  function jaegerE() {
    const mk = (i, ago) => ({ id: 'e-tr' + i, service: 'order', services: ['order', 'catalog', 'inventory'], op: 'POST /api/v1/checkout', dur: 3200, ago, err: true, spans: [
      { id: `e-root${i}`, svc: 'order', name: 'POST /api/v1/checkout', start: 0, dur: 214, tags: { 'http.status_code': 201 } },
      { id: `e-cat${i}`, parent: `e-root${i}`, svc: 'order', name: 'POST catalog', start: 8, dur: 38 },
      { id: `e-inv${i}`, parent: `e-root${i}`, svc: 'order', name: 'POST inventory', start: 60, dur: 74 },
      { id: `e-db${i}`, parent: `e-root${i}`, svc: 'order', name: 'db: INSERT orders / outbox_events', start: 190, dur: 20 },
      ...[0, 1, 2].map((k) => ({ id: `e-pub${i}-${k}`, parent: `e-root${i}`, svc: 'order', name: 'publish PaymentRequested', start: 700 + k * 1250, dur: 5, err: true, async: true, tags: { 'messaging.exchange': 'rovena.events', 'messaging.routing_key': 'PaymentRequested' }, logs: ['dial tcp 172.18.0.3:5672: connect: connection refused'] })),
    ] });
    return { tool: 'jaeger', url: 'localhost:16686/search', service: 'order', traces: [0, 1, 2, 3, 4].map((i) => mk(i, 1 + i * 2)), note: 'Dikkat: kök span 201 ile başarılı. Trace’in “error” rozeti, sonradan çalışan asenkron span’lerden geliyor.' };
  }
  const rabbitE = (fixed) => ({
    tool: 'rabbit', url: 'localhost:15672/#/queues', tab: 'queues',
    queues: [
      rq({ name: 'order.payment-results', ready: 0, consumers: fixed ? 1 : 0, state: fixed ? 'running' : 'idle', incoming: fixed ? 3.2 : 0, deliver: fixed ? 3.2 : 0, ack: fixed ? 3.2 : 0 }), rq({ name: 'order.payment-results.dlq' }), rq({ name: 'order.payment-results.retry' }),
      rq({ name: 'payment.requests', state: fixed ? 'running' : 'idle', ready: 0, consumers: 1, incoming: fixed ? 3.2 : 0, deliver: fixed ? 3.2 : 0, ack: fixed ? 3.2 : 0 }), rq({ name: 'payment.requests.dlq' }), rq({ name: 'payment.requests.retry' }),
    ],
    overview: { ready: 0, unacked: 0, readyHist: flat(0), unackedHist: flat(0), publish: g((t) => (t < T0 ? 3.2 : 0), 0.2, 1), deliver: g((t) => (t < T0 ? 3.2 : 0), 0.2, 2), ack: g((t) => (t < T0 ? 3.2 : 0), 0.2, 3), caption: 'Publish, deliver ve ack aynı anda sıfıra düştü: kuyruk “sağlıklı” görünür ama hiç trafik yok.' },
    connections: fixed
      ? [{ name: '172.18.0.7:50221 → 172.18.0.3:5672  (order)', user: 'rovena', state: 'running', channels: 2 }, { name: '172.18.0.9:51340 → 172.18.0.3:5672  (payment)', user: 'rovena', state: 'running', channels: 2 }]
      : [{ name: '172.18.0.9:51340 → 172.18.0.3:5672  (payment)', user: 'rovena', state: 'running', channels: 2 }],
    channels: fixed ? [{ name: '172.18.0.7:50221 (1)', prefetch: 1, unacked: 0, consumers: 1 }, { name: '172.18.0.9:51340 (1)', prefetch: 1, unacked: 0, consumers: 1 }] : [{ name: '172.18.0.9:51340 (1)', prefetch: 1, unacked: 0, consumers: 1 }],
    exchanges: [{ name: 'rovena.events', type: 'topic', features: 'D' }],
    bindings: [['PaymentRequested', 'payment.requests'], ['RefundRequested', 'payment.requests'], ['PaymentSucceeded', 'order.payment-results'], ['PaymentFailed', 'order.payment-results'], ['PaymentRefunded', 'order.payment-results']],
    queueHint: 'Dikkat çekici bir şey var mı? Consumers sütununa bak.',
  });

  sc.push({
    id: 'E', title: 'Servis çalışıyor ama işler tamamlanmıyor', tools: ['Grafana', 'Prometheus', 'RabbitMQ UI', 'Jaeger'],
    stages: [
      { phase: 'durum', title: 'Durum: sessiz bir akşam', body: '<p>Kampanya yok, trafik normal, hiçbir servis çökmedi. Müşteriler sipariş vermeye devam ediyor.</p>' },
      { phase: 'belirti', title: 'Belirti: HTTP yeşil, siparişler bekliyor', body: '<p>Müşteri: <i>“Ödeme yaptım ama siparişim hâlâ ‘Ödeme bekleniyor’.”</i> Order API’si <code>201</code> dönüyor, 5xx yok, gecikme normal. Hiçbir panel kırmızı değil.</p><div class="fb info"><b>Önemli:</b> HTTP yanıtının başarılı olması, <b>arka planda</b> yapılacak işin (ödeme isteği, tüketim, sipariş onayı) başarılı olduğu anlamına gelmez. İkisi farklı sinyallerle ölçülür.</div>' },
      { phase: 'ilk', title: 'İlk kontrol', choice: { q: 'Hiçbir şey kırmızı değil ama işler bitmiyor. Nereden başlarsın?', opts: [
        { t: 'Grafana: Rovena overview panosunun Messaging satırı (outbox, tüketim, kuyruk)', ok: true, fb: 'Mesajlaşma sağlığı (outbox birikimi, tüketim, kuyruk) tek panoda. HTTP satırı yeşildi; asıl bilgi alt satırlarda.' },
        { t: 'RabbitMQ Management ▸ Queues', ok: true, fb: 'Makul bir başlangıç, ama dikkatli oku: bu ekran yalnızca <b>broker’a ulaşan</b> mesajları gösterir. Boş kuyruk “sorun yok” demek değildir. (Aşağıdaki tabloda order.payment-results’ın Consumers sütununa bak.)', view: rabbitE(false) },
        { t: 'Sadece HTTP latency ve 5xx panelleri', ok: false, fb: 'Hepsi yeşil kalır ve seni “sorun yok” sonucuna götürür; HTTP yolu zaten sağlıklı. Eşzamanlı yol ile asenkron yolu ayrı ölçmen gerekir.' },
        { t: 'Servisleri yeniden başlat', ok: false, fb: 'Neden bilinmeden yeniden başlatma, outbox’taki olayların nerede takıldığını göstermez.' },
      ] } },
      { phase: 'inceleme', title: 'Grafana: Messaging satırında ne yanlış?', body: '<p>Tüm panolara bak: HTTP satırı yeşil. <b>Messaging</b> satırında hangi paneller normal dışı?</p>', view: dashE(false),
        qa: { look: 'Outbox panelleri, yayın sonuçları, tüketim ve kuyruk derinliği.', source: 'outbox_* metrikleri Order’ın veritabanını sorgulayan gauge’larından; messaging_* sayaçları broker katmanından; kuyruk derinliği RabbitMQ’dan.', meaning: 'Outbox yükseliyorsa olaylar yazılıyor ama yayınlanamıyor. Kuyruk derinliği <b>0 kalıyorsa</b> mesajlar RabbitMQ’ya hiç ulaşmıyor: boş kuyruk bir kanıt değildir.', next: 'Neden yayınlanamıyor?' },
        task: { text: '“Outbox: events waiting to be published” ve “Outbox: age of the oldest waiting event” panellerine tıkla.', req: ['panel:outbox', 'panel:age'], hint: 'Kırmızı çizgiler uyarı eşikleri (OutboxBacklog, OutboxEventStuck).' } },
      { phase: 'inceleme', title: 'Prometheus: birikim ve yayın hatası', body: '<p>Aynı bilgiyi doğrudan sor. Önce <i>kuyruğa</i> bakan sorgunun neden yanıltıcı olduğunu, sonra <i>outbox</i> ve <i>yayın hatası</i> sorgularının neyi gösterdiğini gör.</p>', view: promE(),
        task: { text: 'outbox_pending_events ve yayın sonucu (published_total, result) sorgularını çalıştır; kuyruk sorgusunu da dene.', req: ['query:outbox', 'query:pubErr', 'query:depth'] } },
      { phase: 'inceleme', title: 'Jaeger: checkout’un sonrası', body: '<p>Service = <b>order</b>, “Find Traces”. Bir checkout trace’ini aç. Kök span <b>201</b>, ama trace’in <i>error</i> rozeti var. Hangi span hatalı?</p>', view: jaegerE(),
        qa: { look: 'Checkout trace’i ve kök span’den SONRA çalışan asenkron span’ler (mor).', source: 'Olay outbox’ta traceparent ile bekler; relay yayınlarken span’i aynı trace’e bağlar.', meaning: 'Hatalı span <code>publish PaymentRequested</code>: istek başarılı, yayın başarısız. Üç deneme, aralıklı (relay döngüsü).', next: 'Hata metni broker’a ulaşılamadığını söylüyor mu?' },
        task: { text: 'Bir trace aç, hatalı “publish PaymentRequested” span’ine tıkla ve log satırını oku.', req: [/^span:e-pub/] } },
      { phase: 'inceleme', title: 'RabbitMQ: bağlantılar', body: '<p>Yayınlayan taraf bağlanabiliyor mu? <b>Connections / Channels</b> sekmesine geç: hangi servisler broker’a bağlı?</p>', view: rabbitE(false),
        task: { text: '“Connections / Channels” sekmesini aç.', req: ['tab:connections'] } },
      { phase: 'teshis', title: 'Teşhis', choice: { q: 'Bulgular: outbox 1.042 olay, en eskisi 348 sn; yayın hatası 0,52/s; “publish” span’inde connection refused; kuyruk boş; broker’a yalnızca Payment bağlı. Sonuç?', opts: [
        { t: 'Order’dan RabbitMQ’ya giden bağlantı kopuk. Olaylar outbox’ta güvende; bağlantı gelince yayınlanır', ok: true, fb: 'Outbox pattern’ının amacı bu: broker yokken iş <b>kaybolmaz</b>, sipariş ve olay aynı transaction’da yazıldığı için tutarlı kalır. Order’ın consumer’ı da bağlı olmadığı için order.payment-results’ta Consumers = 0.' },
        { t: 'Payment consumer’ı bozuk', ok: false, fb: 'Payment bağlı, kuyruk boş ve consumer sayısı 1: tüketecek bir şey <b>gelmiyor</b>. Sorun yukarı yönde, yayın tarafında.' },
        { t: 'Mesajlar kayboldu, siparişleri elle yeniden oluşturmak gerek', ok: false, fb: 'Mesajlar kaybolmadı: 1.042 olay outbox tablosunda bekliyor.' },
        { t: 'Checkout kodunda hata var, HTTP isteği başarısız', ok: false, fb: 'İstek <b>başarılı</b> (201) ve sipariş yazıldı; hata, sonradan çalışan yayın adımında.' },
      ] } },
      { phase: 'aksiyon', title: 'Aksiyon', choice: { q: 'Order → RabbitMQ yolu kopuk. Neyi kontrol edersin?', opts: [
        { t: 'RABBITMQ_URL/kimlik bilgileri ve ağ erişimi, broker sağlığı (alarm, disk/bellek), Order logları (“publish” hataları)', ok: true, fb: 'Bağlantı kopukluğunun tipik nedenleri: değişen URL/şifre, ağ politikası, broker kaynak alarmı (publish bloklanır). Düzelince relay bekleyen olayları sırayla yayınlar.' },
        { t: 'Outbox tablosunu temizle', ok: false, fb: 'Bu olaylar ödeme isteği: silersen siparişler sonsuza dek ödeme bekler; veri kaybı. Outbox yalnızca <b>yayınlanmış</b> eski kayıtlarda (OUTBOX_RETENTION) temizlenir.' },
        { t: 'Saga kurtarma aralığını kısalt', ok: false, fb: 'Kurtarma işi aynı kırık yolla yeni olay üretir; kök neden broker bağlantısı.' },
        { t: 'Payment’ı çoğalt', ok: false, fb: 'Payment boş kuyrukta zaten bekliyor; kapasite sorunu yok.' },
      ] }, code: [{ title: 'Komutlar', lang: 'c-ok', body: 'docker compose logs --tail=100 order | grep -iE "publish|rabbit|amqp"\ndocker compose exec rabbitmq rabbitmq-diagnostics -q check_running\ndocker compose exec rabbitmq rabbitmq-diagnostics -q alarms\n# RABBITMQ_URL değerini compose/.env ile karşılaştır (çıktıyı paylaşma: şifre içerir)' }] },
      { phase: 'dogrulama', title: 'Doğrulama', view: () => dashE(true), body: '<p>Bağlantı düzeltildi. Aşağıda son 1 saat: birikimin eridiği anı ve sonrasındaki toplu tüketimi gör.</p>', choice: { q: 'İşlerin gerçekten tamamlandığını hangi kanıtlar birlikte gösterir?', opts: [
        { t: 'outbox_pending_events ve en eski yaş 0’a döndü; yayın ve tüketim ok patlaması yaptı; takılı siparişler confirmed oldu', ok: true, fb: 'Üç katman: <b>outbox</b> (yayın tarafı), <b>consumed/published</b> (taşıma), <b>orders_finished_total{status="confirmed"}</b> (iş sonucu).' },
        { t: 'HTTP latency ve 5xx yeşil', ok: false, fb: 'Zaten baştan beri yeşildi; iyileşmeyi göstermez.' },
        { t: 'RabbitMQ kuyruğu boş', ok: false, fb: 'Sorun sırasında da boştu. Boşluk hem “her şey işlendi” hem “hiçbir şey gelmiyor” olabilir.' },
      ] } },
    ],
    summary: '<h3>Hangi durumda hangi araç?</h3><table><tr><th>Belirti</th><th>İlk bakacağın</th><th>Neden</th></tr><tr><td>Yanıt yavaş</td><td>Grafana → Prometheus → <b>Jaeger</b></td><td>Süre nerede geçti: handler, DB, bağımlılık?</td></tr><tr><td>Hata oranı arttı</td><td>Grafana / <b>Prometheus</b> → Jaeger → log</td><td>Hangi servis/route, sonra neden</td></tr><tr><td>Kuyruk büyüyor</td><td><b>RabbitMQ UI</b> + Grafana queue depth</td><td>Consumer, hız ve Ready/Unacked</td></tr><tr><td>Mesaj işlenirken hata</td><td>Grafana (result) → <b>log/Jaeger</b> → RabbitMQ DLQ</td><td>Kaç → hangi/neden → mesajın kendisi</td></tr><tr><td>HTTP yeşil, iş bitmiyor</td><td><b>Prometheus/Grafana outbox</b> + Jaeger + RabbitMQ bağlantıları</td><td>Mesaj yayınlanıyor mu, ulaşıyor mu, tüketiliyor mu?</td></tr></table><p><b>Genel kural:</b> Prometheus <i>ne kadar / ne zaman</i>, Jaeger <i>tek akışta nerede</i>, RabbitMQ <i>mesaj şu an nerede ve kim tüketiyor</i> sorusunu cevaplar. HTTP başarısı asenkron işin başarısını kanıtlamaz.</p>',
  });
})();
