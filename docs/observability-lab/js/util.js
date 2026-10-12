/* Küçük DOM yardımcıları, deterministik veri üreteci ve çizgi grafik. Bağımlılık yok. */
(function () {
  const RL = (window.RL = {});
  const SVGNS = 'http://www.w3.org/2000/svg';

  function append(e, kids) {
    if (kids == null || kids === false) return;
    if (Array.isArray(kids)) { kids.forEach((k) => append(e, k)); return; }
    if (typeof kids === 'string' && /<\/?(code|b|i|br|span)\b/.test(kids)) {
      // Yazarın kendi metinleri (sabit): basit biçim etiketlerini HTML olarak işle.
      const t = document.createElement('template'); t.innerHTML = kids; e.append(t.content); return;
    }
    e.append(kids.nodeType ? kids : document.createTextNode(String(kids)));
  }
  function make(ns, tag, attrs, kids) {
    const e = ns ? document.createElementNS(SVGNS, tag) : document.createElement(tag);
    if (attrs) {
      for (const k in attrs) {
        const v = attrs[k];
        if (v == null || v === false) continue;
        if (k === 'html') e.innerHTML = v;
        else if (k === 'text') e.textContent = v;
        else if (k.startsWith('on')) e.addEventListener(k.slice(2), v);
        else e.setAttribute(k, v === true ? '' : v);
      }
    }
    append(e, kids);
    return e;
  }
  RL.h = (tag, attrs, kids) => make(false, tag, attrs, kids);
  RL.s = (tag, attrs, kids) => make(true, tag, attrs, kids);
  RL.clear = (e) => { while (e.firstChild) e.removeChild(e.firstChild); return e; };

  /* ---- deterministik rastgelelik: her açılışta aynı grafik (tekrarlanabilir eğitim) ---- */
  RL.rng = function (seed) {
    let a = seed >>> 0;
    return () => {
      a = (a + 0x6d2b79f5) | 0;
      let t = Math.imul(a ^ (a >>> 15), 1 | a);
      t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  };
  const smooth = (x) => { x = Math.max(0, Math.min(1, x)); return x * x * (3 - 2 * x); };
  /** lo -> hi geçişi: t `at` anında başlar, `width` sürer (t 0..1). */
  RL.shift = (t, at, lo, hi, width = 0.04) => lo + (hi - lo) * smooth((t - at) / width);
  /** n noktalı seri üretir. fn(t, i) -> değer, noise: ± gürültü. */
  RL.gen = (n, fn, noise = 0, seed = 1) => {
    const r = RL.rng(seed);
    return Array.from({ length: n }, (_, i) => {
      const t = i / (n - 1);
      return Math.max(0, fn(t, i) + (r() - 0.5) * 2 * noise);
    });
  };
  RL.N = 360; // 6 saat, dakikada bir nokta
  RL.COLORS = ['#5ab0ff', '#ffb454', '#7ee787', '#c792ea', '#ff7b72', '#79c0ff', '#f2cc60'];

  RL.fmt = (v) => {
    if (v == null || isNaN(v)) return '–';
    const a = Math.abs(v);
    if (a >= 1000) return (v / 1000).toFixed(1) + 'k';
    if (a >= 100) return v.toFixed(0);
    if (a >= 10) return v.toFixed(1);
    if (a >= 1) return v.toFixed(2);
    if (a === 0) return '0';
    return v.toFixed(3);
  };

  /**
   * Çizgi grafik (SVG). series: [{name, data, color}], range: son kaç nokta (varsayılan hepsi),
   * thresholds: [{y,label}], markers: [{at: 0..1, label}], unit: 's' | '%' | ''
   */
  RL.lineChart = function (o) {
    const W = 400, H = o.height || 130, L = 34, R = 6, T = 8, B = 16;
    const range = Math.min(o.range || RL.N, o.series[0].data.length);
    const sl = o.series.map((s) => ({ ...s, d: s.data.slice(-range) }));
    let max = o.yMax;
    if (max == null) {
      max = Math.max(...sl.flatMap((s) => s.d), ...(o.thresholds || []).map((t) => t.y), 0.0001) * 1.15;
    }
    const min = o.yMin || 0;
    const x = (i) => L + (i / (range - 1)) * (W - L - R);
    const y = (v) => T + (1 - (v - min) / (max - min)) * (H - T - B);
    const unit = o.unit || '';
    const f = (v) => RL.fmt(v) + unit;
    const svg = RL.s('svg', { viewBox: `0 0 ${W} ${H}`, role: 'img', 'aria-label': o.title || 'grafik' });
    for (let g = 0; g <= 3; g++) {
      const v = min + ((max - min) * g) / 3;
      svg.append(RL.s('line', { x1: L, x2: W - R, y1: y(v), y2: y(v), stroke: '#1c2638', 'stroke-width': 1 }));
      svg.append(RL.s('text', { x: L - 4, y: y(v) + 3, 'text-anchor': 'end', class: 'ax' }, f(v)));
    }
    const labels = o.xLabels || [`-${range >= 60 ? Math.round(range / 60) + 'sa' : range + 'dk'}`, 'şimdi'];
    svg.append(RL.s('text', { x: L, y: H - 3, class: 'ax' }, labels[0]));
    svg.append(RL.s('text', { x: W - R, y: H - 3, 'text-anchor': 'end', class: 'ax' }, labels[1]));
    (o.thresholds || []).forEach((t) => {
      svg.append(RL.s('line', { x1: L, x2: W - R, y1: y(t.y), y2: y(t.y), stroke: '#ff6b6b', 'stroke-dasharray': '4 3', 'stroke-width': 1 }));
      svg.append(RL.s('text', { x: W - R, y: y(t.y) - 3, 'text-anchor': 'end', class: 'ax', fill: '#ff6b6b' }, t.label));
    });
    sl.forEach((s, si) => {
      const col = s.color || RL.COLORS[si % RL.COLORS.length];
      const pts = s.d.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(' ');
      if (o.area && sl.length === 1) {
        svg.append(RL.s('polygon', { points: `${x(0)},${y(min)} ${pts} ${x(range - 1)},${y(min)}`, fill: col, opacity: 0.12 }));
      }
      svg.append(RL.s('polyline', { points: pts, fill: 'none', stroke: col, 'stroke-width': 1.5, 'stroke-linejoin': 'round' }));
    });
    (o.markers || []).forEach((m) => {
      const idx = Math.round(m.at * (o.series[0].data.length - 1)) - (o.series[0].data.length - range);
      if (idx < 0 || idx >= range) return;
      svg.append(RL.s('line', { x1: x(idx), x2: x(idx), y1: T, y2: H - B, stroke: '#c792ea', 'stroke-dasharray': '2 3' }));
      svg.append(RL.s('text', { x: x(idx) + 3, y: T + 8, class: 'ax', fill: '#c792ea' }, m.label));
    });
    const wrap = RL.h('div', { class: 'chartwrap' }, svg);
    const guide = RL.s('line', { y1: T, y2: H - B, stroke: '#ffffff55', 'stroke-width': 1, visibility: 'hidden' });
    svg.append(guide);
    const tip = RL.h('div', { class: 'tip', hidden: true });
    wrap.append(tip);
    const hit = RL.s('rect', { x: L, y: T, width: W - L - R, height: H - T - B, fill: 'transparent' });
    svg.append(hit);
    hit.addEventListener('mousemove', (ev) => {
      const r = svg.getBoundingClientRect();
      const px = ((ev.clientX - r.left) / r.width) * W;
      const i = Math.max(0, Math.min(range - 1, Math.round(((px - L) / (W - L - R)) * (range - 1))));
      guide.setAttribute('x1', x(i)); guide.setAttribute('x2', x(i)); guide.setAttribute('visibility', 'visible');
      const ago = range - 1 - i;
      tip.hidden = false;
      tip.style.left = (x(i) / W) * r.width + 'px';
      tip.style.top = '14px';
      tip.innerHTML = `<div style="color:#8b95a7">${ago === 0 ? 'şimdi' : '-' + ago + ' dk'}</div>` +
        sl.map((s, si) => `<div><i style="display:inline-block;width:8px;height:8px;border-radius:2px;background:${s.color || RL.COLORS[si % RL.COLORS.length]};margin-right:5px"></i>${s.name ? s.name + ': ' : ''}${f(s.d[i])}</div>`).join('');
    });
    hit.addEventListener('mouseleave', () => { tip.hidden = true; guide.setAttribute('visibility', 'hidden'); });
    if (sl.length > 1 || sl[0].name) {
      wrap.append(RL.h('div', { class: 'legend-s' }, sl.map((s, si) =>
        RL.h('span', null, [RL.h('i', { style: `background:${s.color || RL.COLORS[si % RL.COLORS.length]}` }), s.name]))));
    }
    return wrap;
  };

  /** Basit sözdizimi renklendirmesi olmadan, kopyalanabilir kod bloğu. */
  RL.code = (text, extra) => RL.h('pre', { class: 'code' + (extra ? ' ' + extra : ''), text });
})();
