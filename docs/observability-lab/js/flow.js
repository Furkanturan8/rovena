/* Mod 1: adım adım, kullanıcı kontrollü veri akışı animasyonu. */
(function () {
  const { h, s, clear } = RL;
  const KIND_COLOR = { http: '#5ab0ff', amqp: '#ffb454', metric: '#7ee787', trace: '#c792ea', db: '#6fd3c4' };
  const KIND_LABEL = { http: 'HTTP isteği', amqp: 'RabbitMQ mesajı', metric: 'Metrik (Prometheus)', trace: 'Trace (OTLP)', db: 'Veritabanı yazımı' };
  const STEP_MS = 2800, HOLD_MS = 1600;
  const ease = (t) => (t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2);

  RL.initFlow = function (root) {
    const A = RL.arch;
    const ctr = {}; A.NODES.forEach((n) => (ctr[n.id] = [n.x + n.w / 2, n.y + n.h / 2]));
    const st = { variant: A.VARIANTS[0], i: 0, playing: false, frozen: false, speed: 1, clock: 0, last: 0, raf: 0, packets: [], sel: null };

    /* ---------- iskelet ---------- */
    const svg = s('svg', { class: 'diagram', viewBox: '0 0 1000 655', role: 'img', 'aria-label': 'Rovena sistem diyagramı' });
    svg.append(s('rect', { class: 'frame', x: 260, y: 245, width: 495, height: 225, rx: 10 }), s('text', { class: 'frame-label', x: 272, y: 262 }, 'RabbitMQ  :5672 · yönetim :15672 · metrik :15692'));
    const gEdges = s('g'), gNodes = s('g'), gPk = s('g');
    svg.append(gEdges, gNodes, gPk);
    const edgeEl = {};
    A.EDGES.forEach((e) => {
      const pts = [ctr[e.from], ...e.via, ctr[e.to]];
      e.pts = pts;
      const p = s('path', { class: 'edge k-' + e.kind, d: 'M' + pts.map((q) => q.join(',')).join(' L') });
      edgeEl[e.id] = p; gEdges.append(p);
    });
    const nodeEl = {}, cntEl = {};
    A.NODES.forEach((n) => {
      if (n.hidden) return;
      const g = s('g', { class: 'node ' + (n.cls || ''), tabindex: 0, role: 'button', 'aria-label': n.label });
      g.append(s('rect', { x: n.x, y: n.y, width: n.w, height: n.h, rx: n.q ? 6 : 9 }));
      if (n.q) {
        g.append(s('text', { x: n.x + 9, y: n.y + 20, style: 'text-anchor:start;font-size:11.5px' }, n.label));
        cntEl[n.q] = s('text', { x: n.x + n.w - 8, y: n.y + 20, style: 'text-anchor:end;font-size:10.5px;font-family:var(--mono)' }, 'R0 U0');
        g.append(cntEl[n.q]);
      } else {
        g.append(s('text', { x: n.x + n.w / 2, y: n.y + (n.sub ? n.h / 2 - 2 : n.h / 2 + 4) }, n.label));
        if (n.sub) g.append(s('text', { class: 'sub', x: n.x + n.w / 2, y: n.y + n.h / 2 + 12 }, n.sub));
      }
      g.addEventListener('click', () => select(n.id));
      g.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); select(n.id); } });
      nodeEl[n.id] = g; gNodes.append(g);
    });

    /* ---------- kontroller ---------- */
    const btnPlay = h('button', { class: 'play', 'aria-label': 'Başlat / duraklat' });
    const btnPrev = h('button', { 'aria-label': 'Önceki adım', title: 'Önceki adım (←)' }, '◀ Geri');
    const btnNext = h('button', { 'aria-label': 'Sonraki adım', title: 'Sonraki adım (→)' }, 'İleri ▶');
    const btnRestart = h('button', { 'aria-label': 'Yeniden başlat' }, '⟲ Baştan');
    const speed = h('select', { 'aria-label': 'Animasyon hızı' }, [0.5, 1, 2, 4].map((v) => h('option', { value: v, selected: v === 1 }, v + '×')));
    const dots = h('div', { class: 'dots' });
    const variantBar = h('div', { class: 'variants' }, A.VARIANTS.map((v) => h('button', { class: v === st.variant ? 'on' : '', onclick: () => setVariant(v) }, v.label)));
    variantBar.append(h('div', { class: 'legend' }, Object.keys(KIND_LABEL).map((k) => h('span', null, [h('i', { style: 'background:' + KIND_COLOR[k] }), KIND_LABEL[k]]))));

    const stage = h('div', { class: 'card stage' }, [variantBar, svg,
      h('div', { class: 'controls' }, [btnRestart, btnPrev, btnPlay, btnNext, h('label', { class: 'muted' }, ['Hız ', speed]), dots])]);

    /* ---------- yan panel ---------- */
    const stepCard = h('div', { class: 'card stepcard' });
    const dataCard = h('div', { class: 'card' });
    const stateCard = h('div', { class: 'card' });
    const inspCard = h('div', { class: 'card inspector' }, [h('h3', null, 'Bileşen rehberi'), h('p', { class: 'muted' }, 'Diyagramdaki herhangi bir kutuya tıkla: ne yaptığını, hangi veriyi ürettiğini ve Rovena’da nerede olduğunu gör.')]);
    root.append(h('div', { class: 'flow' }, [stage, h('div', null, [stepCard, dataCard, stateCard, inspCard])]));

    /* ---------- durum (kümülatif) ---------- */
    function stateAt(i) {
      const out = { queues: { pq: [0, 0], retry: [0, 0], dlq: [0, 0], rq: [0, 0] }, outbox: 0, orderStatus: '–', metrics: [], spans: [], alerts: [] };
      st.variant.steps.slice(0, i + 1).forEach((step) => {
        const sx = step.state || {};
        Object.keys(sx).forEach((k) => { if (k === 'queues') Object.assign(out.queues, sx.queues); else out[k] = sx[k]; });
      });
      return out;
    }

    /* ---------- bir adıma gir ---------- */
    function goto(i, animate = true) {
      const steps = st.variant.steps;
      st.i = Math.max(0, Math.min(steps.length - 1, i));
      const step = steps[st.i];
      st.clock = animate ? 0 : STEP_MS + 1; st.frozen = false;
      // düğümler
      Object.values(nodeEl).forEach((g) => (g.setAttribute('class', g.getAttribute('class').replace(/\b(active|dim|err|down|busy)\b/g, '').replace(/\s+/g, ' ').trim())));
      Object.entries(nodeEl).forEach(([id, g]) => {
        const c = [];
        if ((step.focus || []).includes(id)) c.push('active');
        else if (step.dim) c.push('dim');
        if (step.nodes && step.nodes[id]) c.push(step.nodes[id]);
        if (st.sel === id) c.push('sel');
        g.setAttribute('class', g.getAttribute('class') + ' ' + c.join(' '));
      });
      // kenarlar
      const used = new Set((step.packets || []).map((p) => p.edge));
      A.EDGES.forEach((e) => {
        const c = ['edge', 'k-' + e.kind];
        if (used.has(e.id)) c.push('on'); else if (e.kind === 'metric' || e.kind === 'trace') c.push('off'); else if (step.dim) c.push('dim');
        edgeEl[e.id].setAttribute('class', c.join(' '));
      });
      // paketler
      clear(gPk); st.packets = [];
      (step.packets || []).forEach((p) => {
        const e = A.EDGES.find((x) => x.id === p.edge);
        const pts = p.rev ? e.pts.slice().reverse() : e.pts;
        const seg = []; let tot = 0;
        for (let k = 1; k < pts.length; k++) { const d = Math.hypot(pts[k][0] - pts[k - 1][0], pts[k][1] - pts[k - 1][1]); seg.push(d); tot += d; }
        const g = s('g', { class: 'packet', visibility: 'hidden' }, [s('circle', { r: 6.5, fill: KIND_COLOR[p.kind] }), p.label ? s('text', { x: 10, y: -8 }, p.label) : null]);
        gPk.append(g);
        st.packets.push({ p, pts, seg, tot, g });
      });
      renderPanels(step);
      btnPrev.disabled = st.i === 0; btnNext.disabled = st.i === steps.length - 1;
      [...dots.children].forEach((d, k) => (d.className = k < st.i ? 'done' : k === st.i ? 'cur' : ''));
      kick();
    }

    function renderPanels(step) {
      const steps = st.variant.steps;
      clear(stepCard).append(
        h('div', { class: 'phase-tag' }, `Adım ${st.i + 1} / ${steps.length}  ·  ${st.variant.label}`),
        h('h3', { style: 'margin-top:4px' }, step.title),
        h('p', { html: step.desc }),
        h('div', { class: 'qa' }, [
          h('div', { class: 'q1' }, [h('b', null, 'Şu anda neye bakıyorum?'), h('span', { html: step.qa.look })]),
          h('div', { class: 'q2' }, [h('b', null, 'Bu veri nereden geliyor?'), h('span', { html: step.qa.source })]),
          h('div', { class: 'q3' }, [h('b', null, 'Değer ne anlama gelir?'), h('span', { html: step.qa.meaning })]),
          h('div', { class: 'q4' }, [h('b', null, 'Sonraki adım'), h('span', { html: step.qa.next })]),
        ]),
        h('p', { class: 'muted', style: 'margin-top:10px;font-size:12px' }, [h('span', { class: 'tag real' }, 'GERÇEK'), ' Gerçek Rovena’da bak: ', h('b', null, step.where)]));

      clear(dataCard);
      if (step.compare) {
        dataCard.append(h('h4', null, 'Aynı olayın iki farklı kaydı'), h('div', { class: 'compare' }, [
          h('div', null, [h('b', { class: 'c-metric' }, 'Prometheus'), h('p', { class: 'muted' }, 'Sayı, zaman damgasıyla. Binlerce isteğin özeti.'),
            RL.code('http_server_request_duration_seconds_count\n  {job="order",\n   http_route="/api/v1/checkout"}\n\n12:00:10  1840\n12:00:20  1842\n12:00:30  1847\n→ rate() ile “saniyede kaç”', 'c-metric')]),
          h('div', null, [h('b', { class: 'c-trace' }, 'Jaeger'), h('p', { class: 'muted' }, 'Tek isteğin ayrıntılı hikâyesi. Etiket + süre.'),
            RL.code('trace 5b8a…  (212 ms)\n order POST /checkout   212\n  order POST catalog     38\n  order POST inventory   74\n  order INSERT outbox     9\n → süre NEREYE gitti?', 'c-trace')]),
        ]), h('p', { class: 'muted', style: 'margin-top:8px' }, 'Metrikten trace’e geçiş: Grafana’da yavaş route’u bul, sonra Jaeger’da o route’un yavaş bir isteğini aç.'));
      } else if (step.data) {
        dataCard.append(h('h4', null, step.data.title), RL.code(step.data.body, step.data.lang));
      }

      const sx = stateAt(st.i);
      const q = sx.queues;
      Object.entries(cntEl).forEach(([k, el]) => (el.textContent = `R${q[k][0]} U${q[k][1]}`));
      const qs = (k, name, dlq) => h('div', { class: 'stat' + (dlq && q[k][0] > 0 ? ' bad' : q[k][0] > 0 ? ' hot' : '') }, [h('small', null, name), h('b', null, `R ${q[k][0]} · U ${q[k][1]}`)]);
      clear(stateCard).append(
        h('h4', null, 'O andaki sistem durumu'),
        h('div', { class: 'statgrid' }, [qs('pq', 'payment.requests'), qs('retry', '….retry'), qs('dlq', '….dlq', true), qs('rq', 'order.payment-results'),
          h('div', { class: 'stat' + (sx.outbox > 0 ? ' hot' : '') }, [h('small', null, 'Order outbox (bekleyen)'), h('b', null, sx.outbox)]),
          h('div', { class: 'stat' + (sx.orderStatus === 'confirmed' ? ' good' : /cancel/.test(sx.orderStatus) ? ' bad' : '') }, [h('small', null, 'Sipariş durumu'), h('b', { style: 'font-size:12px' }, sx.orderStatus)])]),
        h('div', { class: 'muted', style: 'font-size:11px' }, 'R = Ready (bekliyor), U = Unacked (işleniyor, henüz ack edilmedi)'),
        sx.spans.length ? [h('h4', { style: 'margin-top:10px' }, 'Trace (Jaeger’da göreceğin ağaç)'),
          h('div', { class: 'spans' }, sx.spans.map((sp) => h('div', { class: 'sp', style: `padding-left:${sp[0] * 14}px` }, [sp[1], h('span', { class: 'ms' }, sp[2])])))] : null,
        sx.metrics.length ? [h('h4', { style: 'margin-top:10px' }, 'Bu adımda değişen / ilgili metrikler'), RL.code(sx.metrics.join('\n'), 'c-metric')] : null,
        sx.alerts.length ? [h('h4', { style: 'margin-top:10px' }, 'Uyarılar'), sx.alerts.map((a) => h('div', null, h('span', { class: 'chip bad' }, a)))] : null,
        h('div', { class: 'note-sim' }, 'Sayılar örnektir (SİMÜLE); akış ve ad kuralları kod tabanından.'));
    }

    function select(id) {
      st.sel = id;
      Object.entries(nodeEl).forEach(([k, g]) => g.classList.toggle('sel', k === id));
      const info = A.INFO[id]; if (!info) return;
      clear(inspCard).append(h('h3', null, info.t), h('p', null, [h('span', { class: 'tag real' }, 'GERÇEK'), ' ', info.real]),
        h('p', { html: info.role }), h('ul', null, [h('li', { html: '<b>Metrik:</b> ' + info.metrics }), h('li', { html: '<b>Trace:</b> ' + info.traces })]));
    }

    /* ---------- animasyon döngüsü ---------- */
    function pos(pk, f) {
      let d = f * pk.tot;
      for (let k = 0; k < pk.seg.length; k++) {
        if (d <= pk.seg[k] || k === pk.seg.length - 1) { const r = pk.seg[k] ? Math.min(1, d / pk.seg[k]) : 1; return [pk.pts[k][0] + (pk.pts[k + 1][0] - pk.pts[k][0]) * r, pk.pts[k][1] + (pk.pts[k + 1][1] - pk.pts[k][1]) * r]; }
        d -= pk.seg[k];
      }
      return pk.pts[0];
    }
    function frame(ts) {
      st.raf = 0;
      const dt = st.last ? Math.min(80, ts - st.last) : 0; st.last = ts;
      const animating = st.clock <= STEP_MS + 400;
      if (!st.frozen && (st.playing || animating)) st.clock += dt * st.speed;
      st.packets.forEach((pk) => {
        const f = (st.clock - pk.p.at * STEP_MS) / (pk.p.dur * STEP_MS);
        if (f < 0) { pk.g.setAttribute('visibility', 'hidden'); return; }
        const fc = ease(Math.min(1, f)); const [x, y] = pos(pk, fc);
        pk.g.setAttribute('transform', `translate(${x.toFixed(1)},${y.toFixed(1)})`);
        pk.g.setAttribute('visibility', f > 1.25 ? 'hidden' : 'visible');
        pk.g.setAttribute('opacity', f > 1 ? Math.max(0, 1 - (f - 1) * 4) : 1);
      });
      if (st.playing && !st.frozen && st.clock > STEP_MS + HOLD_MS) {
        if (st.i < st.variant.steps.length - 1) { goto(st.i + 1); return; }
        setPlaying(false);
      }
      if (st.playing || (animating && !st.frozen)) st.raf = requestAnimationFrame(frame); else st.last = 0;
    }
    function kick() { st.last = 0; if (!st.raf) st.raf = requestAnimationFrame(frame); }

    function setPlaying(p) {
      st.playing = p;
      btnPlay.textContent = p ? '⏸ Duraklat' : (st.i >= st.variant.steps.length - 1 && st.clock > STEP_MS ? '▶ Yeniden oynat' : '▶ Başlat');
      btnPlay.classList.toggle('on', p);
      if (p) { st.frozen = false; kick(); }
    }
    btnPlay.addEventListener('click', () => {
      if (st.playing) { setPlaying(false); st.frozen = true; return; }
      if (st.frozen) { st.frozen = false; setPlaying(true); return; }
      if (st.i >= st.variant.steps.length - 1) goto(0);
      setPlaying(true);
    });
    btnNext.addEventListener('click', () => { setPlaying(false); goto(st.i + 1); });
    btnPrev.addEventListener('click', () => { setPlaying(false); goto(st.i - 1); });
    btnRestart.addEventListener('click', () => { setPlaying(false); goto(0); });
    speed.addEventListener('change', () => (st.speed = +speed.value));

    function setVariant(v) {
      st.variant = v; setPlaying(false);
      [...variantBar.querySelectorAll('button')].forEach((b, k) => b.classList.toggle('on', A.VARIANTS[k] === v));
      clear(dots); v.steps.forEach((_, k) => dots.append(h('button', { 'aria-label': 'Adım ' + (k + 1), onclick: () => { setPlaying(false); goto(k); } }, k + 1)));
      goto(0);
    }
    root.addEventListener('keydown', (e) => {
      if (e.target.closest('select,input,textarea')) return;
      if (e.key === 'ArrowRight') btnNext.click(); else if (e.key === 'ArrowLeft') btnPrev.click(); else if (e.key === ' ' && !e.target.closest('.node,button')) { e.preventDefault(); btnPlay.click(); }
    });
    setVariant(st.variant);
    setPlaying(false);
  };
})();
