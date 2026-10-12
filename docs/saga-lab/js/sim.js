/*
 * Rovena checkout Saga'sının deterministik SİMÜLATÖRÜ.
 *
 * Gerçek kod yolları (backend/services/order/internal/application/{order_service,saga,recovery}.go,
 * payment/internal/application/payment_service.go, shared/outbox, shared/messaging) adım adım modellenir.
 * Bir senaryo `opts` ile hata enjekte eder; çıktı, animasyonun oynattığı adımlar (her biri dünya durumunun
 * bir kopyasıyla) ve son durumdur. Gerçek bir broker'a ya da veritabanına hiçbir şey gönderilmez.
 */
(function () {
  const S = (RL.saga = {});
  const STOCK = 5, QTY = 1;
  // Rovena varsayılanları (docker-compose.yml + application.Config.withDefaults)
  S.CFG = { paymentTimeout: 120, stuckAfter: 30, maxPaymentRequests: 3, maxRefundRequests: 5, retryDelay: 5, paymentMaxRetries: 5, recoveryInterval: 15 };

  const clone = (o) => JSON.parse(JSON.stringify(o));

  /** Rovena'da tek bir saga_state kolonu yok: genel Saga durumu orders.status + reservation_state + refund_state'ten TÜRETİLİR. */
  S.sagaState = function (w) {
    const o = w.order;
    if (w.manual) return 'manual';
    if (!o.status) return 'pending';
    if (w.stuck) return 'stuck';
    if (['pending', 'awaiting_payment', 'paid'].includes(o.status)) return 'running';
    if (o.status === 'confirmed') return 'completed';
    // Laboratuvar dünyanın tamamını görür: para çekilmiş ama iade edilmemişse, Order bunu henüz bilmese de telafi sürüyor.
    if (o.res === 'held' || o.refund === 'requested' || w.pay.charges > w.pay.refunds) return 'compensating';
    if (o.res === 'released' || o.refund === 'refunded') return 'compensated';
    return 'failed';
  };
  S.STATE_LABEL = { pending: 'Pending', running: 'Running', completed: 'Completed', failed: 'Failed', compensating: 'Compensating', compensated: 'Compensated', manual: 'Requires manual intervention', stuck: 'Stuck (görünmez)' };

  S.simulate = function (opts) {
    opts = Object.assign({ stock: 'ok', pay: 'ok', commit: 'ok', loseRequest: false, delayResult: false, dupResult: false, dupRequest: false, refundFails: false, relayCrash: false, naive: false, paymentCrash: false }, opts || {});
    const C = S.CFG;
    const w = {
      t: 0,
      order: { status: '', res: 'none', refund: 'none', payReq: 0, refundReq: 0, code: '', outbox: [] },
      inv: { avail: STOCK, held: 0, sold: 0, resv: '—' },
      pay: { status: '—', charges: 0, refunds: 0, attempts: 0, outbox: [], providerCalls: 0 },
      q: { req: [], res: [], dlq: [] },
      history: [], manual: false, stuck: false, log: [],
    };
    const steps = [];
    const step = (s) => {
      if (s.dt) w.t += s.dt;
      const st = S.sagaState(w);
      steps.push(Object.assign({ t: w.t, state: st, snap: clone(w) }, s));
    };
    const hist = (from, to, reason) => w.history.push(`${from || '∅'} → ${to}  (${reason})`);
    const pkt = (from, to, kind, label, at, dur) => ({ from, to, kind, label, at, dur });

    /* ---------- yardımcı: outbox relay → exchange → kuyruk ---------- */
    function relay(svc, opt) {
      const box = svc === 'order' ? w.order.outbox : w.pay.outbox;
      if (!box.length) return;
      const evs = box.splice(0);
      evs.forEach((ev) => {
        const queue = ev === 'PaymentRequested' || ev === 'RefundRequested' ? 'req' : ev === 'OrderConfirmed' || ev === 'OrderCancelled' ? null : 'res';
        const qName = queue === 'req' ? 'q_req' : 'q_res';
        if (!queue) {
          step({ dt: 0.5, actor: svc, kind: 'publish', title: `${svc === 'order' ? 'Order' : 'Payment'} relay: ${ev} yayınlandı`, packets: [pkt(svc + 'db', 'ex', 'amqp', ev)],
            what: `${ev} bir bilgi olayıdır (fact); Rovena’da şu an bu olayı dinleyen bir kuyruk yok, broker onu bağlı kuyruk olmadığı için düşürür.`, ev: { type: ev }, code: 'relay', next: '—' });
          return;
        }
        w.q[queue].push(ev);
        step({ dt: 0.5, actor: svc, kind: 'publish', title: `Outbox relay: ${ev} → ${queue === 'req' ? 'payment.requests' : 'order.payment-results'}`, packets: [pkt(svc + 'db', 'ex', 'amqp', ev, 0, 0.45), pkt('ex', qName, 'amqp', ev, 0.5, 0.45)],
          what: `Relay döngüsü outbox_events’ten bekleyen satırı okur, broker’a yayınlar, publisher confirm gelince published_at’i doldurur. Mesaj kuyrukta “Ready” bekler.`,
          ev: envelope(ev), code: 'relay', next: queue === 'req' ? 'Payment consumer mesajı alacak.' : 'Order consumer sonucu alacak.' });
        if (opt && opt.twice && ev === 'PaymentRequested') {
          w.q.req.push(ev);
          step({ dt: 0.3, actor: svc, kind: 'warn', title: 'Relay published_at’i yazamadan çöktü → yeniden başlayınca AYNI olayı tekrar yayınladı', packets: [pkt(svc + 'db', 'ex', 'amqp', ev + ' (kopya)', 0, 0.45), pkt('ex', qName, 'amqp', ev + ' (kopya)', 0.5, 0.45)],
            what: 'Outbox “en az bir kez” yayın garanti eder, “tam bir kez” değil. Kuyrukta aynı event_id’ye sahip iki mesaj var: tüketici idempotent olmalı.', ev: envelope(ev, true), code: 'relay', next: 'Payment iki kopyayı da alacak.' });
        }
      });
    }
    function envelope(type, dup) {
      return { event_id: dup ? '0192ab…(aynı)' : '0192ab…', type, version: 1, correlation_id: 'order-id (siparişin kimliği)', traceparent: '00-<trace_id>-…-01', payload: type === 'PaymentRequested' ? { order_id: '…', idempotency_key: 'order:<order-id>', amount_cents: 3000, currency: 'TRY' } : { order_id: '…' } };
    }

    /* ---------- 1. adım: checkout + rezervasyon ---------- */
    step({ actor: 'client', kind: 'http', title: 'Müşteri checkout başlatır', packets: [pkt('client', 'order', 'http', 'POST /checkout')],
      what: 'Saga’yı başlatan istek. Idempotency-Key başlığı aynı isteğin tekrarını tek siparişe bağlar.', code: 'checkout', next: 'Order kendi veritabanında siparişi oluşturacak.' });
    w.order.status = 'pending'; hist(null, 'pending', 'checkout');
    step({ dt: 0.05, actor: 'order', kind: 'tx', tx: 'order', title: 'Yerel transaction #1 (Order DB): sipariş “pending” oluşturulur', packets: [pkt('order', 'orderdb', 'db', 'INSERT orders')],
      what: 'orders + order_items + status_history yazılır, sepet boşaltılır. Bu commit SADECE Order DB’yi kapsar; Inventory ve Payment bundan habersiz.', data: ['orders.status = pending', 'reservation_state = none'], code: 'checkout', next: 'Inventory’den stok rezervasyonu istenecek (senkron REST).' });

    step({ dt: 0.02, actor: 'order', kind: 'http', title: 'Adım 1 (forward): Inventory’de stok rezerve et', packets: [pkt('order', 'inventory', 'http', 'Reserve(order_id)')],
      what: 'Reserve sipariş başına idempotenttir: aynı order_id ile tekrar çağrılırsa aynı rezervasyonu döner (kurtarma işi bunu güvenle tekrarlayabilir).', code: 'advance', next: 'Inventory kendi transaction’ını yürütecek.' });
    if (opts.stock === 'short') {
      step({ dt: 0.02, actor: 'inventory', kind: 'fail', title: 'Inventory: yetersiz stok (409) — hiçbir şey değişmedi', packets: [pkt('inventory', 'order', 'err', '409 shortage')],
        what: 'İlk adım başarısız. Önceden tamamlanmış bir adım olmadığı için TELAFİ EDİLECEK BİR ŞEY YOK.', code: 'advance', next: 'Order siparişi iptal eder.' });
      w.order.status = 'cancelled'; w.order.code = 'out_of_stock'; w.order.outbox.push('OrderCancelled'); hist('pending', 'cancelled', 'out_of_stock');
      step({ dt: 0.02, actor: 'order', kind: 'tx', tx: 'order', title: 'Yerel transaction (Order DB): cancelled(out_of_stock) + OrderCancelled outbox’a', packets: [pkt('order', 'orderdb', 'db', 'UPDATE + outbox')],
        what: 'Durum değişikliği ve olay aynı transaction’da. Sepet müşteriye geri yüklenir.', data: ['status = cancelled', 'failure_code = out_of_stock'], code: 'transition', next: 'Saga bitti: Failed (telafi gerekmedi).' });
      relay('order');
      return finish();
    }
    w.inv.avail -= QTY; w.inv.held += QTY; w.inv.resv = 'held';
    step({ dt: 0.03, actor: 'inventory', kind: 'tx', tx: 'inventory', title: 'Yerel transaction #2 (Inventory DB): stok rezerve edildi', packets: [pkt('inventory', 'invdb', 'db', 'UPDATE stock / INSERT reservation')],
      what: 'Satırlar FOR UPDATE ile kilitlenir, kullanılabilir stok düşer, rezervasyon TTL ile (RESERVATION_TTL=15m) açılır. Bu, Order’ın transaction’ından TAMAMEN ayrı bir commit.', data: [`available ${STOCK} → ${w.inv.avail}`, 'reservation = held (15 dk TTL)'], code: 'advance', next: 'Order rezervasyonu kaydedip ödeme isteyecek.' });
    step({ dt: 0.01, actor: 'inventory', kind: 'http', title: 'Inventory cevap verdi: 201 reservation_id', packets: [pkt('inventory', 'order', 'http', '201 reservation')], what: 'Senkron adım tamamlandı.', code: 'advance', next: 'Order DB’ye yazılacak.' });

    w.order.status = 'awaiting_payment'; w.order.res = 'held'; w.order.payReq = 1; hist('pending', 'awaiting_payment', 'stock reserved, payment requested');
    if (opts.naive) {
      step({ dt: 0.02, actor: 'order', kind: 'tx', tx: 'order', title: '[VARSAYIMSAL] Yerel transaction: awaiting_payment (outbox YOK)', packets: [pkt('order', 'orderdb', 'db', 'UPDATE orders')],
        what: 'Bu varyant Rovena’nın kodu DEĞİL: olay outbox’a yazılmıyor, commit’ten sonra doğrudan broker’a yayınlanacak (dual write).', data: ['status = awaiting_payment'], code: 'outbox', next: 'Şimdi broker’a publish denenecek.' });
      step({ dt: 0.02, actor: 'order', kind: 'fail', title: '[VARSAYIMSAL] Publish başarısız: broker’a ulaşılamadı / süreç çöktü', packets: [pkt('order', 'ex', 'err', 'PaymentRequested ✖')],
        what: 'DB commit oldu, mesaj gitmedi. Olay HİÇBİR YERDE kayıtlı değil: Payment asla haberdar olmayacak, stok rezervasyonda kalacak.', code: 'outbox', next: 'Kimse bu sipariş için bir şey yapmayacak.' });
      w.stuck = true;
      step({ dt: 600, actor: 'order', kind: 'warn', title: '[VARSAYIMSAL] 10 dk sonra: sipariş hâlâ awaiting_payment, hiçbir metrik/uyarı yok', what: 'Sessiz tutarsızlık: DB “ödeme istendi” diyor, broker’da mesaj yok. Bu, dual-write probleminin ta kendisi.', code: 'outbox', next: '—' });
      return finish();
    }
    w.order.outbox.push('PaymentRequested');
    step({ dt: 0.02, actor: 'order', kind: 'tx', tx: 'order', title: 'Yerel transaction #3 (Order DB): awaiting_payment + PaymentRequested OUTBOX’A', packets: [pkt('order', 'orderdb', 'db', 'UPDATE + INSERT outbox')],
      what: 'MarkReserved: durum geçişi (FOR UPDATE ile korunur), history satırı ve PaymentRequested olayı AYNI transaction’da yazılır. Ya hepsi ya hiçbiri: dual-write problemi yok.', data: ['status: pending → awaiting_payment', 'reservation_state = held', 'payment_requests = 1', 'outbox_events += PaymentRequested'], ev: envelope('PaymentRequested'), code: 'transition', next: 'HTTP cevabı döner; olay arka planda relay ile yayınlanır.' });
    step({ dt: 0.01, actor: 'order', kind: 'http', title: 'Müşteriye 201: sipariş awaiting_payment', packets: [pkt('order', 'client', 'http', '201')],
      what: 'HTTP isteği burada BİTER. Saga bitmedi: ödeme ve stok onayı asenkron devam eder (eventual consistency).', code: 'checkout', next: 'Outbox relay PaymentRequested’i yayınlayacak.' });

    if (opts.relayCrash) {
      step({ dt: 0.3, actor: 'order', kind: 'fail', title: 'Relay yayınlayamadı (broker kesintisi / süreç çöktü)', packets: [pkt('orderdb', 'ex', 'err', 'PaymentRequested ✖')],
        what: 'DB commit’i başarılıydı; olay outbox_events tablosunda published_at = NULL olarak duruyor. KAYBOLMADI. outbox_pending_events = 1 olur.', data: ['outbox_events: 1 bekleyen'], code: 'outbox', next: 'Relay her turda tekrar dener; süreç yeniden başlarsa ilk iş “sweep” yapar.' });
      step({ dt: 20, actor: 'order', kind: 'timer', title: '20 sn sonra: Order yeniden başladı, relay açılışta sweep yaptı', what: 'Relay.Run önce sweep çağırır: çöküşten kalan olaylar açılışta gönderilir.', code: 'relay', next: 'Olay yayınlanacak.' });
    }
    relay('order', { twice: opts.dupRequest });

    /* ---------- 2. adım: ödeme (asenkron, RabbitMQ) ---------- */
    let paymentDone = false;
    function paymentConsume(label) {
      if (!w.q.req.length) return;
      const ev = w.q.req.shift();
      if (ev === 'RefundRequested') return refundConsume();
      step({ dt: 0.2, actor: 'payment', kind: 'deliver', title: `Payment consumer ${label || 'PaymentRequested’i aldı'} (Unacked=1)`, packets: [pkt('q_req', 'payment', 'amqp', 'deliver')],
        what: 'prefetch=1: consumer aynı anda bir mesaj tutar. İş bitip ack edilene kadar mesaj kuyruktan silinmez.', code: 'payment', next: 'Payment, idempotency key ile önceki kaydı arar.' });
      if (paymentDone) {
        w.pay.outbox.push(w.pay.status === 'failed' ? 'PaymentFailed' : 'PaymentSucceeded');
        step({ dt: 0.05, actor: 'payment', kind: 'tx', tx: 'payment', title: 'Aynı idempotency key bulundu: YENİ ÇEKİM YOK, saklı sonuç tekrar yayınlanır', packets: [pkt('payment', 'paydb', 'db', 'SELECT by key')],
          what: 'Payment.Process → Begin(key): kayıt zaten tamamlanmış → republish. Para ikinci kez çekilmez; Order cevabı yeniden alır (güvenli).', data: [`charges = ${w.pay.charges} (değişmedi)`], code: 'idempotency', next: 'Mesaj ack edilir, sonuç tekrar yayınlanır.' });
        step({ dt: 0.02, actor: 'payment', kind: 'ack', title: 'ack', packets: [pkt('payment', 'q_req', 'amqp', 'ack')], what: 'Mesaj kuyruktan silindi.', code: 'payment', next: '' });
        relay('payment');
        return;
      }
      w.pay.status = 'pending'; w.pay.attempts++;
      step({ dt: 0.02, actor: 'payment', kind: 'tx', tx: 'payment', title: 'Yerel transaction #4 (Payment DB): ödeme kaydı “pending” açıldı', packets: [pkt('payment', 'paydb', 'db', 'INSERT payment')],
        what: 'Begin: (idempotency_key) benzersiz. Sağlayıcı çağrılmadan önce kayıt açılır ki çöküşte “yarım kalmış ödeme” bulunabilsin.', data: ['payments.status = pending', 'idempotency_key = order:<id>'], code: 'payment', next: 'Sağlayıcı çağrılacak.' });
      if (opts.delayResult) {
        step({ dt: 1, actor: 'payment', kind: 'http', title: 'Sağlayıcı çağrıldı ve CEVAP GECİKİYOR', packets: [pkt('payment', 'provider', 'http', 'Charge(key) …')],
          what: 'Sağlayıcı yavaş. Order bekliyor ama sonucu bilmiyor: ödeme başarısız mı, başarılı mı, hâlâ sürüyor mu? Sonuç BELİRSİZ.', code: 'payment', next: 'Order’ın kurtarma işi PaymentTimeout (2 dk) sonra devreye girecek.' });
        return 'delayed';
      }
      w.pay.providerCalls++;
      const declined = opts.pay === 'declined';
      if (!declined) w.pay.charges = 1;
      step({ dt: 0.3, actor: 'payment', kind: 'http', title: `Sağlayıcı: ${declined ? 'kart reddedildi' : 'çekim onaylandı'}`, packets: [pkt('payment', 'provider', 'http', 'Charge(key)', 0, 0.45), pkt('provider', 'payment', declined ? 'err' : 'http', declined ? 'declined' : 'approved', 0.5, 0.45)],
        what: 'Sağlayıcıya da AYNI idempotency key gider: aynı key ile tekrar çağrı aynı sonucu döner, ikinci çekim olmaz.', code: 'payment', next: 'Sonuç Payment DB’ye ve outbox’a yazılacak.' });
      if (opts.paymentCrash && !w._crashed) {
        w._crashed = true; w.q.req.unshift('PaymentRequested');
        step({ dt: 0.05, actor: 'payment', kind: 'fail', title: 'Payment süreci sonucu kaydedemeden ÇÖKTÜ (ack yok)', packets: [pkt('payment', 'q_req', 'err', 'bağlantı koptu')],
          what: 'Ack gönderilmediği için RabbitMQ mesajı tekrar kuyruğa koyar (redelivery). Payment DB’de ödeme “pending” kaldı.', data: ['payments.status = pending'], code: 'payment', next: 'Consumer yeniden başlayınca aynı mesajı tekrar alacak.' });
        step({ dt: 5, actor: 'payment', kind: 'timer', title: 'Payment yeniden başladı', what: 'Restart: kaldığı yerden devam etmesi için ihtiyacı olan her şey DB’de ve kuyrukta.', code: 'payment', next: '' });
        step({ dt: 0.2, actor: 'payment', kind: 'deliver', title: 'Aynı PaymentRequested yeniden teslim edildi (redelivered)', packets: [pkt('q_req', 'payment', 'amqp', 'redeliver')], what: 'Begin(key) “pending” kaydı bulur → charge’ı aynı key ile tekrar dener.', code: 'idempotency', next: '' });
        w.q.req.shift(); w.pay.providerCalls++;
        step({ dt: 0.3, actor: 'payment', kind: 'http', title: 'Sağlayıcı aynı key’i tanıdı: ÖNCEKİ sonucu döndü (ikinci çekim yok)', packets: [pkt('payment', 'provider', 'http', 'Charge(key)', 0, 0.45), pkt('provider', 'payment', 'http', 'aynı sonuç', 0.5, 0.45)],
          what: 'Sağlayıcı seviyesinde idempotency: sağlayıcı çağrısı tekrarlanabilir. charges hâlâ 1.', data: ['charges = 1'], code: 'idempotency', next: '' });
      }
      w.pay.status = declined ? 'failed' : 'succeeded'; paymentDone = true;
      w.pay.outbox.push(declined ? 'PaymentFailed' : 'PaymentSucceeded');
      step({ dt: 0.02, actor: 'payment', kind: 'tx', tx: 'payment', title: `Yerel transaction #5 (Payment DB): ${declined ? 'failed' : 'succeeded'} + ${declined ? 'PaymentFailed' : 'PaymentSucceeded'} outbox’a`, packets: [pkt('payment', 'paydb', 'db', 'UPDATE + outbox')],
        what: 'complete(): sonuç ve olay aynı transaction’da. Payment’ın bu commit’i Order’ın verisine dokunmaz.', data: [`payments.status = ${w.pay.status}`, `charges = ${w.pay.charges}`], ev: envelope(declined ? 'PaymentFailed' : 'PaymentSucceeded'), code: 'payment', next: 'Mesaj ack edilir; sonuç olayı relay ile yayınlanır.' });
      step({ dt: 0.02, actor: 'payment', kind: 'ack', title: 'Payment mesajı ack etti', packets: [pkt('payment', 'q_req', 'amqp', 'ack')], what: 'payment.requests kuyruğundan silindi (Unacked 1 → 0).', code: 'payment', next: '' });
      relay('payment');
      return 'done';
    }

    function refundConsume() {
      step({ dt: 0.2, actor: 'payment', kind: 'deliver', title: 'Payment RefundRequested’i aldı', packets: [pkt('q_req', 'payment', 'amqp', 'deliver')], what: 'RefundRequested bir KOMUTTUR (Order → Payment: “bu siparişin parasını geri ver”).', code: 'compensation', next: '' });
      if (opts.refundFails) {
        for (let k = 1; k <= C.paymentMaxRetries; k++) {
          step({ dt: k === 1 ? 0.3 : C.retryDelay, actor: 'payment', kind: 'fail', title: `İade denemesi ${k}: sağlayıcı cevap vermiyor → retry kuyruğu (x-retry-count=${k})`, packets: [pkt('payment', 'provider', 'err', 'Refund ✖', 0, 0.4), pkt('payment', 'q_req', 'amqp', 'retry ' + k, 0.5, 0.45)],
            what: 'Ödeme “succeeded” kalır, mesaj RetryDelay (5 sn) sonra tekrar denenir. Para hâlâ müşteride DEĞİL.', code: 'retry', next: '' });
        }
        w.q.dlq.push('RefundRequested');
        step({ dt: 0.3, actor: 'payment', kind: 'fail', title: 'Deneme hakkı bitti: RefundRequested → payment.requests.dlq', packets: [pkt('payment', 'q_dlq', 'err', 'x-error')],
          what: 'DLQ’da insan bekleyen mesaj. DeadLetterQueueNotEmpty uyarısı tetiklenir. Order tarafında refund_state hâlâ “requested”.', data: ['payment.requests.dlq: 1'], code: 'retry', next: 'Order’ın kurtarma işi cevap gelmediği için iadeyi tekrar isteyecek.' });
        return;
      }
      w.pay.status = 'refunded'; w.pay.refunds = 1; w.pay.outbox.push('PaymentRefunded');
      step({ dt: 0.4, actor: 'payment', kind: 'tx', tx: 'payment', title: 'Compensation (Payment DB): sağlayıcıdan iade + refunded + PaymentRefunded outbox’a', packets: [pkt('payment', 'provider', 'http', 'Refund', 0, 0.4), pkt('payment', 'paydb', 'db', 'UPDATE + outbox', 0.5, 0.4)],
        what: 'Telafi, eski çekimi “geri almaz”: YENİ bir para hareketi (iade) yaratır. Ekstrede iki satır görünür: çekim ve iade.', data: ['payments.status = refunded', 'refunds = 1'], code: 'compensation', next: 'Order iadeyi kaydedecek.' });
      relay('payment');
    }

    function orderConsume() {
      while (w.q.res.length) {
        const ev = w.q.res.shift();
        const dup = steps.some((s) => s.actor === 'order' && s.kind === 'deliver' && s.ev && s.ev.type === ev);
        step({ dt: 0.2, actor: 'order', kind: 'deliver', title: `Order consumer ${ev}${dup ? ' (TEKRAR teslim)' : ''} aldı`, packets: [pkt('q_res', 'order', 'amqp', ev)], ev: envelope(ev, dup),
          what: 'order.payment-results kuyruğu. Handler korumalı geçişler kullanır: aynı olay iki kez gelse bile durum yalnızca bir kez değişir.', code: 'onresult', next: '' });
        if (ev === 'PaymentRefunded') {
          w.order.refund = 'refunded';
          step({ dt: 0.02, actor: 'order', kind: 'tx', tx: 'order', title: 'Yerel transaction (Order DB): refund_state = refunded', packets: [pkt('order', 'orderdb', 'db', 'UPDATE')], what: 'Telafi tamamlandı: müşteri parasını geri aldı.', data: ['refund_state: requested → refunded'], code: 'compensation', next: 'Saga: Compensated.' });
          continue;
        }
        if (ev === 'PaymentFailed') {
          if (w.order.status !== 'awaiting_payment') { step({ dt: 0.01, actor: 'order', kind: 'note', title: 'Korumalı geçiş: değişiklik yok', what: 'Sipariş zaten iptal edilmiş; tekrar teslim zararsız.', code: 'onresult' }); continue; }
          w.order.status = 'cancelled'; w.order.code = 'card_declined'; w.order.outbox.push('OrderCancelled'); hist('awaiting_payment', 'cancelled', 'card_declined');
          step({ dt: 0.02, actor: 'order', kind: 'tx', tx: 'order', title: 'Yerel transaction (Order DB): cancelled(card_declined) + OrderCancelled', packets: [pkt('order', 'orderdb', 'db', 'UPDATE + outbox')], what: 'Ödeme adımı başarısız → sipariş iptal. Önceki adımın (stok) etkisi hâlâ duruyor: telafi edilmeli.', data: ['status → cancelled', 'reservation_state = held (henüz)'], code: 'onresult', next: 'Compensation: stok bırakılacak.' });
          compensateRelease();
          continue;
        }
        // PaymentSucceeded
        if (w.order.status === 'confirmed' || (w.order.status === 'paid' && dup)) {
          step({ dt: 0.01, actor: 'order', kind: 'note', title: 'Korumalı geçiş: sipariş zaten confirmed → hiçbir şey yapılmaz', packets: [pkt('order', 'orderdb', 'db', 'SELECT … FOR UPDATE')],
            what: 'repo.apply: from = [awaiting_payment] tutmadığı için UPDATE yapılmaz, outbox’a olay eklenmez. İş etkisi ikinci kez oluşmaz.', code: 'transition', next: 'Mesaj ack edilir.' });
          continue;
        }
        if (w.order.status === 'cancelled') {
          if (w.order.refund !== 'none') { step({ dt: 0.01, actor: 'order', kind: 'note', title: 'İade zaten istenmiş: yeni RefundRequested kuyruğa EKLENMEZ', what: 'RequestRefund yalnızca ilk çağrıda olay yazar (TestConcurrentRefundRequestsQueueOneEvent).', code: 'compensation' }); continue; }
          w.order.refund = 'requested'; w.order.refundReq = 1; w.order.outbox.push('RefundRequested');
          step({ dt: 0.02, actor: 'order', kind: 'tx', tx: 'order', title: 'GEÇ GELEN BAŞARI: sipariş iptal kalır, iade istenir (refund_state = requested)', packets: [pkt('order', 'orderdb', 'db', 'UPDATE + outbox')],
            what: 'Timeout, ödemenin başarısız olduğu anlamına gelmiyordu. Sipariş geri “canlandırılmaz” (stok çoktan bırakıldı); para telafi olarak iade edilir.', data: ['status = cancelled (değişmez)', 'refund_state = requested', 'outbox += RefundRequested'], code: 'compensation', next: 'Payment iadeyi yapacak.' });
          continue;
        }
        w.order.status = 'paid'; hist('awaiting_payment', 'paid', 'payment succeeded');
        step({ dt: 0.02, actor: 'order', kind: 'tx', tx: 'order', title: 'Yerel transaction (Order DB): paid', packets: [pkt('order', 'orderdb', 'db', 'UPDATE')], what: 'MarkPaid. Stok onayı henüz yapılmadı: “paid” bir ara durumdur, kurtarma işi bunu bitirmeyi bilir.', data: ['status: awaiting_payment → paid'], code: 'onresult', next: 'Adım 3: Inventory’de rezervasyon commit edilecek.' });
        step({ dt: 0.02, actor: 'order', kind: 'http', title: 'Adım 3 (forward): Inventory’de rezervasyonu commit et', packets: [pkt('order', 'inventory', 'http', 'Commit(reservation)')], what: 'settle(): paid siparişin stoğu kalıcı olarak düşülür.', code: 'settle', next: '' });
        if (opts.commit === 'gone') {
          w.inv.avail += QTY; w.inv.held -= QTY; w.inv.resv = 'expired';
          step({ dt: 0.02, actor: 'inventory', kind: 'fail', title: 'Inventory: rezervasyon YOK (TTL doldu, stok serbest kaldı) → ErrReservationGone', packets: [pkt('inventory', 'order', 'err', '410 gone')],
            what: 'Ödeme geç geldi: 15 dk’lık rezervasyon süresi dolmuş, stok başkasına satılabilir hale gelmiş. Üçüncü adım başarısız; ilk iki adımın etkisi var (para çekildi).', data: ['reservation = expired', `available = ${w.inv.avail}`], code: 'settle', next: 'Order iptal edip telafiyi başlatacak.' });
          w.order.status = 'cancelled'; w.order.code = 'reservation_lost'; w.order.outbox.push('OrderCancelled'); hist('paid', 'cancelled', 'reservation_lost');
          step({ dt: 0.02, actor: 'order', kind: 'tx', tx: 'order', title: 'Yerel transaction (Order DB): cancelled(reservation_lost)', packets: [pkt('order', 'orderdb', 'db', 'UPDATE + outbox')], what: 'Karar kalıcı hale geldi. Bundan sonra yapılacaklar telafi.', data: ['status: paid → cancelled'], code: 'settle', next: 'Compensation 1: stok tarafı kapatılır.' });
          w.order.res = 'released';
          step({ dt: 0.02, actor: 'order', kind: 'comp', tx: 'order', title: 'Compensation (stok): reservation_state = released', packets: [pkt('order', 'orderdb', 'db', 'MarkReleased')], what: 'Rezervasyon zaten süresi dolarak gitmiş; Release “zaten yoksa da başarılı” sayılır (idempotent). Yalnızca kayıt güncellenir.', data: ['reservation_state = released'], code: 'compensation', next: 'Compensation 2: para iadesi istenecek.' });
          w.order.refund = 'requested'; w.order.refundReq = 1; w.order.outbox.push('RefundRequested');
          step({ dt: 0.02, actor: 'order', kind: 'comp', tx: 'order', title: 'Compensation (ödeme): RefundRequested outbox’a, refund_state = requested', packets: [pkt('order', 'orderdb', 'db', 'UPDATE + outbox')], what: 'requestRefund(): iade isteği komutu, durumla aynı transaction’da yazılır.', data: ['refund_state = requested', 'refund_requests = 1'], code: 'compensation', next: 'Relay RefundRequested’i Payment’a taşıyacak.' });
          return 'refund';
        }
        w.inv.held -= QTY; w.inv.sold += QTY; w.inv.resv = 'committed';
        step({ dt: 0.03, actor: 'inventory', kind: 'tx', tx: 'inventory', title: 'Yerel transaction (Inventory DB): rezervasyon committed, stok satıldı', packets: [pkt('inventory', 'invdb', 'db', 'UPDATE')], what: 'Stok kalıcı olarak düştü.', data: ['reservation = committed', `sold = ${w.inv.sold}`], code: 'settle', next: '' });
        step({ dt: 0.01, actor: 'inventory', kind: 'http', title: 'Inventory: 200', packets: [pkt('inventory', 'order', 'http', '200')], what: '', code: 'settle', next: '' });
        w.order.status = 'confirmed'; w.order.res = 'committed'; w.order.outbox.push('OrderConfirmed'); hist('paid', 'confirmed', 'stock committed');
        step({ dt: 0.02, actor: 'order', kind: 'tx', tx: 'order', title: 'Yerel transaction (Order DB): confirmed + OrderConfirmed outbox’a', packets: [pkt('order', 'orderdb', 'db', 'UPDATE + outbox')],
          what: 'Confirm: from=[paid] ve reservation=held koşulu ile. Saga TAMAMLANDI (son durum).', data: ['status: paid → confirmed', 'reservation_state = committed', 'orders_finished_total{status="confirmed"} +1'], code: 'transition', next: '—' });
      }
    }

    function compensateRelease() {
      step({ dt: 0.02, actor: 'order', kind: 'comp', title: 'Compensation: Inventory’den rezervasyonu bırak', packets: [pkt('order', 'inventory', 'comp', 'Release(reservation)')], what: 'settle() iptal edilmiş siparişin stoğunu bırakır. Inventory’ye ulaşılamazsa sipariş “cancelled + held” kalır ve kurtarma işi tekrar dener.', code: 'compensation', next: '' });
      w.inv.avail += QTY; w.inv.held -= QTY; w.inv.resv = 'released';
      step({ dt: 0.03, actor: 'inventory', kind: 'tx', tx: 'inventory', title: 'Yerel transaction (Inventory DB): rezervasyon released, stok geri döndü', packets: [pkt('inventory', 'invdb', 'db', 'UPDATE')], what: 'Telafi yeni bir işlemdir: stok sayısı geri gelir ama hareket defterinde “reserve” ve “release” iki ayrı kayıt olarak kalır.', data: ['reservation = released', `available = ${w.inv.avail}`], code: 'compensation', next: '' });
      w.order.res = 'released';
      step({ dt: 0.02, actor: 'order', kind: 'tx', tx: 'order', title: 'Yerel transaction (Order DB): reservation_state = released', packets: [pkt('order', 'orderdb', 'db', 'MarkReleased')], what: 'Telafi tamamlandı.', data: ['reservation_state: held → released'], code: 'compensation', next: 'Saga: Compensated.' });
    }

    /* ---------- akış ---------- */
    if (opts.loseRequest) {
      w.q.req.shift();
      step({ dt: 0.2, actor: 'payment', kind: 'fail', title: 'PaymentRequested KAYBOLDU (ör. kuyruk yanlışlıkla purge edildi)', packets: [pkt('q_req', 'q_dlq', 'err', '✖ kayıp')],
        what: 'Order bunu bilmez; sadece cevap gelmediğini görür. Kayıp mı, gecikme mi? Order açısından ayırt edilemez.', code: 'timeout', next: 'Kurtarma işi PaymentTimeout sonra harekete geçer.' });
      step({ dt: C.paymentTimeout, actor: 'recovery', kind: 'timer', title: `${C.paymentTimeout / 60} dk sonra: kurtarma işi “awaiting_payment, cevapsız” siparişi buldu`, packets: [pkt('recovery', 'orderdb', 'timer', 'Unfinished()')],
        what: 'Recover(): updated_at > PaymentTimeout ve payment_requests < 3 → PaymentRequested’i TEKRAR iste (aynı idempotency key).', code: 'timeout', next: '' });
      w.order.payReq++; w.order.outbox.push('PaymentRequested');
      step({ dt: 0.02, actor: 'recovery', kind: 'tx', tx: 'order', title: 'Yerel transaction (Order DB): payment_requests = 2 + PaymentRequested outbox’a', packets: [pkt('recovery', 'orderdb', 'db', 'RequestPaymentAgain')], what: 'Yeni istek AYNI idempotency key’i taşır: Payment ödeme zaten yapılmışsa tekrar çekmez, sonucu yeniden gönderir.', data: [`payment_requests = ${w.order.payReq}`, 'saga_recovery_actions_total{action="payment_requested_again"} +1'], code: 'timeout', next: '' });
      relay('order');
    }

    let r = paymentConsume();
    if (opts.dupRequest) paymentConsume('İKİNCİ kopyayı aldı');
    if (r === 'delayed') {
      for (let k = 2; k <= C.maxPaymentRequests; k++) {
        step({ dt: C.paymentTimeout, actor: 'recovery', kind: 'timer', title: `Timeout: ${C.paymentTimeout / 60} dk cevap yok → PaymentRequested tekrar (${k}/${C.maxPaymentRequests})`, packets: [pkt('recovery', 'orderdb', 'timer', 'Unfinished()')],
          what: 'Timeout bir HATA kararı değil, bir BEKLEME sınırıdır. Kurtarma işi önce yeniden sorar, aynı key ile.', code: 'timeout', next: '' });
        w.order.payReq = k; w.order.outbox.push('PaymentRequested');
        step({ dt: 0.02, actor: 'recovery', kind: 'tx', tx: 'order', title: `payment_requests = ${k} + PaymentRequested outbox’a`, packets: [pkt('recovery', 'orderdb', 'db', 'RequestPaymentAgain')], what: 'Payment hâlâ ilk isteği işliyor; yeni kopyalar kuyrukta bekleyecek.', data: [`payment_requests = ${k}`], code: 'timeout', next: '' });
        relay('order');
      }
      step({ dt: C.paymentTimeout, actor: 'recovery', kind: 'timer', title: `Son istek de cevapsız: ${C.maxPaymentRequests} istek doldu → payment_timeout ile iptal`, packets: [pkt('recovery', 'orderdb', 'timer', 'Unfinished()')], what: 'Order bilinmeyen bir sonuçla sonsuza dek bekleyemez; karar verir. Ama kararı geri döndürülebilir yapar: geç gelen başarı iade edilir.', code: 'timeout', next: '' });
      w.order.status = 'cancelled'; w.order.code = 'payment_timeout'; w.order.outbox.push('OrderCancelled'); hist('awaiting_payment', 'cancelled', 'payment_timeout');
      step({ dt: 0.02, actor: 'recovery', kind: 'tx', tx: 'order', title: 'Yerel transaction (Order DB): cancelled(payment_timeout)', packets: [pkt('recovery', 'orderdb', 'db', 'UPDATE + outbox')], what: 'Sipariş iptal; stok telafi edilecek.', data: ['status → cancelled', 'orders_finished_total{status="cancelled",code="payment_timeout"} +1'], code: 'timeout', next: 'Compensation: stok bırakılacak.' });
      compensateRelease();
      relay('order');
      // geç gelen sonuç
      w.pay.providerCalls++; w.pay.charges = 1; w.pay.status = 'succeeded'; paymentDone = true; w.pay.outbox.push('PaymentSucceeded');
      step({ dt: 30, actor: 'payment', kind: 'tx', tx: 'payment', title: 'Sağlayıcı sonunda cevap verdi: ÇEKİM BAŞARILI → succeeded + PaymentSucceeded', packets: [pkt('provider', 'payment', 'http', 'approved (geç)', 0, 0.45), pkt('payment', 'paydb', 'db', 'UPDATE + outbox', 0.5, 0.45)],
        what: 'İşte belirsizlik: Order “ödeme olmadı” varsaydıysa yanılırdı. Para çekildi.', data: ['payments.status = succeeded', 'charges = 1'], code: 'payment', next: '' });
      relay('payment');
      while (w.q.req.length) paymentConsume('bekleyen tekrar isteği aldı');
      orderConsume();
      relay('order');
      paymentConsume();
      orderConsume();
      return finish();
    }

    if (opts.dupResult && w.q.res.length) {
      w.q.res.push(w.q.res[0]);
      step({ dt: 0.1, actor: 'order', kind: 'warn', title: 'RabbitMQ aynı PaymentSucceeded’i İKİ KEZ teslim edecek', packets: [pkt('ex', 'q_res', 'amqp', 'kopya')],
        what: 'Örneğin Order ack göndermeden bağlantı koptu: broker mesajı yeniden teslim eder. RabbitMQ “en az bir kez” teslim eder.', code: 'idempotency', next: '' });
    }
    const after = orderConsume();
    if (after === 'refund' || w.order.refund === 'requested') {
      relay('order');
      paymentConsume();
      if (opts.refundFails) {
        for (let k = 2; k <= C.maxRefundRequests; k++) {
          step({ dt: C.paymentTimeout, actor: 'recovery', kind: 'timer', title: `Kurtarma işi: PaymentRefunded gelmedi → iadeyi tekrar iste (${k}/${C.maxRefundRequests})`, packets: [pkt('recovery', 'orderdb', 'timer', 'Unfinished()')],
            what: 'refund_state = requested ve PaymentTimeout geçti → RefundRequested tekrar (saga_recovery_actions_total{action="refund_requested_again"}). RefundNotAnswered uyarısı tetiklenir.', code: 'timeout', next: '' });
          w.order.refundReq = k; w.order.outbox.push('RefundRequested');
          step({ dt: 0.02, actor: 'recovery', kind: 'tx', tx: 'order', title: `refund_requests = ${k} + RefundRequested outbox’a`, packets: [pkt('recovery', 'orderdb', 'db', 'RequestRefundAgain')], what: '', data: [`refund_requests = ${k}`], code: 'timeout', next: '' });
          relay('order');
          w.q.req.shift();
          step({ dt: 0.5, actor: 'payment', kind: 'fail', title: 'Payment yine iade edemedi (retry → DLQ)', packets: [pkt('q_req', 'payment', 'amqp', 'deliver', 0, 0.4), pkt('payment', 'q_dlq', 'err', 'x-error', 0.5, 0.45)], what: 'Sağlayıcı iade tarafında kalıcı olarak cevapsız.', code: 'retry', next: '' });
          w.q.dlq.push('RefundRequested');
        }
        w.manual = true;
        step({ dt: 0.1, actor: 'order', kind: 'manual', title: 'REFUND STUCK: “a person must check” — Saga MANUEL MÜDAHALE bekliyor', packets: [pkt('order', 'orderdb', 'err', 'ERROR log')],
          what: 'MaxRefundRequests (5) doldu. Order ERROR seviyesinde “REFUND STUCK: Payment did not answer the last refund request; a person must check” loglar. Sipariş cancelled, refund_state = requested KALIR: sistem bunu sessizce “tamam” saymaz.', data: ['status = cancelled', 'refund_state = requested', 'refund_requests = 5', 'payment.requests.dlq = 5'], code: 'manual', next: 'Bir insan sağlayıcıyı / DLQ’yu inceleyip iadeyi elle tamamlamalı.' });
        return finish();
      }
      orderConsume();
    }
    relay('order');
    return finish();

    function finish() {
      const final = clone(w);
      return { steps, final, state: S.sagaState(w), checks: S.invariants(w) };
    }
  };

  /** Invariant'lar: Testing Lab'de her senaryonun son durumu bunlarla denetlenir. */
  S.invariants = function (w) {
    const st = S.sagaState(w);
    const o = w.order, p = w.pay, i = w.inv;
    return [
      { id: 'I1', t: 'Saga tamamlandıysa tüm iş adımları başarılı olmalı', ok: st !== 'completed' || (o.res === 'committed' && p.status === 'succeeded' && i.resv === 'committed'), why: 'completed ⇒ stok committed, ödeme succeeded' },
      { id: 'I2', t: 'Compensated ise tanımlı telafiler tamamlanmış olmalı', ok: st !== 'compensated' || (i.held === 0 && (p.charges === 0 || p.refunds === p.charges)), why: 'compensated ⇒ tutulan stok yok, çekilen para iade edildi' },
      { id: 'I3', t: 'Başarısız/belirsiz Saga sessizce başarılı işaretlenmemeli', ok: !(o.status === 'cancelled' && p.charges > p.refunds && !['compensating', 'manual'].includes(st)) && !w.stuck, why: 'para alınıp iade edilmediyse durum compensating/manual olmalı; görünmez takılma olmamalı' },
      { id: 'I4', t: 'Tekrarlanan mesajlar duplicate iş etkisi yaratmamalı', ok: p.charges <= 1 && p.refunds <= p.charges, why: 'en fazla bir çekim, çekimden fazla iade yok' },
      { id: 'I5', t: 'Stok korunumu', ok: i.avail + i.held + i.sold === STOCK, why: `available + held + sold = ${STOCK}` },
    ];
  };
})();
