/* Architecture Lab: aynı iş (stok → ödeme → kesinleştir) iki yaklaşımla. Choreography Rovena'da YOK (varsayımsal). */
(function () {
  const { h, clear } = RL;
  const P = (from, to, kind, label) => ({ from, to, kind, label });

  const CHOR = {
    viewBox: '0 0 1000 400', chipY: 370,
    nodes: [
      { id: 'order', x: 40, y: 40, w: 250, h: 66, label: 'Order', lines: () => ['dinler: StockCommitted, PaymentFailed', 'yayınlar: OrderPlaced'] },
      { id: 'inventory', x: 375, y: 40, w: 250, h: 66, label: 'Inventory', lines: () => ['dinler: OrderPlaced, PaymentSucceeded/Failed', 'yayınlar: StockReserved/Committed/Released'] },
      { id: 'payment', x: 710, y: 40, w: 250, h: 66, label: 'Payment', lines: () => ['dinler: StockReserved', 'yayınlar: PaymentSucceeded/Failed'] },
      { id: 'bus', x: 40, y: 230, w: 920, h: 56, label: 'Olay veri yolu (RabbitMQ topic exchange)', cls: 'queue', lines: () => ['Merkez yok: her servis yalnızca tepki verdiği olayları bilir'] },
    ],
    edges: [['order', 'bus'], ['inventory', 'bus'], ['payment', 'bus']],
  };
  const ORCH = {
    viewBox: '0 0 1000 400', chipY: 370,
    nodes: [
      { id: 'orch', x: 360, y: 20, w: 280, h: 66, label: 'Order Saga (orchestrator)', cls: 'orch', lines: () => ['durumu tutar · sıradaki adımı seçer', 'telafileri başlatır'] },
      { id: 'bus', x: 330, y: 160, w: 340, h: 50, label: 'RabbitMQ (komut ↔ cevap)', cls: 'queue', lines: () => ['payment.requests / order.payment-results'] },
      { id: 'inventory', x: 40, y: 270, w: 260, h: 60, label: 'Inventory (katılımcı)', lines: () => ['yalnızca komutları uygular'] },
      { id: 'payment', x: 700, y: 270, w: 260, h: 60, label: 'Payment (katılımcı)', lines: () => ['komut alır, olayla cevap verir'] },
    ],
    edges: [['orch', 'inventory'], ['orch', 'bus'], ['bus', 'payment']],
  };

  const S = (actor, kind, state, title, what, packets) => ({ actor, kind, state, title, what, packets });
  function chorSteps(fail) {
    const a = [
      S('order', 'tx', 'running', 'Order siparişi yazar ve OrderPlaced olayını yayınlar', 'Order sıradaki adımı bilmez; sadece “sipariş verildi” der.', [P('order', 'bus', 'evt', 'OrderPlaced')]),
      S('inventory', 'tx', 'running', 'Inventory OrderPlaced’e tepki verir: stok rezerve eder', 'Inventory, OrderPlaced’i dinlediği için harekete geçer. Kimse ona “rezerve et” demedi.', [P('bus', 'inventory', 'evt', 'OrderPlaced')]),
      S('inventory', 'evt', 'running', 'Inventory StockReserved yayınlar', '', [P('inventory', 'bus', 'evt', 'StockReserved')]),
      S('payment', 'tx', 'running', 'Payment StockReserved’e tepki verir: ödeme alır', 'Payment ödeme için stoğun tutulmasını beklemesi gerektiğini “bilmek” zorunda: iş kuralı Payment’a sızdı.', [P('bus', 'payment', 'evt', 'StockReserved')]),
    ];
    if (!fail) return a.concat([
      S('payment', 'evt', 'running', 'Payment PaymentSucceeded yayınlar', '', [P('payment', 'bus', 'evt', 'PaymentSucceeded')]),
      S('inventory', 'tx', 'running', 'Inventory PaymentSucceeded’e tepki verir: stoğu kesinleştirir, StockCommitted yayınlar', '', [P('bus', 'inventory', 'evt', 'PaymentSucceeded'), P('inventory', 'bus', 'evt', 'StockCommitted')]),
      S('order', 'tx', 'completed', 'Order StockCommitted’e tepki verir: confirmed', 'Saga’nın bittiğini Order, zincirin son olayından çıkarır. “Tamam” diyen merkezi bir kayıt yok.', [P('bus', 'order', 'evt', 'StockCommitted')]),
    ]);
    return a.concat([
      S('payment', 'fail', 'running', 'Kart reddedildi: Payment PaymentFailed yayınlar', '', [P('payment', 'bus', 'err', 'PaymentFailed')]),
      S('inventory', 'comp', 'compensating', 'Inventory PaymentFailed’e tepki verir: stoğu bırakır (telafi), StockReleased yayınlar', 'Telafiyi Inventory kendi başına başlatır: PaymentFailed’in “benim stoğumu etkiler” anlamına geldiğini bilmek zorunda.', [P('bus', 'inventory', 'err', 'PaymentFailed'), P('inventory', 'bus', 'comp', 'StockReleased')]),
      S('order', 'tx', 'compensated', 'Order PaymentFailed (ve StockReleased) ile iptal eder', 'Bir Saga’nın telafisi tamamlandı mı? Bunu bilmek için birden fazla olayı takip etmek gerekir.', [P('bus', 'order', 'evt', 'PaymentFailed')]),
    ]);
  }
  function orchSteps(fail) {
    const a = [
      S('orch', 'tx', 'running', 'Orchestrator: sipariş pending; Inventory’ye “rezerve et” komutu', 'Rovena’da bu çağrı senkron REST’tir (Reserve).', [P('orch', 'inventory', 'cmd', 'Reserve')]),
      S('inventory', 'tx', 'running', 'Inventory rezerve eder ve cevap verir', '', [P('inventory', 'orch', 'evt', 'Reserved')]),
      S('orch', 'tx', 'running', 'Orchestrator: awaiting_payment; PaymentRequested komutu kuyruğa', 'Durum ve komut aynı transaction’da (outbox).', [P('orch', 'bus', 'cmd', 'PaymentRequested'), P('bus', 'payment', 'cmd', 'PaymentRequested')]),
    ];
    if (!fail) return a.concat([
      S('payment', 'tx', 'running', 'Payment ödemeyi alır, PaymentSucceeded ile cevap verir', 'Payment akışın geri kalanını bilmez; sadece komutu uygular ve sonucu söyler.', [P('payment', 'bus', 'evt', 'PaymentSucceeded'), P('bus', 'orch', 'evt', 'PaymentSucceeded')]),
      S('orch', 'tx', 'running', 'Orchestrator: paid; Inventory’ye “kesinleştir” komutu', 'Sıradaki adımı merkez seçer.', [P('orch', 'inventory', 'cmd', 'Commit')]),
      S('orch', 'tx', 'completed', 'Inventory commit eder; orchestrator: confirmed', 'Saga’nın durumu tek bir yerde (orders satırı) okunur.', [P('inventory', 'orch', 'evt', '200')]),
    ]);
    return a.concat([
      S('payment', 'fail', 'running', 'Kart reddedildi: PaymentFailed cevabı', '', [P('payment', 'bus', 'err', 'PaymentFailed'), P('bus', 'orch', 'err', 'PaymentFailed')]),
      S('orch', 'comp', 'compensating', 'Orchestrator: cancelled; Inventory’ye “bırak” komutu (telafi)', 'Telafiyi orchestrator yönetir; katılımcılar neden bırakıldığını bilmek zorunda değil.', [P('orch', 'inventory', 'comp', 'Release')]),
      S('orch', 'tx', 'compensated', 'Inventory bırakır; orchestrator: reservation_state = released', '', [P('inventory', 'orch', 'evt', '200')]),
    ]);
  }

  RL.initArch = function (root) {
    const mk = (title, tag, desc, diagram, gen) => {
      const host = h('div'), bar = h('div', { class: 'variants' }), info = h('div', { class: 'muted', style: 'font-size:12.5px;margin:4px 0' });
      const pl = { p: null };
      const run = (fail) => {
        [...bar.children].forEach((b, k) => b.classList.toggle('on', k === (fail ? 1 : 0)));
        const steps = gen(fail);
        if (!pl.p) pl.p = RL.sagaPlayer(host, { diagram, title, noCaption: true });
        pl.p.load(steps);
        const msgs = steps.reduce((n, s) => n + s.packets.filter((p) => p.from === 'bus' || p.to === 'bus' || ['cmd', 'evt', 'comp', 'err'].includes(p.kind)).length, 0);
        info.textContent = `${steps.length} adım · ${msgs} mesaj/çağrı`;
      };
      bar.append(h('button', { onclick: () => run(false) }, 'Başarılı akış'), h('button', { onclick: () => run(true) }, 'Ödeme reddi + telafi'));
      const card = h('div', { class: 'card' }, [h('div', null, tag), h('h2', { style: 'margin-top:6px' }, title), h('p', null, desc), bar, info, host]);
      setTimeout(() => run(false));
      return card;
    };
    const score = h('div');
    const qs = [
      ['Adım sayısı 4’ten fazla ya da büyüyecek', 2],
      ['Telafi kuralları karmaşık (sıra, koşullar, kısmi iade)', 2],
      ['Tek yerden “bu Saga nerede?” sorusu cevaplanmalı (destek, denetim)', 2],
      ['Timeout ve kurtarma mantığı gerekli', 1],
      ['Akış basit (2-3 adım) ve nadiren değişiyor', -2],
      ['Aynı olaya birbirinden habersiz yeni tüketiciler sık ekleniyor', -2],
      ['Takımlar servisleri bağımsız sahipleniyor, merkezi bir koda dokunmak pahalı', -1],
    ];
    const state = qs.map(() => false);
    function recalc() {
      const s = qs.reduce((n, q, i) => n + (state[i] ? q[1] : 0), 0);
      clear(score).append(h('div', { class: 'fb ' + (s > 0 ? 'info' : s < 0 ? 'good' : '') }, s > 1 ? `Eğilim: ORCHESTRATION (puan ${s}). Merkezi durum ve telafi yönetimi bakım maliyetini düşürür; orchestrator’ın tek hata/darboğaz noktası olmaması için onu kalıcı durum + idempotent adımlarla kur.` : s < -1 ? `Eğilim: CHOREOGRAPHY (puan ${s}). Basit, gevşek bağlı akışlarda merkez gereksiz yük olabilir; ama akışı görünür kılmak için correlation_id ve tracing şart.` : `Kararsız (puan ${s}). Hibrit düşün: çekirdek iş akışı orchestration, yan etkiler (bildirim, analitik) choreography ile olaylara abone.`));
    }
    recalc();
    root.append(
      h('div', { class: 'card' }, [h('h2', null, 'Aynı iş, iki yaklaşım'), h('p', null, 'İki animasyon da aynı checkout’u çalıştırır: stok → ödeme → kesinleştir. Fark, “sıradaki adımı kim biliyor?” sorusunun cevabı. Turuncu: komut, yeşil: olay, mor: telafi, kırmızı: hata.')]),
      mk('Choreography', h('span', { class: 'tag sim' }, 'VARSAYIMSAL · ROVENA’DA BÖYLE DEĞİL'), 'Servisler olaylara tepki verir, merkez yoktur. Olay adları (OrderPlaced, StockReserved…) Rovena’da yoktur; karşılaştırma için kurgulandı.', CHOR, chorSteps),
      mk('Orchestration', h('span', { class: 'tag real' }, 'ROVENA’NIN YAKLAŞIMI'), 'Order servisi Saga’nın durumunu tutar, komut gönderir (Reserve REST, PaymentRequested/RefundRequested mesaj), sonuçlara göre sıradaki adımı ya da telafiyi seçer. Payment ve Inventory akışı bilmez.', ORCH, orchSteps),
      h('div', { class: 'card' }, [h('h2', null, 'Karşılaştırma'), h('table', { class: 't' }, [
        h('tr', null, ['', 'Choreography', 'Orchestration'].map((x) => h('th', null, x))),
        [['Akış nerede yazılı?', 'Hiçbir yerde; olay zincirinin toplamı', 'Orchestrator kodunda (Rovena: saga.go, recovery.go)'],
          ['Saga durumu', 'Dağınık; her servis kendi parçasını bilir', 'Tek kayıt (orders satırı)'],
          ['Telafi', 'Her servis hangi olayın kendisini telafiye zorladığını bilmeli', 'Merkez karar verir, katılımcılara komut gönderir'],
          ['Timeout / kurtarma', 'Kim bekliyor? Belirsiz; her servis ayrı zamanlayıcı', 'Merkezde tek kurtarma işi (Rovena: RunRecovery)'],
          ['Bağımlılık', 'Gevşek; yeni tüketici eklemek kolay', 'Katılımcılar gevşek, merkez herkesi tanır'],
          ['Gözlemlenebilirlik', 'Trace + correlation_id olmadan takip zor', 'Durum sorgusu + trace yeterli'],
          ['Risk', 'Döngüsel olay zincirleri, “kim başlattı?” karmaşası', 'Orchestrator’ın şişmesi, tek nokta; kalıcı durumla hafifletilir']].map((r) => h('tr', null, r.map((c, i) => h(i ? 'td' : 'th', null, c)))),
      ])]),
      h('div', { class: 'card' }, [h('h2', null, 'Karar yardımcısı'), h('p', { class: 'muted' }, 'Sisteminize uyanları işaretle. Hiçbiri her zaman üstün değil; puan, karmaşıklık, hata yönetimi, gözlemlenebilirlik ve bakım maliyetini tartar.'),
        h('div', { class: 'checks' }, qs.map((q, i) => h('label', null, [h('input', { type: 'checkbox', onchange: (e) => { state[i] = e.target.checked; recalc(); } }), ' ', q[0]]))), score]),
      h('div', { class: 'card' }, [h('h2', null, 'Rovena için değerlendirme'), h('ul', null, [
        h('li', null, 'Checkout’un 4 adımı, sıralı bağımlılıkları (önce stok, sonra para, sonra kesinleştirme) ve 3 farklı telafisi var (sepet geri yükleme, release, refund). Zamana bağlı kararlar (rezervasyon TTL, ödeme timeout’u) merkezde: orchestration doğal seçim ve Rovena zaten öyle.'),
        h('li', null, 'Hibrit taraf: Inventory çağrıları senkron REST (cevap hemen lazım, müşteri bekliyor); Payment asenkron (dış sağlayıcı yavaş olabilir).'),
        h('li', null, 'OrderConfirmed / OrderCancelled olayları yayınlanıyor ama şu an dinleyen yok: bildirim, analitik gibi YAN işler ileride bu olaylara choreography ile abone olabilir. Çekirdek akışı değiştirmeden genişleme.'),
        h('li', null, 'Bedeli: Order servisi akışın tüm kurallarını taşır (saga.go + recovery.go). Bu kod büyüdükçe test kapsamı (service_test.go, e2e) kritik hale gelir.'),
      ])]),
    );
  };
})();
