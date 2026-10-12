(function () {
  const init = { flow: RL.initFlow, failure: RL.initFailure, arch: RL.initArch, code: RL.initCode, testing: RL.initTesting, obs: RL.initObs };
  const done = {};
  RL.showTab = function (tab) {
    document.querySelectorAll('#tabs button').forEach((b) => b.classList.toggle('on', b.dataset.tab === tab));
    Object.keys(init).forEach((k) => (document.getElementById('tab-' + k).hidden = k !== tab));
    if (!done[tab]) { done[tab] = true; init[tab](document.getElementById('tab-' + tab)); }
    try { history.replaceState(null, '', '#' + tab); } catch (e) { /* file:// */ }
  };
  // lab.js (gözlem laboratuvarından) son senaryoda RL.show('guide') çağırır.
  RL.show = (m) => RL.showTab(init[m] ? m : 'code');
  document.querySelectorAll('#tabs button').forEach((b) => b.addEventListener('click', () => RL.showTab(b.dataset.tab)));
  const start = (location.hash || '').slice(1);
  RL.showTab(init[start] ? start : 'flow');
})();
