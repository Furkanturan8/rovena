/*
 * Testing Lab: simülatör üzerinde 10 senaryo, beklenen ↔ gerçek sonuç ve invariant denetimi.
 * Her senaryo, aynı davranışı doğrulayan GERÇEK Rovena testlerine bağlanır (son çalıştırma sonuçlarıyla).
 */
(function () {
  const { h, clear } = RL;
  // Son çalıştırma: 2026-10-12, bu laboratuvar hazırlanırken (`make test` + izole entegrasyon testleri).
  const RUN = {
    date: '2026-10-12',
    unit: { ok: true, cmd: 'make test', note: 'Tüm modüller: ok (shared, identity, catalog, inventory, payment, order)' },
    integration: { ok: true, cmd: 'go test -tags integration (shared ./messaging ./outbox, payment ve order repository/postgres)', note: 'orders_test / payment_test veritabanları ve RabbitMQ “test” vhost’u (izole). Order: 26 test PASS.' },
    e2e: { ok: null, cmd: 'make e2e', note: 'ÇALIŞTIRILMADI: servisleri durdurup başlatır ve RabbitMQ kesintisi enjekte eder; çalışan geliştirme ortamını etkilememek için atlandı.' },
  };
  const T = (suite, name) => ({ suite, name });
  const CASES = [
    { id: 1, t: 'Her adım başarılı', opts: {}, exp: { state: 'completed', status: 'confirmed', charges: 1, refunds: 0, sold: 1, held: 0 },
      tests: [T('unit', 'TestCheckout_HappyPathStartsTheSaga'), T('unit', 'TestPaymentSucceeded_CommitsStockAndConfirms'), T('e2e', 'happy_path')] },
    { id: 2, t: 'Bir adım başarısız (kart reddi)', opts: { pay: 'declined' }, exp: { state: 'compensated', status: 'cancelled', charges: 0, refunds: 0, sold: 0, held: 0 },
      tests: [T('unit', 'TestPaymentFailed_CancelsAndReleasesStock'), T('unit', 'TestCheckout_OutOfStockCancelsTheOrderAndSendsNoPayment'), T('e2e', 'payment_declined'), T('e2e', 'insufficient_stock')] },
    { id: 3, t: 'Compensation başarılı (stok kaybı → iade)', opts: { commit: 'gone' }, exp: { state: 'compensated', status: 'cancelled', charges: 1, refunds: 1, sold: 0, held: 0 },
      tests: [T('unit', 'TestPaymentSucceeded_ReservationExpiredBeforePaymentCancelsAndRefunds'), T('unit', 'TestRefundGivesTheMoneyBackAndAnnouncesIt (payment)'), T('consumer', 'TestRefundedClosesTheRefundOfACancelledOrder')] },
    { id: 4, t: 'Compensation başarısız (iade yapılamıyor)', opts: { commit: 'gone', refundFails: true }, exp: { state: 'manual', status: 'cancelled', charges: 1, refunds: 0, sold: 0, held: 0 },
      tests: [T('unit', 'TestProviderOutageDuringRefundLeavesThePaymentSucceededForARetry (payment)'), T('integration', 'TestUnfinishedIncludesOutstandingRefundsUntilTheLimit'), T('unit', 'TestPaymentFailed_ReleaseOutageIsRetriedFromTheCancelledState'), T('unit', 'TestDuplicateChargeWhoseRefundFailsIsRetriedNotForgotten (payment)')] },
    { id: 5, t: 'Aynı mesaj tekrar teslim edildi', opts: { dupResult: true }, exp: { state: 'completed', status: 'confirmed', charges: 1, refunds: 0, sold: 1, held: 0 },
      tests: [T('unit', 'TestPaymentSucceeded_DeliveredTwiceChangesNothingTheSecondTime'), T('unit', 'TestPaymentFailed_DeliveredTwiceReleasesOnce'), T('integration', 'TestSaga_ConcurrentDuplicatePaymentResultsConfirmOnce'), T('e2e', 'duplicate_events_change_nothing')] },
    { id: 6, t: 'Consumer yeniden başlatıldı', opts: { paymentCrash: true }, exp: { state: 'completed', status: 'confirmed', charges: 1, refunds: 0, sold: 1, held: 0 },
      tests: [T('unit', 'TestPendingPaymentLeftByACrashIsContinued (payment)'), T('e2e', 'restart_order_mid_saga'), T('e2e', 'restart_payment_while_requests_wait')] },
    { id: 7, t: 'Mesaj gecikti (geç gelen başarı)', opts: { delayResult: true }, exp: { state: 'compensated', status: 'cancelled', charges: 1, refunds: 1, sold: 0, held: 0 },
      tests: [T('unit', 'TestPaymentSucceeded_ForACancelledOrderRefundsAndKeepsItCancelled'), T('e2e', 'silent_payment_cancels_and_late_success_is_refunded')] },
    { id: 8, t: 'Timeout gerçekleşti (istek kayboldu)', opts: { loseRequest: true }, exp: { state: 'completed', status: 'confirmed', charges: 1, refunds: 0, sold: 1, held: 0 },
      tests: [T('unit', 'TestRecovery_AsksForPaymentAgainThenGivesUp'), T('unit', 'TestRecovery_RepeatedPaymentRequestsReuseTheSameIdempotencyKey'), T('e2e', 'lost_payment_request_is_asked_again')] },
    { id: 9, t: 'Event iki kez yayımlandı', opts: { dupRequest: true }, exp: { state: 'completed', status: 'confirmed', charges: 1, refunds: 0, sold: 1, held: 0 },
      tests: [T('unit', 'TestRepeatedRequestChargesOnceAndSendsTheResultAgain (payment)'), T('unit', 'TestConcurrentRequestsForOneKeyChargeOnce (payment)'), T('integration', 'TestSaga_ConcurrentCheckoutRetriesWithOneKeyReserveOnce')] },
    { id: 10, t: 'DB işlemi ile yayın arasında hata', opts: { relayCrash: true }, naiveOpts: { naive: true }, exp: { state: 'completed', status: 'confirmed', charges: 1, refunds: 0, sold: 1, held: 0 },
      tests: [T('integration', 'TestTransition_StatusAndEventStandOrFallTogether'), T('integration', 'TestMarkReserved_RecordsReservationAndQueuesThePaymentRequestTogether'), T('integration', 'shared/outbox (relay) entegrasyon testleri'), T('e2e', 'rabbitmq_outage')] },
  ];
  const FIELDS = [['state', 'Saga durumu'], ['status', 'orders.status'], ['charges', 'Çekim sayısı'], ['refunds', 'İade sayısı'], ['sold', 'Satılan stok'], ['held', 'Tutulan stok']];
  const actualOf = (r) => ({ state: r.state, status: r.final.order.status, charges: r.final.pay.charges, refunds: r.final.pay.refunds, sold: r.final.inv.sold, held: r.final.inv.held });
  const suiteStatus = (s) => (s === 'e2e' ? RUN.e2e.ok : s === 'integration' ? RUN.integration.ok : RUN.unit.ok);

  RL.initTesting = function (root) {
    const results = {};
    const tbody = h('tbody'), detail = h('div'), playerHost = h('div');
    let player = null;
    root.append(
      h('div', { class: 'card' }, [h('h2', null, 'Rovena’nın gerçek test sonuçları'), h('p', { class: 'muted' }, `Son çalıştırma: ${RUN.date} (bu laboratuvar hazırlanırken).`),
        h('table', { class: 't' }, [h('tr', null, ['Paket', 'Komut', 'Sonuç', 'Not'].map((x) => h('th', null, x))),
          [['Birim', RUN.unit], ['Entegrasyon', RUN.integration], ['Uçtan uca', RUN.e2e]].map(([n, r]) => h('tr', null, [h('td', null, n), h('td', { class: 'mono' }, r.cmd), h('td', { class: r.ok ? 'ok-t' : r.ok === false ? 'bad-t' : 'warn-t' }, r.ok ? 'GEÇTİ' : r.ok === false ? 'KALDI' : 'ÇALIŞTIRILMADI'), h('td', null, r.note)]))]),
        RL.code('cd backend\nmake test               # birim\nmake test-integration    # yığın açıkken; DB/RabbitMQ gerekir\nmake e2e                 # yığın açıkken; servisleri durdurup başlatır', 'c-ok')]),
      h('div', { class: 'card' }, [h('h2', null, 'Simülatörde Saga testleri'), h('p', { class: 'muted' }, 'Her satır, Rovena modelini (SİMÜLE) bir hata enjeksiyonuyla çalıştırır ve son durumu beklentiyle ve beş invariant’la karşılaştırır. Beklentiler Rovena’nın gerçek testlerinin doğruladığı davranıştan alındı.'),
        h('div', { class: 'row-actions' }, [h('button', { class: 'btn', onclick: () => CASES.forEach((c) => run(c, false, true)) }, 'Tümünü çalıştır')]),
        h('table', { class: 't tests' }, [h('thead', null, h('tr', null, ['#', 'Senaryo', 'Beklenen Saga durumu', 'Gerçek', 'Invariant’lar', ''].map((x) => h('th', null, x)))), tbody])]),
      detail,
      h('div', { class: 'card' }, [h('h3', null, 'Adımları izle'), h('p', { class: 'muted' }, 'Bir testin “Adımları izle” düğmesine bas.'), playerHost]),
    );
    function row(c) {
      const r = results[c.id];
      const pass = r && FIELDS.every(([k]) => r.act[k] === c.exp[k]) && r.res.checks.every((x) => x.ok);
      return h('tr', null, [h('td', null, c.id), h('td', null, c.t), h('td', null, RL.stateChip(c.exp.state)), h('td', null, r ? RL.stateChip(r.act.state) : h('span', { class: 'muted' }, '—')),
        h('td', null, r ? h('span', { class: pass ? 'ok-t' : 'bad-t' }, `${r.res.checks.filter((x) => x.ok).length}/5 ${pass ? '✔ GEÇTİ' : '✖ KALDI'}`) : '—'),
        h('td', null, h('button', { class: 'btn small', onclick: () => run(c, false) }, 'Çalıştır'))]);
    }
    function drawRows() { clear(tbody); CASES.forEach((c) => tbody.append(row(c))); }
    function run(c, naive, quiet) {
      const res = RL.saga.simulate(naive ? c.naiveOpts : c.opts);
      results[c.id] = { res, act: actualOf(res), naive };
      drawRows();
      if (!quiet) show(c);
    }
    function show(c) {
      const r = results[c.id];
      clear(detail).append(h('div', { class: 'card' }, [
        h('div', { class: 'phase-tag' }, `Test ${c.id}${r.naive ? ' · VARSAYIMSAL: outbox yok' : ''}`), h('h2', null, c.t),
        h('table', { class: 't' }, [h('tr', null, ['Alan', 'Beklenen', 'Gerçek', ''].map((x) => h('th', null, x))),
          FIELDS.map(([k, l]) => { const ok = r.act[k] === c.exp[k]; return h('tr', null, [h('td', null, l), h('td', { class: 'mono' }, k === 'state' ? RL.stateChip(c.exp[k]) : String(c.exp[k])), h('td', { class: 'mono' }, k === 'state' ? RL.stateChip(r.act[k]) : String(r.act[k])), h('td', { class: ok ? 'ok-t' : 'bad-t' }, ok ? '✔' : '✖')]); })]),
        h('h4', { style: 'margin-top:10px' }, 'Invariant’lar'),
        h('table', { class: 't' }, r.res.checks.map((x) => h('tr', null, [h('td', { class: 'mono' }, x.id), h('td', null, x.t), h('td', { class: 'muted' }, x.why), h('td', { class: x.ok ? 'ok-t' : 'bad-t' }, x.ok ? '✔' : '✖ İHLAL')]))),
        r.naive ? h('div', { class: 'fb bad' }, 'Outbox olmadan: DB “ödeme istendi” diyor ama olay hiçbir yerde yok. Saga sessizce takıldı (I3 ihlali); beklenen “completed” sonuca ulaşılamadı. Rovena’da bu varyant mümkün değil çünkü olay durumla aynı transaction’da yazılır.') : null,
        h('h4', { style: 'margin-top:10px' }, 'Bu davranışı doğrulayan gerçek Rovena testleri'),
        h('table', { class: 't' }, c.tests.map((t) => { const s = suiteStatus(t.suite === 'consumer' ? 'unit' : t.suite); return h('tr', null, [h('td', null, h('span', { class: 'chip' }, t.suite)), h('td', { class: 'mono' }, t.name), h('td', { class: s ? 'ok-t' : s === false ? 'bad-t' : 'warn-t' }, s ? 'geçti' : s === false ? 'kaldı' : 'çalıştırılmadı')]); })),
        h('div', { class: 'row-actions' }, [
          h('button', { class: 'btn', onclick: () => { if (!player) player = RL.sagaPlayer(playerHost, { title: c.t }); player.load(r.res.steps); playerHost.scrollIntoView({ behavior: 'smooth' }); } }, 'Adımları izle'),
          c.naiveOpts ? h('button', { class: 'btn ghost', onclick: () => run(c, !r.naive) }, r.naive ? 'Rovena (outbox ile) çalıştır' : 'Outbox OLMADAN çalıştır (varsayımsal)') : null,
        ]),
      ]));
    }
    drawRows();
  };
})();
