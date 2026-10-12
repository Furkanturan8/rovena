/* Failure Lab: önce tahmin, sonra animasyon. Her senaryo Rovena modelinin (sim.js) bir hata enjeksiyonudur. */
(function () {
  const { h, clear } = RL;
  const SC = [
    {
      id: 'A', title: 'Ortadaki adım başarısız', blurb: 'Stok tutuldu, para çekildi, stok kesinleştirilemedi.',
      intro: '<p>Müşteri ödeme yaptı: <b>Adım 2 (rezervasyon)</b> ve <b>Adım 3 (ödeme)</b> başarılı. Ama ödeme o kadar geç geldi ki Inventory’deki 15 dakikalık rezervasyon süresi dolmuş: <b>Adım 4 (commit)</b> “rezervasyon yok” diyor.</p>',
      q: 'Rovena bu durumda ne yapar?',
      opts: [
        { t: 'Global rollback: üç veritabanı da işlem öncesine döner, para kendiliğinden geri gelir', ok: false, fb: 'Servisleri kapsayan bir transaction yok. Payment DB’deki “succeeded” kaydı ve sağlayıcıdaki çekim commit edildi; geri alınacak bir şey yok, ancak telafi edilebilirler.' },
        { t: 'Sipariş iptal edilir, stok kaydı kapatılır, para için RefundRequested komutu gönderilir', ok: true, fb: 'settle() ErrReservationGone görünce: cancel(reservation_lost) → MarkReleased → requestRefund. Her biri ayrı, yeni bir işlem.' },
        { t: 'Para çekildiği için sipariş confirmed sayılır', ok: false, fb: 'Stok başkasına satılmış olabilir; sipariş onaylanamaz. Saga başarılı sayılmaz.' },
        { t: 'Sipariş “paid” durumunda sonsuza kadar bekler', ok: false, fb: '“paid” bir ara durum; settle ve kurtarma işi ya commit ya telafi ile onu kapatır.' },
      ],
      variants: [
        { label: 'Adım 4 başarısız (rezervasyon süresi doldu)', opts: { commit: 'gone' } },
        { label: 'Adım 3 başarısız (kart reddedildi)', opts: { pay: 'declined' } },
        { label: 'Adım 2 başarısız (stok yok)', opts: { stock: 'short' } },
      ],
      lessons: '<ul><li><b>Neyi telafi edersin?</b> Yalnızca etkisi kalan adımları. Stok yoksa (Adım 2 başarısız) telafi edilecek bir şey yoktur → Saga <i>Failed</i>. Kart reddinde yalnızca stok bırakılır. Adım 4 başarısızsa hem stok hem para.</li><li><b>Ters sıra bir kural değil, varsayılandır.</b> Rovena önce siparişi iptal eder (kararı kalıcı yapar), sonra stoğu bırakır (yerel ve hızlı), en son iadeyi ister (dış sağlayıcı, yavaş, asenkron). Sıra iş bağımlılıklarına göre seçilir; birbirinden bağımsız telafiler paralel de çalışabilir.</li><li><b>Telafi başarısız olursa?</b> Release başarısızsa sipariş cancelled+held kalır, kurtarma işi tekrar dener. İade başarısızsa Senaryo D.</li><li><b>Telafi önceki durumu birebir geri getirmez:</b> sipariş “cancelled”dır (silinmez), Inventory hareket defterinde reserve ve release ayrı satırlardır, ekstrede çekim ve iade görünür.</li></ul>',
    },
    {
      id: 'B', title: 'Mesaj kayboldu ya da gecikti', blurb: 'Order cevap bekliyor, cevap gelmiyor.',
      intro: '<p>Order <code>PaymentRequested</code> yayınladı ve <code>awaiting_payment</code>’ta bekliyor. <b>2 dakikadır</b> (PaymentTimeout) cevap yok. Order, mesajın kaybolduğunu mu, Payment’ın yavaş olduğunu mu, ödemenin zaten yapıldığını mı bilemez.</p>',
      q: 'Order ne yapmalı?',
      opts: [
        { t: 'Ödemeyi başarısız say, siparişi hemen iptal et', ok: false, fb: 'Timeout “başarısız” değil “bilinmiyor” demektir. Ödeme yapılmış olabilir; iptal edip unutursak para müşteride değil bizde kalır.' },
        { t: 'Yeni bir idempotency key ile yeni bir ödeme iste', ok: false, fb: 'İlk istek de işlenirse iki çekim olur. Tekrar istek AYNI anahtarı taşımalı.' },
        { t: 'Aynı key ile tekrar sor (sınırlı sayıda); sınır dolunca iptal et ve geç gelen başarılı ödemeyi iade et', ok: true, fb: 'Rovena’nın kurtarma işi tam olarak bunu yapar: 3 istek (ORDER_MAX_PAYMENT_REQUESTS), sonra payment_timeout ile iptal. OnPaymentSucceeded iptal edilmiş sipariş için iade ister.' },
        { t: 'Cevap gelene kadar sonsuza dek bekle', ok: false, fb: 'Stok rezervasyonu ve müşteri bekler; sistem karar vermelidir. Rezervasyonun da süresi var.' },
      ],
      variants: [
        { label: 'Mesaj kayboldu (tekrar istek kurtarır)', opts: { loseRequest: true } },
        { label: 'Cevap gecikti (timeout → iptal → geç başarı → iade)', opts: { delayResult: true } },
      ],
      lessons: '<ul><li><b>Kayıp ve gecikme Order açısından ayırt edilemez.</b> İkisinde de doğru tepki aynı anahtarla yeniden sormaktır; Payment zaten işlemişse yalnızca sonucu tekrar gönderir.</li><li><b>Belirsiz sonuç</b> “karar + telafi edilebilirlik” ile ele alınır: Order sınırda iptal eder ama kararını geri dönüşlü tutar (geç gelen para iade edilir).</li><li><b>Duplicate:</b> Tekrar istekler kuyrukta birikir; her biri Payment’tan aynı sonucu tekrar yayınlatır. Order’ın ilk sonucu işlemesi yeter, diğerleri korumalı geçişte etkisizdir.</li><li>Metrik: <code>saga_recovery_actions_total{action="payment_requested_again"}</code>, <code>orders_finished_total{code="payment_timeout"}</code>.</li></ul>',
    },
    {
      id: 'C', title: 'Aynı mesaj iki kez işleniyor', blurb: 'RabbitMQ aynı sonucu tekrar teslim ediyor.',
      intro: '<p>Order <code>PaymentSucceeded</code>’i işledi, stoğu commit etti ve siparişi onayladı; ama ack’i göndermeden bağlantısı koptu. RabbitMQ aynı mesajı <b>tekrar</b> teslim ediyor.</p>',
      q: 'İkinci teslimde ne olur?',
      opts: [
        { t: 'Stok ikinci kez düşer, OrderConfirmed ikinci kez yayınlanır', ok: false, fb: 'Bu olsaydı handler idempotent olmazdı. Rovena’da geçişler “from” durumuna bağlı.' },
        { t: 'Hiçbir iş etkisi olmaz: sipariş zaten confirmed, korumalı geçiş değişiklik yapmaz, mesaj ack edilir', ok: true, fb: 'OnPaymentSucceeded: case StatusConfirmed → return nil. Eşzamanlı iki teslim bile FOR UPDATE ile sıraya girer (TestSaga_ConcurrentDuplicatePaymentResultsConfirmOnce).' },
        { t: 'RabbitMQ aynı mesajı iki kez teslim etmez, bu senaryo olmaz', ok: false, fb: 'RabbitMQ “en az bir kez” teslim eder: ack’ten önce bağlantı koparsa mesaj yeniden teslim edilir. Retry kuyruğu ve outbox relay de kopya üretebilir.' },
      ],
      variants: [
        { label: 'Sonuç olayı iki kez teslim edildi', opts: { dupResult: true } },
        { label: 'Ödeme isteği iki kez yayınlandı', opts: { dupRequest: true } },
        { label: 'Consumer işlerken çöktü ve yeniden başladı', opts: { paymentCrash: true } },
      ],
      lessons: '<ul><li><b>Neden şart?</b> “Tam bir kez teslim” yok; ama iş etkisinin <i>bir kez</i> olması gerekir (bir çekim, bir stok düşümü).</li><li><b>Rovena’daki yöntemler:</b> iş anahtarı (<code>order:&lt;id&gt;</code> → Payment tek çekim), sağlayıcıya aynı key, işleme kaydı (shared <code>Inbox</code>: processed_events), korumalı durum geçişleri (Order).</li><li>İdempotency etkiyi tek tutar, <b>cevabı</b> değil: Payment tekrar istekte sonucu yeniden yayınlar ki kaybolmuş bir cevap da telafi edilsin.</li></ul>',
    },
    {
      id: 'D', title: 'Compensation başarısız oluyor', blurb: 'İade gerekiyor, iade yapılamıyor.',
      intro: '<p>Senaryo A’daki gibi sipariş iptal edildi ve iade istendi. Ama ödeme sağlayıcısı <b>iade</b> çağrılarına uzun süredir cevap vermiyor.</p>',
      q: 'Saga ne durumda bitmeli?',
      opts: [
        { t: 'Compensated: iade istendi, iş bitti', ok: false, fb: 'İstemek yapmak değildir. Para müşteriye dönmediyse Saga başarılı/telafi edilmiş sayılamaz.' },
        { t: 'Sınırlı sayıda tekrar; sonra durum açıkça “müdahale gerekiyor” olarak görünür kalır (log, uyarı, DLQ, refund_state=requested)', ok: true, fb: 'Rovena: kurtarma işi iadeyi 5 kez ister, sonra ERROR “REFUND STUCK … a person must check” loglar; RefundNotAnswered uyarısı ve DLQ’daki mesajlar bunu görünür kılar.' },
        { t: 'Sonsuza kadar her 15 sn’de tekrar dene', ok: false, fb: 'Sınırsız deneme kalıcı sorunu gizler, yük üretir ve kimseyi uyarmaz.' },
        { t: 'Siparişi confirmed’a çevir, nasılsa para alındı', ok: false, fb: 'Stok yok; müşteriye teslim edilemeyecek bir siparişi onaylamak sorunu büyütür.' },
      ],
      variants: [{ label: 'İade sağlayıcısı kalıcı olarak cevapsız', opts: { commit: 'gone', refundFails: true } }],
      lessons: '<ul><li><b>Telafiler de başarısız olabilir.</b> Bu yüzden telafiler idempotent ve tekrar denenebilir tasarlanır.</li><li><b>Sessiz başarı yasak:</b> refund_state = requested kalır; durum hiçbir zaman otomatik “refunded” olmaz.</li><li><b>Görünürlük:</b> <code>saga_recovery_actions_total{action="refund_requested_again"}</code>, RefundNotAnswered uyarısı, DeadLetterQueueNotEmpty, ERROR log (order_id ile), admin uçlarında refund_state.</li><li><b>Rovena’da eksik:</b> ayrı bir “manual_intervention” durumu ve bildirim (Alertmanager) yok; operasyon ekibi uyarıyı Prometheus/Grafana’da görmelidir.</li></ul>',
    },
    {
      id: 'E', title: 'DB işlemi tamam, event yayımlanamadı', blurb: 'Dual-write problemi ve Transactional Outbox.',
      intro: '<p>Order, siparişi <code>awaiting_payment</code> yapan transaction’ı commit etti. Hemen ardından RabbitMQ’ya yayın sırasında süreç çöktü (ya da broker erişilemez oldu).</p>',
      q: 'Rovena’da ne olur?',
      opts: [
        { t: 'Olay kaybolur; sipariş sonsuza dek ödeme bekler', ok: false, fb: 'Bu, outbox OLMAYAN bir sistemde olurdu (aşağıda “varsayımsal” varyantı izle). Rovena’da olay DB’de satır olarak bekler.' },
        { t: 'Olay outbox tablosunda bekler; relay sonra (açılışta da) gönderir. Bir olay iki kez de gidebilir, tüketici idempotent', ok: true, fb: 'outbox.Add aynı transaction’da yazdı; relay published_at = NULL olanları yayınlar. Yayın ile işaretleme arasında çöküş kopya yaratabilir.' },
        { t: 'DB transaction’ı otomatik geri alınır', ok: false, fb: 'Commit olmuş bir transaction geri alınmaz; DB ile broker ayrı sistemlerdir.' },
      ],
      variants: [
        { label: 'Rovena: outbox + relay çöküşü', opts: { relayCrash: true } },
        { label: 'Rovena: aynı olay iki kez yayınlandı', opts: { dupRequest: true } },
        { label: 'VARSAYIMSAL: outbox yok (dual write)', opts: { naive: true }, hypo: true },
      ],
      lessons: '<ul><li><b>Dual write:</b> iki ayrı sisteme (DB ve broker) atomik yazamazsın; aradaki her çöküş ikisini tutarsız bırakır.</li><li><b>Outbox çözer:</b> “DB değişti ama olay yok” durumunu (olay da DB’de).</li><li><b>Outbox ÇÖZMEZ:</b> “en az bir kez” → tüketici idempotent olmalı; gecikme → relay geride kalabilir (outbox_oldest_pending_age_seconds); karşı servisin işi başarısız yapması → yine Saga telafileri; servisler arası tutarlılık → yine eventual.</li><li>Outbox’ın karşılığı tüketici tarafında <b>Inbox</b>’tır (shared/outbox/inbox.go).</li></ul>',
    },
  ];

  RL.initFailure = function (root) {
    const list = h('div', { class: 'sclist' }), main = h('div');
    root.append(h('div', { class: 'labgrid' }, [list, main]));
    const answered = {};
    let cur = SC[0];
    function drawList() {
      clear(list).append(h('h4', null, 'Hata senaryoları'));
      SC.forEach((sc) => list.append(h('button', { class: 'sc' + (sc === cur ? ' on' : ''), onclick: () => { cur = sc; drawList(); draw(); } }, [answered[sc.id] ? h('span', { class: 'done' }, '✔') : null, h('b', null, `${sc.id} · ${sc.title}`), h('small', null, sc.blurb)])));
    }
    function draw() {
      const sc = cur;
      const fb = h('div'), after = h('div', { hidden: !answered[sc.id] });
      const opts = h('div', { class: 'opts' });
      sc.opts.forEach((o) => opts.append(h('button', { class: 'opt', onclick: (e) => {
        e.currentTarget.classList.add(o.ok ? 'good' : 'bad');
        fb.append(h('div', { class: 'fb ' + (o.ok ? 'good' : 'bad') }, [h('b', null, o.ok ? 'Doğru. ' : 'Pek değil. '), h('span', { html: o.fb })]));
        if (o.ok && !answered[sc.id]) { answered[sc.id] = true; after.hidden = false; drawList(); run(sc.variants[0]); }
      } }, o.t)));
      const vbar = h('div', { class: 'variants' });
      const playerHost = h('div');
      const result = h('div');
      let player = null;
      function run(v) {
        [...vbar.children].forEach((b) => b.classList.toggle('on', b.textContent === v.label));
        const r = RL.saga.simulate(v.opts);
        if (!player) player = RL.sagaPlayer(playerHost, { title: sc.title });
        player.load(r.steps);
        clear(result).append(h('div', { class: 'fb info' }, [
          v.hypo ? h('div', null, [h('span', { class: 'tag sim' }, 'VARSAYIMSAL'), ' Bu akış Rovena’nın kodu değildir; outbox’ın neden gerekli olduğunu göstermek için kurgulandı.']) : null,
          h('b', null, 'Son durum: '), RL.stateChip(r.state), `  ·  orders.status=${r.final.order.status} · reservation=${r.final.order.res} · refund=${r.final.order.refund} · çekim=${r.final.pay.charges} · iade=${r.final.pay.refunds}`,
        ]));
      }
      sc.variants.forEach((v) => vbar.append(h('button', { onclick: () => run(v) }, v.label)));
      clear(main).append(h('div', { class: 'card' }, [
        h('div', { class: 'phase-tag' }, `Senaryo ${sc.id}`), h('h2', null, sc.title), h('div', { html: sc.intro }),
        h('h3', { style: 'margin-top:12px' }, 'Tahminin: ' + sc.q), opts, fb,
        answered[sc.id] ? null : h('p', { class: 'muted' }, 'Doğru cevabı bulunca animasyon açılır. Yanlış seçeneklerin açıklamalarını da oku.'),
      ]), after);
      after.append(h('div', { class: 'card' }, [h('h3', null, 'Ne oldu? Animasyonla izle'), h('p', { class: 'muted' }, 'Varyantlar aynı Rovena modelinde farklı hata enjeksiyonlarıdır. Her adıma tıklayıp veriyi, mesajı ve kodu görebilirsin.'), vbar, result, playerHost]),
        h('div', { class: 'card' }, [h('h3', null, 'Dersler ve hata politikası'), h('div', { html: sc.lessons })]));
      if (answered[sc.id]) run(sc.variants[0]);
    }
    drawList(); draw();
  };
})();
