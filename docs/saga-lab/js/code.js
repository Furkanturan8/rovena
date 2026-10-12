/*
 * Code Lab: Rovena'daki GERÇEK Saga kodundan kısaltılmış parçalar (… ile işaretli yerler çıkarıldı) ve her biri için
 * beş soru: ne yapar, ne zaman çalışır, başarısız olursa, aynı mesaj tekrar gelirse, yeniden başlatınca.
 * `real: false` olanlar projede YOK; öneri olarak işaretlidir.
 */
(function () {
  const { h, clear } = RL;
  const L = (RL.codeLib = {});
  const add = (id, o) => (L[id] = Object.assign({ id, real: true }, o));

  add('state', {
    group: 'Model', title: 'Saga durum modeli', file: 'backend/services/order/internal/domain/order.go · migrations/00001, 00002',
    code: `type Status string
const (
    StatusPending         Status = "pending"          // sipariş var, stok henüz tutulmadı
    StatusAwaitingPayment Status = "awaiting_payment" // stok tutuldu, PaymentRequested kuyruğa alındı
    StatusPaid            Status = "paid"             // ödeme başarılı, stok commit'i hâlâ borç
    StatusConfirmed       Status = "confirmed"        // son durum
    StatusCancelled       Status = "cancelled"        // son durum
)
// Stok tarafı durumdan AYRI izlenir: kurtarma işi Inventory'ye commit/release borcunu bilsin diye.
type ReservationState string // none | held | committed | released
type RefundState string      // none | requested | refunded

-- SQL: veritabanı tutarsız durumu reddeder
CONSTRAINT orders_cancelled_has_reason CHECK (status <> 'cancelled' OR failure_code <> ''),
CONSTRAINT orders_refund_only_when_cancelled CHECK (refund_state = 'none' OR status = 'cancelled'),
payment_requests int NOT NULL DEFAULT 0,  -- kurtarma işinin tekrar sayısını sınırlar
refund_requests  int NOT NULL DEFAULT 0`,
    q: {
      what: 'Saga’nın durumu ayrı bir “saga” tablosunda değil, orders satırında tutulur: status (iş akışının nerede olduğu) + reservation_state (stok borcu) + refund_state (para borcu). Genel “Compensating / Compensated / Manual” etiketleri bu üçünden türetilir.',
      when: 'Her durum geçişinde (checkout, ödeme sonucu, iptal, kurtarma).',
      fail: 'CHECK kısıtları imkânsız kombinasyonları (sebepsiz iptal, iptal edilmemiş siparişe iade) veritabanı seviyesinde reddeder.',
      dup: 'Durum alanları tekrar teslimde korumalı geçişlerin dayanağıdır: “from” durumu tutmuyorsa hiçbir şey değişmez.',
      restart: 'Durum kalıcı (PostgreSQL). Yeniden başlayan servis hafızada bir şey kaybetmez; orders_unfinished_idx ile yarım kalan siparişler bulunur.',
    },
  });
  add('checkout', {
    group: 'Akış', title: 'Saga başlatma: Checkout', file: 'backend/services/order/internal/application/order_service.go · Checkout',
    code: `func (s *OrderService) Checkout(ctx context.Context, in CheckoutInput) (order *domain.Order, created bool, err error) {
    if o, err := s.existing(ctx, in); o != nil || err != nil {
        return o, false, err // aynı Idempotency-Key: mevcut siparişi döndür (ve pending ise ilerlet)
    }
    cart, err := s.repo.GetCart(ctx, in.CustomerID)
    …
    items, total, currency, err := s.price(ctx, cart) // Catalog'dan güncel fiyat
    …
    o := &domain.Order{ID: id, Status: domain.StatusPending, ReservationState: domain.ReservationNone, …}
    if err := s.repo.Create(ctx, o, cart); err != nil { … } // yerel transaction #1
    s.advance(ctx, o)                                       // ilk Saga adımı
    fresh, err := s.repo.GetByID(ctx, o.ID)
    return fresh, true, err
}`,
    q: {
      what: 'Sepeti siparişe çevirir ve Saga’yı başlatır. Sipariş “pending” olarak tek bir yerel transaction’da yazılır.',
      when: 'POST /api/v1/checkout (Idempotency-Key zorunlu).',
      fail: 'Catalog cevap veremezse 503 döner ve hiçbir şey yazılmaz. Inventory cevap veremezse sipariş “pending” kalır ve 201 döner; Saga arka planda kurtarma işiyle devam eder.',
      dup: 'Aynı key ile ikinci istek existing() ile aynı siparişi bulur; farklı ödeme yöntemiyle aynı key 422 (ErrIdempotencyKeyReused). UNIQUE (customer_id, idempotency_key) yarışı da çözer.',
      restart: 'Create commit olduktan sonra çökme olursa sipariş “pending” kalır; kurtarma işi advance’i tekrar çağırır (Reserve sipariş başına idempotent).',
    },
  });
  add('advance', {
    group: 'Akış', title: 'Adım yürütme: advance (stok → ödeme isteği)', file: 'backend/services/order/internal/application/order_service.go · advance',
    code: `func (s *OrderService) advance(ctx context.Context, o *domain.Order) {
    res, err := s.inventory.Reserve(ctx, o.ID, lines) // senkron REST, sipariş başına idempotent
    var shortage *StockShortage
    if errors.As(err, &shortage) {
        s.cancel(ctx, o, []domain.Status{domain.StatusPending}, domain.CodeOutOfStock, msg)
        return
    }
    if err != nil {
        s.log.WarnContext(ctx, "stock reservation failed, order stays pending", …)
        return // kurtarma işi tekrar deneyecek
    }
    ev, err := s.paymentRequested(ctx, o)
    …
    changed, err := s.repo.MarkReserved(ctx, o.ID, res.ID, ev) // durum + olay TEK transaction
    …
    if !changed {
        s.releaseIfOrphaned(ctx, o.ID, res.ID) // bu arada iptal edildiyse yetim rezervasyonu bırak
    }
}`,
    q: {
      what: 'Saga’nın ilk adımı: Inventory’de stok tutar, sonra durumu awaiting_payment yapıp PaymentRequested’i outbox’a yazar.',
      when: 'Checkout içinde, aynı key’le tekrar eden istekte ve kurtarma işinin “pending” siparişler için yaptığı turda.',
      fail: 'Stok yoksa: sipariş iptal (telafi edilecek bir şey yok). Inventory ulaşılamazsa: sipariş pending kalır, hata döndürülmez. MarkReserved başarısız olursa: rezervasyon var ama kayıt yok; Reserve idempotent olduğu için sonraki deneme aynı rezervasyonu bulur.',
      dup: 'Reserve sipariş başına idempotent: ikinci çağrı aynı rezervasyonu döner. MarkReserved yalnızca “pending”den geçer; ikinci kez değişiklik yapmaz.',
      restart: 'Durum DB’de; kurtarma işi “pending” siparişi bulur ve advance’i tekrar çağırır. PendingMaxAge (5 dk) aşılırsa checkout_timeout ile iptal eder.',
    },
  });
  add('transition', {
    group: 'Akış', title: 'Korumalı durum geçişi + outbox (tek transaction)', file: 'backend/services/order/internal/repository/postgres/order_repository.go · apply',
    code: `func (r *OrderRepository) apply(ctx context.Context, id uuid.UUID, t transition) (bool, error) {
    changed := false
    err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
        // satırı kilitle: eşzamanlı "ödendi" ve "iptal" yarışında tek kazanan olsun
        err := tx.NewSelect().TableExpr("orders").Column("status", "reservation_state").
            Where("id = ?", id).For("UPDATE").Scan(ctx, &cur.Status, &cur.ReservationState)
        …
        if !containsStatus(t.from, domain.Status(cur.Status)) {
            return nil // geçiş bu durumdan yapılamaz: sessizce hiçbir şey yapma
        }
        q := tx.NewUpdate().Table("orders").Set("status = ?", string(t.to)).Set("updated_at = now()")…
        addHistory(ctx, tx, id, &from, t.to, t.reason)  // order_status_history
        if t.event != nil {
            outbox.Add(ctx, tx, *t.event)                // AYNI transaction
        }
        changed = true
        return nil
    })
    return changed, err
}`,
    q: {
      what: 'Bir Saga geçişinin atomik birimi: durum değişikliği, geçmiş kaydı ve yayınlanacak olay ya birlikte commit olur ya hiç.',
      when: 'MarkReserved, MarkPaid, Confirm, Cancel, RequestRefund… her geçişte.',
      fail: 'Transaction geri alınır: ne durum değişir ne olay outbox’a girer (TestTransition_StatusAndEventStandOrFallTogether).',
      dup: '“from” listesi tutmazsa changed=false döner ve olay yazılmaz: aynı mesajın ikinci işlenmesi iş etkisi yaratmaz. FOR UPDATE eşzamanlı iki teslimi sıraya koyar (TestSaga_ConcurrentDuplicatePaymentResultsConfirmOnce).',
      restart: 'Yarım transaction yoktur: ya commit oldu (olay outbox’ta, relay gönderir) ya olmadı (mesaj ack edilmediği için yeniden teslim edilir).',
    },
  });
  add('outbox', {
    group: 'Güvenilirlik', title: 'Transactional Outbox: outbox.Add', file: 'backend/shared/outbox/outbox.go · Add',
    code: `// Add stores env in the outbox. Pass the transaction that performs the business change
// as db: that shared transaction is the whole point. Calling it with a plain *bun.DB
// works but gives up the guarantee.
func Add(ctx context.Context, db bun.IDB, env messaging.Envelope) error {
    if err := env.Validate(); err != nil {
        return err
    }
    body, err := json.Marshal(env)
    …
    _, err = db.NewInsert().Model(&row{ID: env.ID, Type: env.Type, Envelope: string(body)}).Exec(ctx)
    return err
}

// YANLIŞ (dual write) — Rovena'da YOK, karşılaştırma için:
//   tx.Commit()                    // 1) DB'ye yazdı
//   broker.Publish(ev)             // 2) burada çökerse olay sonsuza dek kaybolur`,
    q: {
      what: 'Olayı broker’a değil, iş değişikliğiyle aynı veritabanı transaction’ına bir satır olarak yazar. Broker’a gönderme işini sonra relay yapar.',
      when: 'Olay üreten her geçişte (PaymentRequested, OrderCancelled, RefundRequested, PaymentSucceeded…).',
      fail: 'Insert başarısızsa tüm transaction geri alınır; “DB değişti ama olay yok” durumu oluşamaz.',
      dup: 'Outbox satırının id’si = event_id (UNIQUE). Relay’in iki kez yayınlaması mümkündür; tüketiciler event_id/iş anahtarı ile idempotent olmalıdır.',
      restart: 'Bekleyen olaylar tabloda durur; relay açılışta sweep yapar ve gönderir.',
    },
  });
  add('relay', {
    group: 'Güvenilirlik', title: 'Outbox relay: yayınlama ve yeniden deneme', file: 'backend/shared/outbox/relay.go · Run, RunOnce',
    code: `// Run publishes pending events until ctx is cancelled. It sweeps immediately, so events
// left behind by a crash are sent on startup.
func (r *Relay) Run(ctx context.Context) {
    r.sweep(ctx)
    ticker := time.NewTicker(r.interval)
    for { select { case <-ctx.Done(): return; case <-ticker.C: r.sweep(ctx) } }
}

// RunOnce … stops at the first event the broker does not accept, so order is kept.
func (r *Relay) sweep(ctx context.Context) {
    n, err := r.RunOnce(ctx)
    if err != nil {
        // The rows stay pending and are retried on the next tick.
        r.log.ErrorContext(ctx, "outbox publish failed", "error", err, "published", n)
    }
}`,
    q: {
      what: 'outbox_events’teki published_at = NULL satırları sırayla broker’a yayınlar, publisher confirm gelince işaretler.',
      when: 'Sürekli, belirli aralıkla; ve süreç açılışında hemen.',
      fail: 'Broker kabul etmezse satır bekler, sonraki turda tekrar denenir; outbox_pending_events ve outbox_oldest_pending_age_seconds yükselir (OutboxBacklog / OutboxEventStuck uyarıları).',
      dup: 'Yayınlandı ama published_at yazılmadan çöktüyse olay tekrar yayınlanır: “en az bir kez”. Bu yüzden Payment’ın idempotency key’i ve Order’ın korumalı geçişleri şart.',
      restart: 'Run önce sweep yapar: çöküşten kalanlar açılışta gönderilir.',
    },
  });
  add('payment', {
    group: 'Akış', title: 'Katılımcı: Payment ödeme isteğini işler', file: 'backend/services/payment/internal/application/payment_service.go · Process, charge',
    code: `func (s *PaymentService) Process(ctx context.Context, in ProcessInput) error {
    …
    p, created, err := s.repo.Begin(ctx, candidate)   // (idempotency_key) ile kayıt aç / bul
    if !created && !p.SameRequest(candidate) {
        return fmt.Errorf("%w: %q", domain.ErrIdempotencyKeyReused, in.IdempotencyKey)
    }
    if p.Status != domain.StatusPending {
        return s.republish(ctx, p)                    // zaten bitmiş: sonucu tekrar yayınla, ÇEKME
    }
    return s.charge(ctx, p)
}

func (s *PaymentService) charge(ctx context.Context, p *domain.Payment) error {
    paid, err := s.repo.HasSucceeded(ctx, p.OrderID)  // bir sipariş bir kez ödenir
    …
    res, err := s.provider.Charge(cctx, domain.ChargeRequest{IdempotencyKey: p.IdempotencyKey, …})
    switch {
    case err != nil:
        if p.Attempts >= s.cfg.MaxAttempts {
            return s.complete(ctx, p, domain.Failed(domain.CodeProviderUnavailable, …))
        }
        return fmt.Errorf("charge payment …: %w", err)   // mesaj retry kuyruğuna
    case res.Approved:
        return s.complete(ctx, p, domain.Succeeded(res.Reference))
    default:
        return s.complete(ctx, p, domain.Failed(res.DeclineCode, res.DeclineMessage))
    }
}`,
    q: {
      what: 'Saga’nın ikinci adımı (katılımcı tarafı): ödeme kaydını açar, sağlayıcıyı aynı key ile çağırır, sonucu ve olayı tek transaction’da yazar.',
      when: 'payment.requests kuyruğundan PaymentRequested geldiğinde.',
      fail: 'Sağlayıcı cevapsızsa hata döner → mesaj retry kuyruğuna; PAYMENT_MAX_ATTEMPTS dolunca “provider_unavailable” ile PaymentFailed. Kart reddi: PaymentFailed.',
      dup: 'Begin aynı key’i bulur; tamamlanmışsa yalnızca sonucu yeniden yayınlar (TestRepeatedRequestChargesOnceAndSendsTheResultAgain). Eşzamanlı iki istek tek çekim yapar (TestConcurrentRequestsForOneKeyChargeOnce).',
      restart: '“pending” kalmış ödeme bir sonraki teslimde devam ettirilir; sağlayıcı aynı key ile önceki sonucu döner (TestPendingPaymentLeftByACrashIsContinued).',
    },
  });
  add('idempotency', {
    group: 'Güvenilirlik', title: 'Idempotency: üç katman', file: 'order/domain/order.go · shared/outbox/inbox.go · payment Begin',
    code: `// 1) İş anahtarı: siparişten türetilir, tüm PaymentRequested'ler aynı anahtarı taşır
func (o *Order) PaymentIdempotencyKey() string { return "order:" + o.ID.String() }

// 2) İşleme kaydı (Inbox): "işlendi" işareti handler'ın değişiklikleriyle AYNI transaction'da
func (i *Inbox) Handle(ctx context.Context, env messaging.Envelope, fn func(ctx context.Context, tx bun.Tx) error) (bool, error) {
    err := i.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
        res, err := tx.NewRaw(
            "INSERT INTO processed_events (consumer, event_id) VALUES (?, ?) ON CONFLICT DO NOTHING",
            i.consumer, env.ID).Exec(ctx)
        …
        if n == 0 { return nil }  // daha önce işlenmiş: hiçbir şey yapma
        ran = true
        return fn(ctx, tx)
    })
    …
}

// 3) Korumalı geçiş: Order'ın sonuç handler'ı Inbox yerine bunu kullanır
//    "The service's steps are guarded transitions plus idempotent Inventory calls,
//     so a repeated delivery is harmless" (transport/consumer/payment_result.go)`,
    q: {
      what: 'Aynı mantıksal işlemin iki kez işlenmesinin iş etkisini tek tutar. Rovena üçünü de kullanır: anahtar (Payment), işleme kaydı (shared Inbox), korumalı durum geçişi (Order).',
      when: 'Her mesaj tüketiminde ve her tekrar edilebilir dış çağrıda (sağlayıcı, Inventory).',
      fail: 'Inbox: fn başarısızsa işaret de geri alınır, mesaj sıfırdan tekrar denenir.',
      dup: 'İkinci teslim: Inbox satırı bulur ve atlar; Payment saklı sonucu tekrar gönderir; Order’ın geçişi “from” tutmadığı için hiçbir şey yapmaz.',
      restart: 'İşaretler DB’de; yeniden başlatma idempotency’yi bozmaz.',
    },
  });
  add('onresult', {
    group: 'Akış', title: 'Başarı / başarısızlık geçişleri: ödeme sonucu', file: 'backend/services/order/internal/application/saga.go · OnPaymentSucceeded, OnPaymentFailed',
    code: `func (s *OrderService) OnPaymentSucceeded(ctx context.Context, p events.PaymentSucceededPayload) error {
    o, err := s.paymentOrder(ctx, p.OrderID)
    …
    if p.AmountCents != o.TotalCents || p.Currency != o.Currency {
        s.log.ErrorContext(ctx, "PAYMENT AMOUNT MISMATCH: order not confirmed", …)
        return fmt.Errorf("%w: …", ErrUnexpectedEvent)   // kalıcı hata → DLQ, insan bakar
    }
    switch o.Status {
    case domain.StatusAwaitingPayment:
        s.repo.MarkPaid(ctx, o.ID)
    case domain.StatusPaid:       // tekrar teslim ya da commit'i başarısız olmuş önceki deneme
    case domain.StatusConfirmed:
        return nil                // zaten bitti
    case domain.StatusCancelled:  // ödeme iptalden SONRA geldi: sipariş iptal kalır, para geri
        return s.requestRefund(ctx, o, o.FailureCode)
    }
    return s.settleByID(ctx, o.ID)
}

func (s *OrderService) OnPaymentFailed(ctx context.Context, p events.PaymentFailedPayload) error {
    …
    case domain.StatusAwaitingPayment:
        s.cancel(ctx, o, []domain.Status{domain.StatusAwaitingPayment}, code, p.FailureMessage)
    case domain.StatusCancelled: // tekrar teslim ya da release'i başarısız olmuş önceki deneme
    …
    return s.settleByID(ctx, o.ID)   // iptal edilmiş siparişin stoğunu bırak (telafi)
}`,
    q: {
      what: 'Orchestrator’ın karar noktası: ödeme sonucuna göre bir sonraki adımı (commit) ya da telafiyi (release / refund) seçer.',
      when: 'order.payment-results kuyruğundan PaymentSucceeded / PaymentFailed / PaymentRefunded geldiğinde.',
      fail: 'Inventory’ye ulaşılamazsa hata döner → mesaj retry; sipariş “paid” ya da “cancelled+held” kalır, kurtarma işi bitirir. Tutar uyuşmazlığı kalıcı hatadır → DLQ.',
      dup: 'Her durum dalı tekrar teslimi tolere eder: confirmed → nil; paid → settle tekrar (idempotent); cancelled → refund yalnızca ilk kez kuyruğa girer.',
      restart: 'Mesaj ack edilmediyse yeniden teslim edilir; durum DB’de olduğu için handler doğru dalı seçer.',
    },
  });
  add('settle', {
    group: 'Akış', title: 'Son adım ve stok telafisi: settle', file: 'backend/services/order/internal/application/saga.go · settle',
    code: `func (s *OrderService) settle(ctx context.Context, o *domain.Order) error {
    if o.ReservationState != domain.ReservationHeld || o.ReservationID == nil {
        return nil
    }
    switch o.Status {
    case domain.StatusPaid:
        err := s.inventory.Commit(ctx, *o.ReservationID)
        if errors.Is(err, ErrReservationGone) {
            // rezervasyon süresi ödeme gelmeden doldu: müşteri başkasına satılmış olabilecek stoğa ödedi
            s.cancel(ctx, o, []domain.Status{domain.StatusPaid}, domain.CodeReservationLost, …)
            s.repo.MarkReleased(ctx, o.ID)
            return s.requestRefund(ctx, o, domain.CodeReservationLost)
        }
        if err != nil { return err }               // retry
        ev, _ := s.event(ctx, o, events.OrderConfirmed, …)
        _, err = s.repo.Confirm(ctx, o.ID, ev)     // from=[paid], reservation=held
        return err
    case domain.StatusCancelled:
        if err := s.inventory.Release(ctx, *o.ReservationID); err != nil {
            return err                             // retry; kurtarma işi de dener
        }
        _, err := s.repo.MarkReleased(ctx, o.ID)
        return err
    }
    return nil
}`,
    q: {
      what: 'Kararı verilmiş siparişin Inventory tarafını tamamlar: ödenmişse commit (ileri adım), iptalse release (telafi).',
      when: 'Ödeme sonucu işlenirken, müşteri iptalinde ve kurtarma işinde.',
      fail: 'Commit’te rezervasyon yoksa: üçüncü adım başarısız → iptal + iade (telafi). Inventory cevapsızsa: hata döner, sonra tekrar denenir.',
      dup: 'Commit/Release idempotent; Release “zaten yoksa” da başarılı. Confirm yalnızca paid+held iken uygulanır.',
      restart: 'reservation_state = held + status = paid/cancelled, kurtarma işinin “borç var” işaretidir (orders_unfinished_idx).',
    },
  });
  add('compensation', {
    group: 'Telafi', title: 'Compensation handler’ları: cancel, release, refund', file: 'order_service.go · cancel, requestRefund · saga.go · OnPaymentRefunded',
    code: `func (s *OrderService) cancel(ctx context.Context, o *domain.Order, from []domain.Status, code, message string) bool {
    ev, err := s.event(ctx, o, events.OrderCancelled, …)
    changed, err := s.repo.Cancel(ctx, o.ID, from, code, message, ev)
    if changed {
        ordersFinished.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "cancelled"), attribute.String("code", code)))
        if code != domain.CodeCancelledByCustomer {
            s.restoreCart(ctx, o)   // iş kuralı: müşteri tekrar deneyebilsin
        }
    }
    return changed
}

// Safe to repeat: only the first call for an order queues the request.
func (s *OrderService) requestRefund(ctx context.Context, o *domain.Order, reason string) error {
    ev, err := s.event(ctx, o, events.RefundRequested, …)
    changed, err := s.repo.RequestRefund(ctx, o.ID, ev)   // refund_state: none → requested + outbox
    …
}

func (s *OrderService) OnPaymentRefunded(ctx context.Context, p events.PaymentRefundedPayload) error {
    …
    changed, err := s.repo.MarkRefunded(ctx, o.ID)        // requested → refunded
    …
}`,
    q: {
      what: 'Telafiler: siparişi iptal (son durum), stoğu bırak (release), parayı iade et (RefundRequested komutu). Hiçbiri geçmişi silmez; her biri YENİ bir iş işlemidir (iade bir para hareketi, release bir stok hareketi).',
      when: 'Ödeme reddi, rezervasyon kaybı, timeout, müşteri iptali, iptalden sonra gelen ödeme.',
      fail: 'Release başarısızsa sipariş cancelled+held kalır, kurtarma işi tekrar dener. İade cevapsızsa kurtarma işi MaxRefundRequests (5) kez tekrar ister, sonra “REFUND STUCK” loglar.',
      dup: 'Cancel ve RequestRefund korumalı: ikinci çağrı olay yazmaz (TestConcurrentRefundRequestsQueueOneEvent).',
      restart: 'Telafi borçları (reservation_state=held, refund_state=requested) DB’de; kurtarma işi bulur.',
    },
  });
  add('timeout', {
    group: 'Güvenilirlik', title: 'Timeout ve kurtarma işi', file: 'backend/services/order/internal/application/recovery.go · recoverOne',
    code: `case domain.StatusAwaitingPayment:
    if now.Sub(o.UpdatedAt) < s.cfg.PaymentTimeout {      // 2 dk dolmadıysa bekle
        return false
    }
    if o.PaymentRequests >= s.cfg.MaxPaymentRequests {    // 3 istek de cevapsız
        if s.cancel(ctx, o, []domain.Status{domain.StatusAwaitingPayment}, domain.CodePaymentTimeout, …) {
            s.settleOrLog(ctx, o)                          // stoğu bırak
        }
        countRecovery(ctx, "payment_timeout")
        return true
    }
    countRecovery(ctx, "payment_requested_again")
    ev, err := s.paymentRequested(ctx, o)                 // AYNI idempotency key
    s.repo.RequestPaymentAgain(ctx, o.ID, ev)             // payment_requests+1 + outbox
    return true`,
    q: {
      what: 'Cevabı gelmeyen adımı tespit eder. Önce aynı anahtarla yeniden sorar; sınır dolunca karar verip telafiyi başlatır.',
      when: 'ORDER_RECOVERY_INTERVAL (15 sn) aralıkla; yalnızca ORDER_STUCK_AFTER (30 sn) boyunca değişmeyen siparişler için.',
      fail: 'Kurtarma turu hata verirse loglanır, sonraki turda tekrar. settleOrLog başarısızlıkta updated_at’i ileri alır: her turda aynı siparişe saldırmaz (basit backoff).',
      dup: 'Tekrar istekler aynı key’i taşır: Payment ödemeyi yalnızca bir kez yapar (TestRecovery_RepeatedPaymentRequestsReuseTheSameIdempotencyKey). Timeout’tan sonra gelen başarı iade edilir.',
      restart: 'İş durumsuzdur: her turda DB’den “takılı” siparişleri okur.',
    },
  });
  add('retry', {
    group: 'Güvenilirlik', title: 'Retry, backoff ve DLQ (mesaj katmanı)', file: 'backend/shared/messaging/{topology,consumer}.go · order/transport/consumer',
    code: `// Her kuyruk için: iş kuyruğu + .retry (TTL ile bekleme) + .dlq (insan bekleyen)
messaging.Subscription{
    Queue:      "order.payment-results",
    Types:      []string{events.PaymentSucceeded, events.PaymentFailed, events.PaymentRefunded},
    MaxRetries: 5,
    RetryDelay: retryDelay,      // x-message-ttl: süre dolunca iş kuyruğuna geri döner
}

func (b *Broker) settle(ctx context.Context, sub Subscription, d amqp.Delivery, eventType string, cause error, permanent bool) {
    attempts := retryCount(d.Headers)
    key := sub.dlqQueue()
    if !permanent && attempts < sub.MaxRetries {
        key = sub.retryQueue()
        attempts++
    }
    headers[headerRetryCount] = int64(attempts)
    if key == sub.dlqQueue() {
        headers[headerError] = cause.Error()           // x-error
        headers[headerFailedAt] = time.Now().UTC()…    // x-failed-at
    }
    err := b.publish(ctx, "", key, …)   // önce taşı…
    …
    b.ack(ctx, sub, d)                  // …sonra orijinali ack et: kayıp yok, en kötü ihtimalle kopya
}`,
    q: {
      what: 'Geçici hataları sabit gecikmeyle tekrar dener; hakkı biten ya da kalıcı (ErrUnexpectedEvent, messaging.Permanent) hataları DLQ’ya ayırır.',
      when: 'Handler hata döndürdüğünde.',
      fail: 'Taşıma başarısızsa mesaj nack ile kuyruğa döner. DLQ’ya düşen mesaj otomatik işlenmez (DeadLetterQueueNotEmpty).',
      dup: 'Taşıma ile ack arasında çöküş kopya yaratabilir; tüketici idempotent olduğu için zararsız.',
      restart: 'Bekleyen mesajlar kuyruklarda kalıcıdır (durable); consumer yeniden bağlanınca devam eder.',
    },
  });
  add('manual', {
    group: 'Telafi', title: 'Telafi başarısız: manuel müdahale', file: 'backend/services/order/internal/application/recovery.go · StatusCancelled dalı',
    code: `case domain.StatusCancelled:
    …
    // Asked for a refund and no PaymentRefunded came back: ask again, a bounded number of times.
    if o.RefundState == domain.RefundRequested && o.RefundRequests < domain.MaxRefundRequests &&
        now.Sub(o.UpdatedAt) >= s.cfg.PaymentTimeout {
        countRecovery(ctx, "refund_requested_again")
        ev, err := s.event(ctx, o, events.RefundRequested, …, events.RefundRequestedPayload{…, Reason: "no_answer"})
        _, err = s.repo.RequestRefundAgain(ctx, o.ID, ev)
        if o.RefundRequests+1 >= domain.MaxRefundRequests {
            s.log.ErrorContext(ctx, "REFUND STUCK: Payment did not answer the last refund request; a person must check",
                "order_id", o.ID, "amount_cents", o.TotalCents)
        }
    }`,
    q: {
      what: 'Telafinin kendisi başarısız olduğunda sınırsız denemez; sınır dolunca durumu açıkça görünür bırakır: refund_state=requested, ERROR log, RefundNotAnswered uyarısı.',
      when: 'İade istenmiş ve PaymentTimeout boyunca cevap gelmemiş iptal siparişlerinde.',
      fail: 'Rovena’da ayrı bir “requires_manual_intervention” durumu YOK: işaret, refund_state=requested + refund_requests=5 + ERROR log + DLQ. (Laboratuvardaki “Requires manual intervention” etiketi bu birleşimden türetilir.)',
      dup: 'Tekrar istekler aynı sipariş için; Payment iadeyi bir kez yapar (TestRepeatedRefundRequestsRefundOnceAndAnswerEachTime).',
      restart: 'Durum DB’de, log ve metrik sayaçları süreçle kaybolabilir ama refund_state kalıcıdır; insan admin uçlarından görebilir.',
    },
  });
  add('events', {
    group: 'Model', title: 'Komutlar ve olaylar', file: 'backend/shared/events/{payment,order}.go · shared/messaging/event.go',
    code: `// Komutlar (Order → Payment: "şunu yap")
PaymentRequested = "PaymentRequested"
RefundRequested  = "RefundRequested"   // "a command from Order to Payment: give the money for this order back"

// Olaylar (oldu bitti, geçmiş zaman)
PaymentSucceeded, PaymentFailed, PaymentRefunded   // Payment → Order
OrderConfirmed, OrderCancelled                     // Order yayınlar (şu an dinleyen kuyruk yok)

type PaymentSucceededPayload struct {
    PaymentID   uuid.UUID \`json:"payment_id"\`
    OrderID     uuid.UUID \`json:"order_id"\`
    AmountCents int64     \`json:"amount_cents"\`
    Currency    string    \`json:"currency"\`
    ProviderRef string    \`json:"provider_ref"\`
}

// Zarf: event_id (tekrar tespiti), correlation_id (= sipariş id'si), traceparent (iz)
type Envelope struct { ID uuid.UUID; Type string; Version int; CorrelationID string; Traceparent string; … }`,
    q: {
      what: 'Saga’nın mesaj sözleşmesi. Order komut gönderir, Payment olay ile cevap verir: bu, orchestration’ın işaretidir.',
      when: 'Her adım geçişinde.',
      fail: 'Bozuk zarf handler’a hiç gitmeden DLQ’ya düşer. Version alanı payload evrimini mümkün kılar.',
      dup: 'event_id her teslimde aynıdır; correlation_id bir Saga’nın tüm olaylarını birleştirir (Order event() fonksiyonu correlation_id’yi sipariş id’si yapar).',
      restart: 'Zarf kalıcı (outbox satırı / kuyruk mesajı); traceparent sayesinde iz yeniden başlatmadan sonra da aynı trace’e bağlanır.',
    },
  });
  add('instrumentation', {
    group: 'Gözlem', title: 'ÖNERİ: Saga süresi ve devam eden Saga metrikleri', file: 'Projede YOK · order/internal/application (öneri)', real: false,
    code: `// Rovena bugün: orders_finished_total{status,code}, saga_recovery_actions_total{action},
// orders_refunds_requested_total{reason}. Süre ve "devam eden" sayısı YOK. Örnek ekleme:

var sagaDuration = telemetry.DurationHistogram("saga.checkout.duration",
    "Time from checkout to a final order status, by outcome")

// settle() içinde Confirm başarılı olunca (changed == true):
sagaDuration.Record(ctx, time.Since(o.CreatedAt).Seconds(),
    metric.WithAttributes(attribute.String("outcome", "confirmed")))   // sipariş id'si ETİKET DEĞİL

// Devam eden Saga'lar: outbox gauge'u gibi bir ObservableGauge, durum başına sayım
meter.Int64ObservableGauge("saga.checkout.in_progress")  // callback:
//   SELECT status, count(*) FROM orders WHERE status IN ('pending','awaiting_payment','paid') GROUP BY status

// PromQL (Prometheus adları: saga_checkout_duration_seconds, saga_checkout_in_progress)
// histogram_quantile(0.95, sum by (le, outcome) (rate(saga_checkout_duration_seconds_bucket[15m])))
// sum by (status) (saga_checkout_in_progress)`,
    q: {
      what: 'Saga sağlığını toplu olarak ölçmek için iki eksik sinyali tamamlar: tamamlanma süresi (histogram) ve şu an yolda olan Saga sayısı (gauge).',
      when: 'Süre: Saga son duruma ulaştığında bir kez. Gauge: her scrape’te sorgu.',
      fail: 'Gauge sorgusu başarısızsa değer yazılmaz (outbox gauge’undaki gibi scrape bozulmaz).',
      dup: 'Süre yalnızca geçiş gerçekten olduğunda (changed == true) kaydedilir; tekrar teslim iki kez saymaz.',
      restart: 'Histogram süreç belleğinde; yeniden başlatmada sayaç sıfırlanır, rate() bunu doğru ele alır. Gauge DB’den hesaplandığı için kalıcıdır.',
    },
  });

  RL.codeSnippet = function (id) {
    const c = L[id];
    if (!c) return h('div');
    return h('div', { class: 'snip' }, [
      h('div', { class: 'muted mono', style: 'font-size:11.5px' }, [c.real ? h('span', { class: 'tag real' }, 'GERÇEK KOD') : h('span', { class: 'tag sim' }, 'ÖNERİ'), ' ', c.file]),
      RL.code(c.code, 'c-go'),
      h('div', { class: 'muted', style: 'font-size:12.5px' }, ['Ayrıntılı açıklama: ', h('a', { href: '#', onclick: (e) => { e.preventDefault(); RL.openCode(id); } }, 'Code Lab’de aç')]),
    ]);
  };

  const QS = [['what', 'Bu kod ne yapıyor?'], ['when', 'Hangi durumda çalışıyor?'], ['fail', 'Başarısız olursa ne oluyor?'], ['dup', 'Aynı mesaj tekrar gelirse?'], ['restart', 'Yeniden başlatınca kaldığı yerden nasıl devam ediyor?']];
  RL.openCode = (id) => { RL._codeId = id; RL.showTab('code'); if (RL._drawCode) RL._drawCode(); window.scrollTo(0, 0); };

  RL.initCode = function (root) {
    const list = h('div', { class: 'sclist' }), main = h('div');
    root.append(h('div', { class: 'labgrid' }, [list, main]));
    const groups = {};
    Object.values(L).forEach((c) => (groups[c.group] = groups[c.group] || []).push(c));
    let cur = RL._codeId || 'state';
    function draw() {
      clear(list);
      Object.entries(groups).forEach(([g, cs]) => {
        list.append(h('h4', null, g));
        cs.forEach((c) => list.append(h('button', { class: 'sc' + (c.id === cur ? ' on' : ''), onclick: () => { cur = c.id; draw(); } }, [h('b', null, c.title), h('small', null, c.real ? 'Rovena kodu' : 'öneri · projede yok')])));
      });
      const c = L[cur];
      clear(main).append(h('div', { class: 'card' }, [
        h('div', null, c.real ? h('span', { class: 'tag real' }, 'GERÇEK KOD (kısaltılmış)') : h('span', { class: 'tag sim' }, 'ÖNERİ · PROJEDE YOK')),
        h('h2', { style: 'margin-top:6px' }, c.title), h('div', { class: 'muted mono', style: 'font-size:12px' }, c.file),
        RL.code(c.code, 'c-go'),
        h('div', { class: 'qa' }, QS.map(([k, t], i) => h('div', { class: 'q' + ((i % 4) + 1) }, [h('b', null, t), c.q[k]]))),
      ]));
    }
    RL._drawCode = () => { cur = RL._codeId; draw(); };
    draw();
  };
})();
