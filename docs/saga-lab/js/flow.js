/* Saga Flow: kavramlar + Rovena'daki Saga'nın adımları + başarılı akış animasyonu. */
(function () {
  const { h } = RL;
  const TERMS = [
    ['Distributed transaction', 'Birden fazla bağımsız kaynağı (farklı servislerin veritabanları, broker) kapsayan iş. Rovena’da checkout: Order DB, Inventory DB, Payment DB ve RabbitMQ.'],
    ['Local transaction', 'TEK bir servisin kendi veritabanındaki ACID transaction’ı. Saga, bunların zinciridir. Her biri kendi başına commit olur.'],
    ['Saga', 'Bir dağıtık işi yerel transaction’lar dizisine bölen desen. Bir adım başarısız olursa, önceki adımların etkisi <b>telafi</b> işlemleriyle giderilir. Global kilit ve global rollback yoktur.'],
    ['Saga step', 'Bir servisin tek yerel transaction’ı + onu tetikleyen/takip eden mesaj. Rovena: rezervasyon, ödeme, stok commit.'],
    ['Forward action', 'Adımın asıl işi: Reserve, Charge, Commit.'],
    ['Compensation action', 'Tamamlanmış bir adımın iş etkisini gideren YENİ işlem: Release (stoğu bırak), Refund (parayı iade et). Geçmişi silmez; ekstrede çekim ve iade iki ayrı satırdır.'],
    ['Saga state', 'Saga’nın nerede olduğu. Rovena’da orders.status + reservation_state + refund_state, kalıcı olarak Order DB’de.'],
    ['Orchestrator', 'Saga’yı yöneten, sıradaki adımı seçen merkez. Rovena’da Order servisi: komut gönderir (PaymentRequested), cevaba göre karar verir.'],
    ['Choreography', 'Merkez yok; her servis olaylara tepki verip kendi olayını yayınlar. Akış, olay zincirinin toplamıdır.'],
    ['Retry ve timeout', 'Geçici hataları tekrar dene; cevabı gelmeyen adımı bir süre sonra fark et. Timeout “başarısız” demek değildir, “bilmiyorum” demektir.'],
    ['Idempotency', 'Aynı işlemi iki kez yapmak, bir kez yapmakla aynı etkiyi bırakır. Mesajlar “en az bir kez” teslim edildiği için şart.'],
    ['Eventual consistency', 'Servisler bir süre farklı şeyler bilebilir (Order “ödeme bekleniyor”, Payment “ödendi”), ama sonunda tutarlı duruma ulaşırlar.'],
  ];
  const STEPS = [
    ['1', 'Siparişi oluştur', 'Order', 'yerel', 'orders.status = pending', 'cancel (sepet müşteriye geri yüklenir)', 'telafi edilebilir'],
    ['2', 'Stok rezerve et', 'Inventory', 'REST (senkron)', 'Reserve → held', 'Release → released', 'telafi edilebilir'],
    ['3', 'Ödeme al', 'Payment', 'RabbitMQ: PaymentRequested → PaymentSucceeded/Failed', 'charge → succeeded', 'RefundRequested → refund', 'telafi edilebilir (iade)'],
    ['4', 'Stoğu kesinleştir, onayla', 'Inventory + Order', 'REST + yerel', 'Commit → confirmed', '— (son adım; başarısızsa 2 ve 3 telafi edilir)', 'son adım'],
  ];

  RL.initFlow = function (root) {
    const playerHost = h('div');
    root.append(
      h('div', { class: 'card' }, [
        h('h2', null, 'Saga hangi problemi çözer?'),
        h('div', { class: 'compare' }, [
          h('div', null, [h('b', null, 'Monolit + tek veritabanı'), h('p', null, 'BEGIN; stok düş; ödeme kaydı; sipariş onayla; COMMIT. Bir şey ters giderse ROLLBACK her şeyi geri alır.'), RL.code('BEGIN\n  UPDATE stock …\n  INSERT payment …\n  UPDATE orders …\nCOMMIT   -- ya hepsi ya hiçbiri', 'c-ok')]),
          h('div', null, [h('b', null, 'Rovena: servis başına veritabanı'), h('p', null, 'Order, Inventory, Payment ayrı PostgreSQL veritabanlarında; arada RabbitMQ ve dış ödeme sağlayıcısı var. Hepsini kapsayan bir transaction yok (2PC ne broker’da ne sağlayıcıda mümkün). Çekilen para ROLLBACK ile geri alınamaz.'), RL.code('Order DB    : COMMIT ✔\nInventory DB: COMMIT ✔\nSağlayıcı   : para çekildi ✔\nInventory   : commit ✖  →  ROLLBACK edilecek global tx YOK\n             →  telafi: iade + iptal', 'c-bad')]),
        ]),
        h('p', null, [h('b', null, 'Saga cevabı: '), 'işi yerel transaction’lara böl, her adımın sonucunu kalıcı kaydet, bir adım başarısız olursa önceki adımların etkisini ', h('b', null, 'telafi'), ' eden yeni işlemler çalıştır. Sonuç anında değil, ', h('i', null, 'sonunda'), ' tutarlıdır.']),
      ]),
      h('div', { class: 'card' }, [
        h('h2', null, 'Rovena’da Saga var mı?'),
        h('p', null, [h('span', { class: 'tag real' }, 'GERÇEK'), ' Evet. Checkout bir Saga’dır ve ', h('b', null, 'Order servisi orchestrator'), ' rolündedir (backend/services/order/internal/application/saga.go, recovery.go). Inventory ile senkron REST, Payment ile RabbitMQ üzerinden komut/olay konuşur; Transactional Outbox, idempotency anahtarları, retry/DLQ ve bir kurtarma işi (recovery) zaten var. Bu laboratuvar yeni bir Saga eklemez; var olanı öğretir.']),
        h('table', { class: 't' }, [
          h('tr', null, ['#', 'Adım', 'Servis', 'İletişim', 'Forward', 'Compensation', 'Tür'].map((x) => h('th', null, x))),
          STEPS.map((r) => h('tr', null, r.map((c) => h('td', null, c)))),
        ]),
        h('p', { class: 'muted', style: 'margin-top:8px' }, 'Durum modeli: Rovena’da ayrı bir saga tablosu yok. Laboratuvardaki Pending / Running / Completed / Failed / Compensating / Compensated / Requires manual intervention etiketleri orders.status + reservation_state + refund_state’ten TÜRETİLİR (Code Lab ▸ Saga durum modeli).'),
        h('div', { class: 'legend-states' }, ['pending', 'running', 'completed', 'failed', 'compensating', 'compensated', 'manual'].map((s) => RL.stateChip(s))),
      ]),
      h('div', { class: 'card' }, [h('h2', null, 'Kavramlar'), h('div', { class: 'gcols' }, TERMS.map(([t, d]) => h('div', { class: 'qcard' }, [h('b', null, t), h('div', { class: 'meta', html: d })])))]),
      h('div', { class: 'card' }, [h('h2', null, 'Başarılı Saga, adım adım'), h('p', { class: 'muted' }, 'Başlat’a bas ya da zaman çizelgesinden bir adıma tıkla. Çerçeveler servislerin transaction sınırlarıdır: bir adımın yerel transaction’ı çalışırken yalnızca o çerçeve yanar. Kutulardaki değerler o andaki veritabanı durumudur (SİMÜLE, Rovena’nın koduna göre modellenmiştir).'), playerHost]),
    );
    RL.sagaPlayer(playerHost, { steps: RL.saga.simulate({}).steps, title: 'Başarılı checkout Saga’sı' });
  };
})();
