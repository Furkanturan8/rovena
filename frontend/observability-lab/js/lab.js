/*
 * Mod 2: senaryo tabanlı laboratuvar. Her senaryo aşamalardan (stage) oluşur; aşama bir 'phase' altında gruplanır:
 * Durum → Belirti → İlk kontrol → İnceleme → Teşhis → Aksiyon → Doğrulama.
 * Aşama türleri: düz anlatım, çoktan seçmeli (choice), araçta bir şey bulma görevi (task).
 * Cevap önce gösterilmez: kullanıcı seçer/bulur, sonra açıklama gelir. Yanlış seçimde de ilgili aracın (yanlış) ekranı görünebilir.
 */
(function () {
  const { h, clear } = RL;
  const PHASES = [['durum', '1 · Durum'], ['belirti', '2 · Belirti'], ['ilk', '3 · İlk kontrol'], ['inceleme', '4 · İnceleme'], ['teshis', '5 · Teşhis'], ['aksiyon', '6 · Aksiyon'], ['dogrulama', '7 · Doğrulama']];
  const store = { done: {}, first: {} };
  try { Object.assign(store, JSON.parse(localStorage.getItem('rovena-lab') || '{}')); } catch (e) { /* kalıcılık opsiyonel */ }
  const save = () => { try { localStorage.setItem('rovena-lab', JSON.stringify(store)); } catch (e) { /* yok say */ } };

  RL.initLab = function (root) {
    const list = h('div', { class: 'sclist' });
    const main = h('div');
    root.append(h('div', { class: 'labgrid' }, [list, main]));
    let cur = null;

    function drawList() {
      clear(list);
      list.append(h('h4', null, 'Senaryolar'));
      RL.scenarios.forEach((sc) => list.append(h('button', { class: 'sc' + (cur && cur.id === sc.id ? ' on' : ''), onclick: () => start(sc) }, [
        store.done[sc.id] ? h('span', { class: 'done' }, '✔') : null, h('b', null, `${sc.id} · ${sc.title}`), h('small', null, sc.tools.join(' · ')),
      ])));
      const n = Object.keys(store.done).length;
      list.append(h('div', { class: 'score card' }, [`Tamamlanan: ${n} / ${RL.scenarios.length}`, h('br'), h('button', { class: 'btn small ghost', style: 'margin-top:6px', onclick: () => { store.done = {}; store.first = {}; save(); drawList(); if (cur) start(cur); } }, 'İlerlemeyi sıfırla')]));
    }

    function intro() {
      clear(main).append(h('div', { class: 'card' }, [
        h('h2', null, 'Sorun nasıl tespit edilir?'),
        h('p', null, 'Her senaryo gerçek bir geliştiricinin gününden: bir belirti ile başlar, sen aracı seçer, metriği bulur, trace’i incelersin. Cevap hemen verilmez; önce sen karar verirsin, sonra açıklama gelir.'),
        h('p', { class: 'muted' }, 'Her senaryo aynı yedi aşamadan geçer:'),
        h('div', { class: 'phases' }, PHASES.map((p) => h('div', null, p[1]))),
        h('p', null, [h('span', { class: 'tag sim' }, 'SİMÜLE'), ' Araç pencerelerindeki sayılar ve grafikler eğitim için üretilmiştir (her açılışta aynıdır). ', h('span', { class: 'tag real' }, 'GERÇEK'), ' Metrik adları, PromQL sorguları, kuyruk adları, uyarı kuralları ve Rovena’daki arayüz adresleri kod tabanından alınmıştır; bu yüzden aynı sorguları çalışan yığında da deneyebilirsin (', h('code', null, 'make up'), ').']),
        h('p', null, 'Soldan bir senaryo seç.'),
      ]));
    }

    function start(sc) {
      cur = sc; drawList();
      const st = { i: 0, picked: {}, wrong: {}, events: new Set(), done: {} };
      render(sc, st);
    }

    function render(sc, st) {
      const stage = sc.stages[st.i];
      const total = sc.stages.length;
      const events = st.events;
      const needsChoice = !!stage.choice && !st.done['c' + st.i];
      const needsTask = !!stage.task && !taskDone(stage, events, st.i);
      let ready = false; // araç pencereleri çizilirken olay yayabilir; gezinme düğmeleri henüz yok
      const box = h('div');
      clear(main).append(box);

      box.append(
        h('div', { class: 'card' }, [
          h('div', { class: 'progress' }, h('i', { style: `width:${Math.round(((st.i + 1) / total) * 100)}%` })),
          h('div', { class: 'phases' }, PHASES.map((p) => {
            const idx = PHASES.findIndex((q) => q[0] === stage.phase), me = PHASES.indexOf(p);
            return h('div', { class: me === idx ? 'cur' : me < idx ? 'past' : '' }, p[1]);
          })),
          h('div', { class: 'phase-tag' }, `${sc.id} · ${sc.title}  —  ${PHASES.find((p) => p[0] === stage.phase)[1]}`),
          h('h2', { style: 'margin-top:4px' }, stage.title),
          stage.body ? h('div', { html: stage.body }) : null,
          stage.code ? stage.code.map((c) => [c.title ? h('h4', { style: 'margin-top:10px' }, c.title) : null, RL.code(c.body, c.lang), c.explain ? h('div', { class: 'muted', html: c.explain }) : null]) : null,
        ]));

      const body = box.firstChild;
      if (stage.qa) body.append(h('div', { class: 'qa' }, [
        h('div', { class: 'q1' }, [h('b', null, 'Şu anda neye bakıyorum?'), h('span', { html: stage.qa.look })]),
        h('div', { class: 'q2' }, [h('b', null, 'Bu veri nereden geliyor?'), h('span', { html: stage.qa.source })]),
        h('div', { class: 'q3' }, [h('b', null, 'Yükselmesi / düşmesi ne anlama gelir?'), h('span', { html: stage.qa.meaning })]),
        h('div', { class: 'q4' }, [h('b', null, 'Sonraki adım'), h('span', { html: stage.qa.next })]),
      ]));

      // tool görünümü (statik)
      const viewHost = h('div');
      if (stage.view) viewHost.append(RL.tools.window(typeof stage.view === 'function' ? stage.view() : stage.view, (ev) => onEvent(ev)));
      body.append(viewHost);

      // görev
      let taskBox = null;
      if (stage.task) {
        taskBox = h('div', { class: 'task' + (!needsTask ? ' done' : '') }, [h('b', null, !needsTask ? '✔ Görev tamam: ' : 'Görev: '), stage.task.text,
          stage.task.hint ? h('div', { class: 'muted', style: 'margin-top:4px;font-size:12.5px' }, ['İpucu: ', stage.task.hint]) : null]);
        body.append(taskBox);
      }

      // seçim
      if (stage.choice) {
        const fbHost = h('div'), extra = h('div');
        const ch = stage.choice;
        const optsEl = h('div', { class: 'opts' });
        body.append(h('h3', { style: 'margin-top:14px' }, ch.q), optsEl, fbHost, extra);
        (ch.opts).forEach((o, k) => {
          const b = h('button', { class: 'opt' + (st.picked['c' + st.i + '-' + k] ? (o.ok ? ' good' : ' bad') : ''), onclick: () => pick(o, k, b) }, [h('span', { class: 'mono muted' }, String.fromCharCode(65 + k) + '  '), o.t]);
          optsEl.append(b);
          if (st.picked['c' + st.i + '-' + k]) fb(o);
        });
        function fb(o) {
          fbHost.append(h('div', { class: 'fb ' + (o.ok ? 'good' : 'bad') }, [h('b', null, o.ok ? 'Doğru. ' : 'Pek değil. '), h('span', { html: o.fb })]));
          if (o.view) { clear(extra); extra.append(RL.tools.window(o.view, onEvent)); }
        }
        function pick(o, k, b) {
          if (st.done['c' + st.i]) { /* çözüldükten sonra diğer seçeneklerin açıklamasını da okuyabilsin */ }
          st.picked['c' + st.i + '-' + k] = true;
          b.classList.add(o.ok ? 'good' : 'bad');
          if (!o.ok) { st.wrong[st.i] = (st.wrong[st.i] || 0) + 1; }
          if (o.ok && !st.done['c' + st.i]) {
            st.done['c' + st.i] = true;
            const key = sc.id + ':' + st.i;
            if (store.first[key] == null) { store.first[key] = (st.wrong[st.i] || 0) === 0; save(); }
          }
          fb(o);
          if (o.ok && ch.explain) { fbHost.append(h('div', { class: 'fb info', html: ch.explain })); }
          refreshNav();
        }
      }

      // gezinme
      const prev = h('button', { class: 'btn ghost', disabled: st.i === 0, onclick: () => { st.i--; render(sc, st); } }, '← Önceki');
      const next = h('button', { class: 'btn', onclick: () => { if (st.i < total - 1) { st.i++; render(sc, st); window.scrollTo({ top: 0, behavior: 'smooth' }); } else finish(sc, st); } }, st.i < total - 1 ? 'Devam →' : 'Senaryoyu bitir ✔');
      const skip = h('button', { class: 'btn small ghost', hidden: true, onclick: () => { st.done['c' + st.i] = true; events.add('__skip' + st.i); render(sc, st); } }, 'Bu aşamayı atla');
      box.append(h('div', { class: 'row-actions' }, [prev, next, skip, h('span', { class: 'score' }, `Aşama ${st.i + 1} / ${total}`)]));
      function refreshNav() {
        const blockedC = stage.choice && !st.done['c' + st.i];
        const blockedT = stage.task && !taskDone(stage, events, st.i);
        next.disabled = !!(blockedC || blockedT);
        skip.hidden = !(blockedC || blockedT);
        if (taskBox) {
          taskBox.classList.toggle('done', !blockedT);
          taskBox.firstChild.textContent = blockedT ? 'Görev: ' : '✔ Görev tamam: ';
        }
      }
      function onEvent(ev) { events.add(ev); if (ready) refreshNav(); }
      ready = true;
      refreshNav();
    }

    function taskDone(stage, events, idx) {
      if (!stage.task) return true;
      if (events.has('__skip' + idx)) return true;
      return stage.task.req.every((r) => [...events].some((e) => (r instanceof RegExp ? r.test(e) : e === r)));
    }

    function finish(sc, st) {
      store.done[sc.id] = true; save(); drawList();
      const keys = sc.stages.map((s, i) => (s.choice ? sc.id + ':' + i : null)).filter(Boolean);
      const ok = keys.filter((k) => store.first[k]).length;
      clear(main).append(h('div', { class: 'card summary' }, [
        h('h2', null, `Senaryo ${sc.id} tamamlandı`),
        h('p', null, `İlk denemede doğru: ${ok} / ${keys.length} karar noktası.`),
        h('div', { html: sc.summary }),
        h('div', { class: 'row-actions' }, [
          h('button', { class: 'btn ghost', onclick: () => start(sc) }, 'Senaryoyu tekrarla'),
          RL.scenarios.indexOf(sc) < RL.scenarios.length - 1 ? h('button', { class: 'btn', onclick: () => start(RL.scenarios[RL.scenarios.indexOf(sc) + 1]) }, 'Sonraki senaryo →') : h('button', { class: 'btn', onclick: () => RL.show('guide') }, 'Araç rehberine geç →'),
        ]),
      ]));
    }

    drawList(); intro();
    RL.startScenario = (id) => start(RL.scenarios.find((s) => s.id === id));
  };
})();
