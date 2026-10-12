/*
 * Saga oynatıcısı: diyagram + transaction sınırları + paket animasyonu + zaman çizelgesi + adım ayrıntısı.
 * Adımları sim.js (Rovena modeli) ya da arch.js (elle yazılmış karşılaştırma akışları) üretir.
 */
(function () {
  const { h, s, clear } = RL;
  const COLOR = { http: '#5ab0ff', amqp: '#ffb454', db: '#6fd3c4', timer: '#f2cc60', comp: '#c792ea', err: '#ff6b6b', cmd: '#ff9e64', evt: '#7ee787' };
  const STEP_MS = 1500, HOLD_MS = 900;
  const ease = (t) => (t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2);

  /* -------- Rovena Saga diyagramı (gerçek bileşenler; sağlayıcı Rovena içindeki sahte sağlayıcıdır) -------- */
  const fmtOut = (n) => (n ? `outbox: ${n} bekleyen` : 'outbox: boş');
  RL.sagaDiagram = {
    viewBox: '0 0 1000 570',
    frames: [
      { id: 'order', x: 150, y: 8, w: 280, h: 252, label: 'Order servisi · kendi PostgreSQL DB’si' },
      { id: 'inventory', x: 690, y: 8, w: 290, h: 196, label: 'Inventory servisi · kendi DB’si' },
      { id: 'mq', x: 150, y: 286, w: 830, h: 84, label: 'RabbitMQ · rovena.events (topic)', mq: true },
      { id: 'payment', x: 430, y: 392, w: 270, h: 170, label: 'Payment servisi · kendi DB’si' },
    ],
    nodes: [
      { id: 'client', x: 20, y: 34, w: 110, h: 44, label: 'İstemci', lines: () => ['POST /checkout'] },
      { id: 'order', x: 170, y: 30, w: 240, h: 44, label: 'Order (Saga yöneticisi)', lines: () => [':8086 · checkout + consumer'] },
      { id: 'orderdb', x: 170, y: 86, w: 240, h: 104, label: 'Order DB', cls: 'db', lines: (w) => [`status: ${w.order.status || '—'}`, `reservation: ${w.order.res} · refund: ${w.order.refund}`, `payment_req: ${w.order.payReq} · refund_req: ${w.order.refundReq}`, fmtOut(w.order.outbox.length)] },
      { id: 'recovery', x: 170, y: 202, w: 240, h: 44, label: '⏱ Kurtarma işi', cls: 'timer', lines: () => ['her 15 sn · timeout 2 dk'] },
      { id: 'inventory', x: 710, y: 30, w: 250, h: 44, label: 'Inventory', lines: () => [':8083 · Reserve/Commit/Release'] },
      { id: 'invdb', x: 710, y: 86, w: 250, h: 104, label: 'Inventory DB', cls: 'db', lines: (w) => [`available: ${w.inv.avail}`, `held: ${w.inv.held} · sold: ${w.inv.sold}`, `rezervasyon: ${w.inv.resv}`] },
      { id: 'ex', x: 168, y: 312, w: 128, h: 46, label: 'exchange', cls: 'queue', lines: () => ['rovena.events'] },
      { id: 'q_req', x: 312, y: 312, w: 210, h: 46, label: 'payment.requests', cls: 'queue', lines: (w) => [`Ready: ${w.q.req.length}`] },
      { id: 'q_res', x: 536, y: 312, w: 210, h: 46, label: 'order.payment-results', cls: 'queue', lines: (w) => [`Ready: ${w.q.res.length}`] },
      { id: 'q_dlq', x: 760, y: 312, w: 206, h: 46, label: 'payment.requests.dlq', cls: 'queue dlq', lines: (w) => [`Ready: ${w.q.dlq.length}`] },
      { id: 'payment', x: 448, y: 412, w: 234, h: 44, label: 'Payment', lines: () => [':8085 · consumer'] },
      { id: 'paydb', x: 448, y: 466, w: 234, h: 88, label: 'Payment DB', cls: 'db', lines: (w) => [`payment: ${w.pay.status}`, `charges: ${w.pay.charges} · refunds: ${w.pay.refunds}`, fmtOut(w.pay.outbox.length)] },
      { id: 'provider', x: 752, y: 440, w: 210, h: 54, label: 'Ödeme sağlayıcısı', cls: 'ext', lines: (w) => [`Rovena’daki sahte sağlayıcı · çağrı: ${w.pay.providerCalls}`] },
    ],
    edges: [['client', 'order'], ['order', 'orderdb'], ['recovery', 'orderdb'], ['order', 'inventory'], ['inventory', 'invdb'], ['orderdb', 'ex'], ['ex', 'q_req'], ['ex', 'q_res'], ['q_req', 'payment'], ['q_res', 'order'], ['payment', 'paydb'], ['payment', 'provider'], ['paydb', 'ex'], ['payment', 'q_dlq']],
    frameOf: { order: 'order', orderdb: 'order', recovery: 'order', inventory: 'inventory', invdb: 'inventory', payment: 'payment', paydb: 'payment' },
  };

  RL.STATE_CLS = { pending: 'st-pending', running: 'st-running', completed: 'st-completed', failed: 'st-failed', compensating: 'st-comp', compensated: 'st-compd', manual: 'st-manual', stuck: 'st-manual' };
  RL.stateChip = (st) => h('span', { class: 'stchip ' + (RL.STATE_CLS[st] || '') }, (RL.saga && RL.saga.STATE_LABEL[st]) || st);

  RL.sagaPlayer = function (root, cfg) {
    const D = cfg.diagram || RL.sagaDiagram;
    const ctr = {}; D.nodes.forEach((n) => (ctr[n.id] = [n.x + n.w / 2, n.y + n.h / 2]));
    const st = { steps: [], i: 0, playing: false, paused: false, speed: 1, clock: 0, last: 0, raf: 0, pk: [] };

    const svg = s('svg', { class: 'diagram saga', viewBox: D.viewBox, role: 'img', 'aria-label': cfg.title || 'Saga diyagramı' });
    const frameEl = {};
    (D.frames || []).forEach((f) => {
      const g = s('g', { class: 'frame-g' + (f.mq ? ' mq' : '') }, [s('rect', { class: 'txframe', x: f.x, y: f.y, width: f.w, height: f.h, rx: 12 }), s('text', { class: 'frame-label', x: f.x + 10, y: f.y + f.h - 6 }, f.label), s('text', { class: 'tx-badge', x: f.x + f.w - 10, y: f.y + 16, 'text-anchor': 'end' }, 'BEGIN … COMMIT')]);
      frameEl[f.id] = g; svg.append(g);
    });
    const gE = s('g'); svg.append(gE);
    (D.edges || []).forEach(([a, b]) => gE.append(s('line', { class: 'edge', x1: ctr[a][0], y1: ctr[a][1], x2: ctr[b][0], y2: ctr[b][1] })));
    const nodeEl = {}, lineEl = {};
    D.nodes.forEach((n) => {
      const g = s('g', { class: 'node ' + (n.cls || ''), tabindex: 0, role: 'button', 'aria-label': n.label });
      g.append(s('rect', { x: n.x, y: n.y, width: n.w, height: n.h, rx: 8 }), s('text', { x: n.x + 10, y: n.y + 17, style: 'text-anchor:start;font-weight:600' }, n.label));
      lineEl[n.id] = [];
      for (let k = 0; k < 4; k++) { const t = s('text', { class: 'sub', x: n.x + 10, y: n.y + 32 + k * 14, style: 'text-anchor:start;font-size:10.5px' }); g.append(t); lineEl[n.id].push(t); }
      g.addEventListener('click', () => cfg.onNode && cfg.onNode(n.id));
      nodeEl[n.id] = g; svg.append(g);
    });
    const gP = s('g'); svg.append(gP);
    const stateBadge = s('g', null, [s('rect', { x: 8, y: D.chipY || 540, width: 128, height: 24, rx: 12, class: 'svg-chip' }), s('text', { x: 72, y: (D.chipY || 540) + 16, 'text-anchor': 'middle', class: 'svg-chip-t' }, '')]);
    svg.append(stateBadge);
    if (!cfg.noCaption) svg.append(s('text', { class: 'caption', x: 8, y: (D.chipY || 540) - 10 }, 'Her çerçeve ayrı DB = ayrı transaction. Tek bir global ACID transaction YOK.'));

    const btnRestart = h('button', { 'aria-label': 'Yeniden başlat' }, '⟲ Baştan');
    const btnPrev = h('button', { 'aria-label': 'Geri' }, '◀ Geri');
    const btnPlay = h('button', { class: 'play' }, '▶ Başlat');
    const btnNext = h('button', { 'aria-label': 'İleri' }, 'İleri ▶');
    const speed = h('select', { 'aria-label': 'Hız' }, [0.5, 1, 2, 4].map((v) => h('option', { value: v, selected: v === 1 }, v + '×')));
    const pos = h('span', { class: 'muted mono' });
    const timeline = h('ol', { class: 'timeline' });
    const detail = h('div', { class: 'card stepdetail' });

    root.append(h('div', { class: 'player' }, [
      h('div', { class: 'card pl-stage' }, [svg, h('div', { class: 'controls' }, [btnRestart, btnPrev, btnPlay, btnNext, h('label', { class: 'muted' }, ['Hız ', speed]), pos])]),
      h('div', { class: 'pl-side' }, [detail, h('div', { class: 'card' }, [h('h4', null, 'Zaman çizelgesi (tıkla)'), timeline])]),
    ]));

    function snapOf(i) { return st.steps[i] ? st.steps[i].snap : null; }

    function goto(i, animate = true) {
      if (!st.steps.length) return;
      st.i = Math.max(0, Math.min(st.steps.length - 1, i));
      const stp = st.steps[st.i];
      st.clock = animate ? 0 : STEP_MS + 1;
      // düğüm durumları
      Object.entries(nodeEl).forEach(([id, g]) => {
        const cls = ['node', ...(D.nodes.find((n) => n.id === id).cls || '').split(' ')];
        if (stp.actor === id || (stp.focus || []).includes(id)) cls.push(stp.kind === 'fail' || stp.kind === 'manual' ? 'err' : 'active');
        g.setAttribute('class', cls.join(' ').trim());
      });
      const fr = stp.tx ? (D.frameOf || {})[stp.tx] || stp.tx : null;
      Object.entries(frameEl).forEach(([id, g]) => g.classList.toggle('txon', id === fr));
      // canlı değerler
      const snap = snapOf(st.i);
      D.nodes.forEach((n) => { const ls = n.lines && snap ? n.lines(snap) : n.lines ? n.lines({}) : []; lineEl[n.id].forEach((t, k) => (t.textContent = ls[k] || '')); });
      stateBadge.setAttribute('class', 'svgstate ' + (RL.STATE_CLS[stp.state] || ''));
      stateBadge.lastChild.textContent = 'Saga: ' + ((RL.saga && RL.saga.STATE_LABEL[stp.state]) || stp.state || '');
      // paketler
      clear(gP); st.pk = [];
      const ps = stp.packets || [];
      ps.forEach((p, k) => {
        const a = ctr[p.from], b = ctr[p.to]; if (!a || !b) return;
        const g = s('g', { class: 'packet', visibility: 'hidden' }, [s('circle', { r: 7, fill: COLOR[p.kind] || '#fff' }), p.label ? s('text', { x: 10, y: -9 }, p.label) : null]);
        gP.append(g);
        st.pk.push({ a, b, g, at: p.at != null ? p.at : (k / ps.length) * 0.9, dur: p.dur != null ? p.dur : Math.min(0.6, 0.9 / ps.length + 0.15) });
      });
      renderDetail(stp);
      [...timeline.children].forEach((li, k) => { li.classList.toggle('cur', k === st.i); li.classList.toggle('done', k < st.i); });
      const cur = timeline.children[st.i]; if (cur && cfg.scrollTimeline !== false) cur.scrollIntoView({ block: 'nearest' });
      btnPrev.disabled = st.i === 0; btnNext.disabled = st.i === st.steps.length - 1;
      pos.textContent = `${st.i + 1} / ${st.steps.length}  ·  t = ${fmtT(stp.t)}`;
      cfg.onStep && cfg.onStep(stp, st.i);
      kick();
    }
    const fmtT = (t) => (t == null ? '' : t < 1 ? `+${Math.round(t * 1000)} ms` : t < 120 ? `+${t.toFixed(1)} s` : `+${(t / 60).toFixed(1)} dk`);

    function renderDetail(stp) {
      const code = stp.code && RL.codeLib && RL.codeLib[stp.code];
      const codeHost = h('div');
      const snap = stp.snap;
      clear(detail).append(
        h('div', { class: 'phase-tag' }, [`Adım ${st.i + 1}`, '  ', RL.stateChip(stp.state)]),
        h('h3', { style: 'margin-top:6px' }, stp.title),
        stp.what ? h('p', null, stp.what) : null,
        h('dl', { class: 'kv' }, [
          h('dt', null, 'Çalışan'), h('dd', null, ACTOR[stp.actor] || stp.actor || '—'),
          h('dt', null, 'Tür'), h('dd', null, KIND[stp.kind] || stp.kind),
          stp.tx ? [h('dt', null, 'Yerel transaction'), h('dd', null, (ACTOR[stp.tx] || stp.tx) + ' DB · BEGIN … COMMIT')] : null,
          stp.data ? [h('dt', null, 'Değişen veri'), h('dd', null, stp.data.map((d) => h('div', null, d)))] : null,
          stp.next ? [h('dt', null, 'Sonraki adımı ne tetikler?'), h('dd', null, stp.next)] : null,
          snap && snap.order ? [h('dt', null, 'Rovena durumu'), h('dd', null, `orders.status=${snap.order.status || '—'} · reservation_state=${snap.order.res} · refund_state=${snap.order.refund}`)] : null,
        ]),
        stp.ev ? [h('h4', { style: 'margin-top:8px' }, 'Mesaj / olay içeriği'), RL.code(JSON.stringify(stp.ev, null, 2), 'c-amqp')] : null,
        code ? h('button', { class: 'btn small', style: 'margin-top:8px', onclick: () => { clear(codeHost).append(RL.codeSnippet(stp.code)); } }, `İlgili kodu göster: ${code.title}`) : null,
        codeHost,
      );
    }
    const ACTOR = { client: 'İstemci', order: 'Order servisi', orderdb: 'Order DB', recovery: 'Order · kurtarma işi', inventory: 'Inventory servisi', invdb: 'Inventory DB', payment: 'Payment servisi', paydb: 'Payment DB', provider: 'Ödeme sağlayıcısı', orch: 'Orchestrator', ex: 'RabbitMQ' };
    const KIND = { http: 'Senkron HTTP çağrısı', tx: 'Yerel transaction', publish: 'Olay yayını (outbox relay)', deliver: 'Mesaj teslimi (consumer)', ack: 'Ack', fail: 'Hata', comp: 'Compensation (telafi)', timer: 'Zamanlayıcı / timeout', note: 'Bilgi', warn: 'Uyarı', manual: 'Manuel müdahale', cmd: 'Komut', evt: 'Olay' };

    function frame(ts) {
      st.raf = 0;
      const dt = st.last ? Math.min(80, ts - st.last) : 0; st.last = ts;
      const animating = st.clock <= STEP_MS + 200;
      if (!st.paused && (st.playing || animating)) st.clock += dt * st.speed;
      st.pk.forEach((p) => {
        const f = (st.clock - p.at * STEP_MS) / (p.dur * STEP_MS);
        if (f < 0) { p.g.setAttribute('visibility', 'hidden'); return; }
        const e = ease(Math.min(1, f));
        p.g.setAttribute('transform', `translate(${(p.a[0] + (p.b[0] - p.a[0]) * e).toFixed(1)},${(p.a[1] + (p.b[1] - p.a[1]) * e).toFixed(1)})`);
        p.g.setAttribute('visibility', f > 1.3 ? 'hidden' : 'visible');
        p.g.setAttribute('opacity', f > 1 ? Math.max(0, 1 - (f - 1) * 3.3) : 1);
      });
      if (st.playing && !st.paused && st.clock > STEP_MS + HOLD_MS) {
        if (st.i < st.steps.length - 1) { goto(st.i + 1); return; }
        setPlay(false);
      }
      if ((st.playing || animating) && !st.paused) st.raf = requestAnimationFrame(frame); else st.last = 0;
    }
    function kick() { st.last = 0; if (!st.raf) st.raf = requestAnimationFrame(frame); }
    function setPlay(p) {
      st.playing = p; st.paused = false;
      btnPlay.textContent = p ? '⏸ Duraklat' : '▶ Başlat';
      if (p) kick();
    }
    btnPlay.addEventListener('click', () => {
      if (st.playing && !st.paused) { st.paused = true; btnPlay.textContent = '▶ Devam et'; return; }
      if (st.playing && st.paused) { st.paused = false; btnPlay.textContent = '⏸ Duraklat'; kick(); return; }
      if (st.i >= st.steps.length - 1) goto(0);
      setPlay(true);
    });
    btnNext.addEventListener('click', () => { setPlay(false); goto(st.i + 1); });
    btnPrev.addEventListener('click', () => { setPlay(false); goto(st.i - 1); });
    btnRestart.addEventListener('click', () => { setPlay(false); goto(0); });
    speed.addEventListener('change', () => (st.speed = +speed.value));

    function load(steps) {
      st.steps = steps; setPlay(false); clear(timeline);
      steps.forEach((x, k) => timeline.append(h('li', { class: 'k-' + x.kind, tabindex: 0, onclick: () => { setPlay(false); goto(k); }, onkeydown: (e) => { if (e.key === 'Enter') { setPlay(false); goto(k); } } }, [
        h('span', { class: 'tl-t mono' }, fmtT(x.t)), h('span', { class: 'tl-dot ' + (RL.STATE_CLS[x.state] || '') }), h('span', null, x.title),
      ])));
      goto(0, false);
    }
    if (cfg.steps) load(cfg.steps);
    return { load, goto, play: () => setPlay(true) };
  };
})();
