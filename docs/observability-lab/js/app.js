(function () {
  const inited = {};
  const modes = { flow: () => RL.initFlow(document.getElementById('mode-flow')), lab: () => RL.initLab(document.getElementById('mode-lab')), guide: () => RL.initGuide(document.getElementById('mode-guide')) };
  function show(mode) {
    document.querySelectorAll('#modes button').forEach((b) => b.classList.toggle('on', b.dataset.mode === mode));
    ['flow', 'lab', 'guide'].forEach((m) => (document.getElementById('mode-' + m).hidden = m !== mode));
    if (!inited[mode]) { inited[mode] = true; modes[mode](); }
    try { history.replaceState(null, '', '#' + mode); } catch (e) { /* file:// */ }
  }
  document.querySelectorAll('#modes button').forEach((b) => b.addEventListener('click', () => show(b.dataset.mode)));
  RL.show = show;
  const start = (location.hash || '').slice(1);
  show(modes[start] ? start : 'flow');
})();
