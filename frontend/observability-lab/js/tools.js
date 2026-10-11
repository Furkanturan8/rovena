/*
 * Gerçek arayüzlerden esinlenen SİMÜLE araç pencereleri: Grafana, Prometheus, Jaeger, RabbitMQ Management, log.
 * Her pencere bir `spec` (veri) alır ve `emit(olay)` ile kullanıcı eylemlerini bildirir. Veri kaynağı değiştirilirse
 * (örn. gerçek Prometheus HTTP API'si) yalnızca spec üreten kod değişir; çizim kodu aynı kalır.
 */
(function () {
  const { h, s, clear } = RL;
  const T = (RL.tools = {});

  const META = {
    grafana: { name: 'Grafana', port: 'http://localhost:3000' },
    prom: { name: 'Prometheus', port: 'http://localhost:9090' },
    jaeger: { name: 'Jaeger', port: 'http://localhost:16686' },
    rabbit: { name: 'RabbitMQ Management', port: 'http://localhost:15672' },
    logs: { name: 'Terminal', port: 'backend/ içinde' },
  };

  T.window = function (spec, emit) {
    emit = emit || (() => {});
    const m = META[spec.tool];
    const body = h('div', { class: 'tw-body' });
    const build = { grafana, prom, jaeger, rabbit, logs }[spec.tool];
    build(body, spec, emit);
    return h('div', { class: 'tw t-' + spec.tool }, [
      h('div', { class: 'tw-bar' }, [
        h('b', null, m.name),
        h('span', { class: 'url' }, spec.url || m.port),
        h('span', { class: 'tag sim' }, 'SİMÜLE VERİ'),
      ]),
      body,
      spec.note ? h('div', { class: 'note-sim', style: 'padding:0 12px 8px' }, spec.note) : null,
    ]);
  };

  /* ================= Grafana ================= */
  function grafana(root, spec, emit) {
    let range = spec.range || 60;
    const grid = h('div');
    const sel = h('select', { 'aria-label': 'Time range', onchange: (e) => { range = +e.target.value; draw(); emit('range:' + range); } }, [
      [15, 'Last 15 minutes'], [60, 'Last 1 hour'], [360, 'Last 6 hours'],
    ].map(([v, l]) => h('option', { value: v, selected: v === range }, l)));
    root.append(h('div', { class: 'gf-head' }, [
      h('b', null, spec.dashboard || 'Rovena overview'),
      h('span', { class: 'pill' }, 'Datasource: Prometheus'),
      h('span', { class: 'pill' }, 'Time range'), sel,
      h('span', { class: 'pill' }, 'refresh 10s'),
    ]), grid);
    const shown = new Set();
    function draw() {
      clear(grid);
      spec.rows.forEach((row) => {
        grid.append(h('div', { class: 'gf-row' }, row.title));
        grid.append(h('div', { class: 'gf-grid' }, row.panels.map((p) => {
          const exprBox = h('div', { class: 'expr', hidden: !shown.has(p.id) }, p.expr);
          const el = h('div', { class: 'panel', tabindex: 0, role: 'button', 'aria-label': p.title,
            onclick: () => emit('panel:' + p.id),
            onkeydown: (e) => { if (e.key === 'Enter') emit('panel:' + p.id); } }, [
            h('h5', null, [p.title, h('span', { title: 'Panelin PromQL sorgusunu göster (Edit → Query)', onclick: (e) => {
              e.stopPropagation();
              shown.has(p.id) ? shown.delete(p.id) : shown.add(p.id);
              exprBox.hidden = !shown.has(p.id);
              emit('query-peek:' + p.id);
            } }, '‹/› sorgu')]),
            exprBox,
            RL.lineChart({ series: p.series, unit: p.unit, range, thresholds: p.thr, markers: p.markers, area: true, yMax: p.yMax, height: 120, title: p.title }),
          ]);
          return el;
        })));
      });
    }
    draw();
  }

  /* ================= Prometheus ================= */
  function prom(root, spec, emit) {
    const tabs = ['graph', 'targets', 'alerts'].filter((t) => spec[t === 'graph' ? 'queries' : t]);
    let tab = spec.tab || 'graph';
    const bar = h('div', { class: 'tw-tabs' });
    const view = h('div');
    root.append(bar, view);
    const label = { graph: 'Graph', targets: 'Status ▸ Targets', alerts: 'Alerts' };
    function draw() {
      clear(bar);
      tabs.forEach((t) => bar.append(h('button', { class: t === tab ? 'on' : '', onclick: () => { tab = t; draw(); emit('tab:' + t); } }, label[t])));
      clear(view);
      ({ graph: drawGraph, targets: drawTargets, alerts: drawAlerts })[tab]();
    }
    function drawGraph() {
      const box = h('textarea', { class: 'pq-input', rows: 2, readonly: true, placeholder: 'Expression (PromQL) — aşağıdaki önerilerden birini seç' });
      const out = h('div');
      const list = h('div', { class: 'pq-q' });
      spec.queries.forEach((q) => {
        list.append(h('button', { onclick: (ev) => {
          box.value = q.expr; clear(out);
          ev.currentTarget.classList.add('ran');
          out.append(renderResult(q));
          emit('query:' + q.id);
        } }, q.expr));
      });
      view.append(h('div', { class: 'muted', style: 'font-size:12px;margin-bottom:4px' }, spec.prompt || 'Çalıştırmak istediğin sorguyu seç (gerçek arayüzde buraya elle yazarsın):'), box, list, out);
    }
    function renderResult(q) {
      const wrap = h('div');
      if (q.rows) {
        wrap.append(h('table', { class: 't' }, [
          h('tr', null, [h('th', null, 'Series'), h('th', { class: 'num' }, 'Value')]),
          q.rows.map((r) => h('tr', null, [
            h('td', null, Object.entries(r.labels).map(([k, v]) => h('span', { class: 'lbl' }, `${k}="${v}"`))),
            h('td', { class: 'num ' + (r.cls || '') }, r.value),
          ])),
        ]));
      }
      if (q.series) wrap.append(RL.lineChart({ series: q.series, unit: q.unit, height: 130, range: 60, thresholds: q.thr }));
      if (q.verdict) wrap.append(h('div', { class: 'verdict ' + q.verdict.type }, q.verdict.text));
      return wrap;
    }
    function drawTargets() {
      view.append(h('table', { class: 't' }, [
        h('tr', null, ['Endpoint', 'State', 'Labels', 'Last scrape', 'Scrape duration'].map((x) => h('th', null, x))),
        spec.targets.map((t) => h('tr', null, [
          h('td', { class: 'mono' }, t.endpoint),
          h('td', { class: t.up ? 'ok-t' : 'bad-t' }, t.up ? 'UP' : 'DOWN'),
          h('td', null, h('span', { class: 'lbl' }, `job="${t.job}"`)),
          h('td', null, t.last), h('td', null, t.dur),
        ])),
      ]));
    }
    function drawAlerts() {
      view.append(h('table', { class: 't' }, [
        h('tr', null, ['Alert', 'State', 'Açıklama'].map((x) => h('th', null, x))),
        spec.alerts.map((a) => h('tr', null, [
          h('td', { class: 'mono' }, a.name),
          h('td', { class: a.state === 'firing' ? 'bad-t' : a.state === 'pending' ? 'warn-t' : 'ok-t' }, a.state.toUpperCase()),
          h('td', null, a.summary),
        ])),
      ]), h('div', { class: 'muted', style: 'font-size:12px;margin-top:6px' }, 'Rovena\'da Alertmanager yok: uyarılar yalnızca bu sayfada ve Grafana\'da görünür, kimseye bildirim gitmez.'));
    }
    draw();
  }

  /* ================= Jaeger ================= */
  function parseDur(x) {
    const m = String(x || '').trim().match(/^([\d.]+)\s*(ms|s)?$/);
    if (!m) return 0;
    return m[2] === 's' ? +m[1] * 1000 : +m[1];
  }
  const fmtMs = (ms) => (ms >= 1000 ? (ms / 1000).toFixed(2) + ' s' : Math.round(ms) + ' ms');
  T.fmtMs = fmtMs;

  function jaeger(root, spec, emit) {
    const has = (t, svc) => (t.services || [t.service]).includes(svc);
    const services = [...new Set(spec.traces.flatMap((t) => t.services || [t.service]))].sort();
    const state = { service: spec.service || services[0], op: 'all', tags: spec.tags || '', min: spec.min || '' };
    const results = h('div'); const detail = h('div');
    const opSel = h('select', { 'aria-label': 'Operation', onchange: (e) => { state.op = e.target.value; } });
    const svcSel = h('select', { 'aria-label': 'Service', onchange: (e) => { state.service = e.target.value; fillOps(); } },
      services.map((v) => h('option', { value: v, selected: v === state.service }, v)));
    function fillOps() {
      clear(opSel);
      const ops = ['all', ...new Set(spec.traces.filter((t) => has(t, state.service)).map((t) => t.op))];
      ops.forEach((o) => opSel.append(h('option', { value: o }, o)));
      state.op = 'all';
    }
    fillOps();
    const tagIn = h('input', { value: state.tags, placeholder: 'error=true', 'aria-label': 'Tags', oninput: (e) => { state.tags = e.target.value; } });
    const minIn = h('input', { value: state.min, placeholder: 'örn. 1s veya 500ms', 'aria-label': 'Min Duration', oninput: (e) => { state.min = e.target.value; } });
    root.append(
      h('div', { class: 'jg-search' }, [
        h('div', null, [h('label', null, 'Service'), svcSel]),
        h('div', null, [h('label', null, 'Operation'), opSel]),
        h('div', null, [h('label', null, 'Tags'), tagIn]),
        h('div', null, [h('label', null, 'Min Duration'), minIn]),
        h('div', { style: 'align-self:end' }, h('button', { class: 'btn small', onclick: find }, 'Find Traces')),
      ]), results, detail);
    function find() {
      emit('search:' + state.service + (state.op !== 'all' ? '/' + state.op : '') + (state.tags ? '?' + state.tags : '') + (state.min ? '>' + state.min : ''));
      clear(results); clear(detail);
      const min = parseDur(state.min);
      const onlyErr = /error\s*=\s*true/.test(state.tags);
      let list = spec.traces.filter((t) => has(t, state.service) && (state.op === 'all' || t.op === state.op) && t.dur >= min && (!onlyErr || t.err));
      list = list.slice().sort((a, b) => a.ago - b.ago);
      const max = Math.max(...spec.traces.map((t) => t.dur), 1);
      results.append(h('div', { class: 'muted', style: 'margin:6px 0;font-size:12px' }, `${list.length} Traces`));
      if (!list.length) results.append(h('div', { class: 'verdict meh' }, 'Eşleşen trace yok. Filtreyi gevşet (operation, min duration, tag) ya da zaman aralığını kontrol et.'));
      list.forEach((t) => {
        const row = h('div', { class: 'tracerow' + (t.err ? ' err' : ''), tabindex: 0, role: 'button', onclick: () => open(t, row) }, [
          h('div', null, [h('b', null, `${t.service}: ${t.op}`), h('span', { class: 'muted' }, `  · ${t.spans.length} Spans · ${t.id.slice(0, 7)} · ${t.ago} dk önce`), t.err ? h('span', { class: 'chip bad', style: 'margin-left:8px' }, 'error') : null,
            h('div', { class: 'bar', style: `width:${Math.max(3, (t.dur / max) * 100)}%` })]),
          h('div', { class: 'num' }, fmtMs(t.dur)),
        ]);
        row.addEventListener('keydown', (e) => { if (e.key === 'Enter') open(t, row); });
        results.append(row);
      });
    }
    function open(t, row) {
      results.querySelectorAll('.tracerow').forEach((r) => r.classList.remove('sel')); row.classList.add('sel');
      emit('trace:' + t.id);
      clear(detail);
      const total = Math.max(...t.spans.map((x) => x.start + x.dur));
      const depth = {}; t.spans.forEach((x) => { depth[x.id] = x.parent ? (depth[x.parent] || 0) + 1 : 0; });
      const info = h('div');
      const wf = h('div', { class: 'wf' }, [
        h('div', { class: 'muted', style: 'font-size:12px;margin-bottom:6px' }, `Trace ${t.id} · toplam ${fmtMs(t.dur)} · ${t.spans.length} span · ${new Set(t.spans.map((x) => x.svc)).size} servis`),
        t.spans.map((x) => {
          const left = (x.start / total) * 100, w = Math.max(0.4, (x.dur / total) * 100);
          const r = h('div', { class: 'wf-row', tabindex: 0, role: 'button', onclick: () => pick(x, r) }, [
            h('div', { class: 'wf-name', style: `padding-left:${depth[x.id] * 14}px`, title: x.name }, [h('span', { class: 'svc' }, x.svc + ' '), x.name]),
            h('div', { class: 'wf-track' }, h('div', { class: 'wf-bar' + (x.err ? ' err' : '') + (x.async ? ' async' : '') + (left > 70 ? ' late' : ''), style: `left:${left}%;width:${w}%` }, h('span', null, fmtMs(x.dur)))),
          ]);
          r.addEventListener('keydown', (e) => { if (e.key === 'Enter') pick(x, r); });
          return r;
        }),
      ]);
      function pick(x, r) {
        wf.querySelectorAll('.wf-row').forEach((e) => e.classList.remove('sel')); r.classList.add('sel');
        emit('span:' + x.id);
        clear(info);
        const children = t.spans.filter((c) => c.parent === x.id);
        const self = Math.max(0, x.dur - children.filter((c) => !c.async).reduce((a, c) => a + c.dur, 0));
        info.append(h('div', { class: 'spandetail' }, [
          h('b', null, `${x.svc} · ${x.name}`), x.err ? h('span', { class: 'chip bad', style: 'margin-left:8px' }, 'error=true') : null,
          h('dl', { class: 'kv', style: 'margin-top:6px' }, [
            h('dt', null, 'Duration'), h('dd', null, fmtMs(x.dur)),
            h('dt', null, 'Self time (çocuklar hariç)'), h('dd', null, fmtMs(self)),
            h('dt', null, 'Start (trace başından)'), h('dd', null, fmtMs(x.start)),
            Object.entries(x.tags || {}).map(([k, v]) => [h('dt', null, k), h('dd', null, String(v))]),
          ]),
          (x.logs || []).map((l) => h('div', { class: 'bad-t mono', style: 'margin-top:6px;font-size:12px' }, `log · ${l}`)),
          x.async ? h('div', { class: 'muted', style: 'margin-top:6px' }, 'Bu span HTTP yanıtı döndükten SONRA çalıştı: istemcinin beklediği süreye dahil değil.') : null,
        ]));
      }
      detail.append(wf, info);
    }
    if (spec.autoSearch !== false) find();
  }

  /* ================= RabbitMQ Management ================= */
  function rabbit(root, spec, emit) {
    const tabsAll = ['overview', 'connections', 'exchanges', 'queues'];
    let tab = spec.tab || 'queues';
    let queue = null;
    const bar = h('div', { class: 'tw-tabs' }); const view = h('div');
    root.append(bar, view);
    const lab = { overview: 'Overview', connections: 'Connections / Channels', exchanges: 'Exchanges', queues: 'Queues' };
    function draw() {
      clear(bar);
      tabsAll.forEach((t) => bar.append(h('button', { class: t === tab ? 'on' : '', onclick: () => { tab = t; queue = null; draw(); emit('tab:' + t); } }, lab[t])));
      clear(view);
      ({ overview, connections, exchanges, queues })[tab]();
    }
    const rate = (v) => (v == null ? '' : v.toFixed(2) + '/s');
    function overview() {
      const o = spec.overview;
      view.append(
        h('h4', null, 'Totals'),
        h('div', { class: 'rm-stat' }, [
          h('div', null, [h('small', null, 'Ready'), h('b', null, o.ready)]),
          h('div', null, [h('small', null, 'Unacked'), h('b', null, o.unacked)]),
          h('div', null, [h('small', null, 'Total'), h('b', null, o.ready + o.unacked)]),
        ]),
        RL.lineChart({ series: [{ name: 'Ready', data: o.readyHist, color: '#ffb454' }, { name: 'Unacked', data: o.unackedHist, color: '#5ab0ff' }], height: 120, range: 60 }),
        h('h4', { style: 'margin-top:10px' }, 'Message rates'),
        RL.lineChart({ series: [{ name: 'Publish', data: o.publish, color: '#7ee787' }, { name: 'Deliver / get', data: o.deliver, color: '#ffb454' }, { name: 'Ack', data: o.ack, color: '#c792ea' }], unit: '/s', height: 120, range: 60 }),
        h('div', { class: 'muted', style: 'font-size:12px;margin-top:6px' }, o.caption || ''),
      );
    }
    function connections() {
      view.append(h('h4', null, `Connections (${spec.connections.length})`), h('table', { class: 't' }, [
        h('tr', null, ['Name / client', 'User', 'State', 'Channels'].map((x) => h('th', null, x))),
        spec.connections.map((c) => h('tr', null, [h('td', { class: 'mono' }, c.name), h('td', null, c.user), h('td', { class: c.state === 'running' ? 'state-run' : 'bad-t' }, c.state), h('td', null, c.channels)])),
      ]), spec.connections.length === 0 ? h('div', { class: 'verdict bad' }, 'Hiç bağlantı yok.') : null,
      h('h4', { style: 'margin-top:12px' }, `Channels (${spec.channels.length})`), h('table', { class: 't' }, [
        h('tr', null, ['Channel', 'Prefetch', 'Unacked', 'Consumers'].map((x) => h('th', null, x))),
        spec.channels.map((c) => h('tr', null, [h('td', { class: 'mono' }, c.name), h('td', null, c.prefetch), h('td', null, c.unacked), h('td', null, c.consumers)])),
      ]));
    }
    function exchanges() {
      view.append(h('table', { class: 't' }, [
        h('tr', null, ['Name', 'Type', 'Features'].map((x) => h('th', null, x))),
        spec.exchanges.map((e) => h('tr', { class: 'click' + (e.name === 'rovena.events' ? '' : ''), onclick: () => emit('exchange:' + e.name) }, [h('td', { class: 'mono' }, e.name), h('td', null, e.type), h('td', null, e.features)])),
      ]), h('h4', { style: 'margin-top:12px' }, 'Bindings of rovena.events (routing key → queue)'),
      h('table', { class: 't' }, [h('tr', null, ['Routing key (olay tipi)', '→ Queue'].map((x) => h('th', null, x))), spec.bindings.map((b) => h('tr', null, [h('td', { class: 'mono' }, b[0]), h('td', { class: 'mono' }, b[1])]))]));
    }
    function queues() {
      if (queue) return detail();
      view.append(h('table', { class: 't' }, [
        h('tr', null, ['Name', 'State', 'Ready', 'Unacked', 'Total', 'incoming', 'deliver/get', 'ack', 'Consumers'].map((x, i) => h('th', { class: i > 1 ? 'num' : '' }, x))),
        spec.queues.map((q) => h('tr', { class: 'click', tabindex: 0, onclick: () => { queue = q; draw(); emit('queue:' + q.name); } }, [
          h('td', { class: 'mono' }, q.name), h('td', { class: q.state === 'idle' ? 'state-idle' : 'state-run' }, q.state),
          h('td', { class: 'num ' + (q.ready > 100 ? 'warn-t' : '') }, q.ready), h('td', { class: 'num' }, q.unacked), h('td', { class: 'num' }, q.ready + q.unacked),
          h('td', { class: 'num' }, rate(q.incoming)), h('td', { class: 'num' }, rate(q.deliver)), h('td', { class: 'num' }, rate(q.ack)),
          h('td', { class: 'num ' + (q.consumers === 0 && !/retry|dlq/.test(q.name) ? 'bad-t' : '') }, q.consumers),
        ])),
      ]), h('div', { class: 'muted', style: 'font-size:12px;margin-top:6px' }, spec.queueHint || 'Bir kuyruğa tıkla: grafikler, bağlı consumer’lar ve mesajlar.'));
    }
    function detail() {
      const q = queue;
      const msgs = h('div');
      view.append(
        h('button', { class: 'btn small ghost', onclick: () => { queue = null; draw(); } }, '← Queues'),
        h('h3', { style: 'margin-top:8px' }, q.name),
        h('div', { class: 'rm-stat' }, [
          h('div', null, [h('small', null, 'Ready'), h('b', null, q.ready)]), h('div', null, [h('small', null, 'Unacked'), h('b', null, q.unacked)]),
          h('div', null, [h('small', null, 'Consumers'), h('b', null, q.consumers)]), h('div', null, [h('small', null, 'Features'), h('b', null, q.features || 'D')]),
        ]),
        q.hist ? RL.lineChart({ series: [{ name: 'Ready', data: q.hist.ready, color: '#ffb454' }, { name: 'Unacked', data: q.hist.unacked, color: '#5ab0ff' }], height: 110, range: 60 }) : null,
        q.hist && q.hist.rates ? RL.lineChart({ series: q.hist.rates, unit: '/s', height: 110, range: 60 }) : null,
        h('h4', { style: 'margin-top:10px' }, 'Consumers'),
        q.consumersList && q.consumersList.length
          ? h('table', { class: 't' }, [h('tr', null, ['Channel', 'Ack required', 'Prefetch'].map((x) => h('th', null, x))), q.consumersList.map((c) => h('tr', null, [h('td', { class: 'mono' }, c.channel), h('td', null, 'yes'), h('td', null, c.prefetch)]))])
          : h('div', { class: 'verdict bad' }, 'Bu kuyruğu dinleyen consumer yok.'),
        q.arguments ? [h('h4', { style: 'margin-top:10px' }, 'Arguments'), h('div', { class: 'mono' }, q.arguments)] : null,
        h('h4', { style: 'margin-top:10px' }, 'Get messages'),
        h('button', { class: 'btn small', onclick: () => {
          clear(msgs); emit('get-messages:' + q.name);
          if (!q.messages || !q.messages.length) { msgs.append(h('div', { class: 'muted' }, 'The queue is empty')); return; }
          q.messages.forEach((m) => msgs.append(RL.code(m)));
          msgs.append(h('div', { class: 'note-sim' }, 'Ack mode "Nack message requeue true" ile bak: mesajı tüketmeden okursun. "Automatic ack" mesajı kuyruktan SİLER.'));
        } }, 'Get Message(s)'), msgs,
      );
    }
    draw();
  }

  /* ================= Terminal / log ================= */
  function logs(root, spec) {
    root.append(h('div', { class: 'mono muted', style: 'margin-bottom:6px' }, '$ ' + spec.command),
      h('div', { class: 'logs', html: spec.lines.map((l) => l.replace(/trace_id=(\w+)/, '<span class="tid">trace_id=$1</span>').replace(/level=ERROR/, '<span class="lv-e">level=ERROR</span>').replace(/level=WARN/, '<span class="lv-w">level=WARN</span>')).join('\n') }));
  }
})();
