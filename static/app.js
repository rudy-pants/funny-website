(() => {
  const discoveries = document.querySelector('#discoveries');
  const status = document.querySelector('[data-map-status]');
  const count = document.querySelector('[data-discovery-count]');
  const reset = document.querySelector('[data-reset]');
  const regions = [...document.querySelectorAll('[data-region-id]')];

  if (!discoveries || !status || !count || !reset || !regions.length) return;

  const startingRegion = 'dunes';
  const emptyState = discoveries.querySelector('.empty-state')?.outerHTML ||
    '<div class="empty-state"><span class="empty-icon" aria-hidden="true">⌖</span><div><h3>Your notebook is blank.</h3><p>Choose a region above and your first joke will appear here.</p></div></div>';
  const unlocked = new Set([startingRegion]);
  const names = new Map(regions.map((link) => [
    link.dataset.regionId,
    link.getAttribute('aria-label')?.split(' — ')[0] || link.dataset.regionId,
  ]));

  const regionName = (id) => names.get(id) || id;

  const setRegionState = (link, open) => {
    const id = link.dataset.regionId;
    link.classList.toggle('is-unlocked', open);
    link.classList.toggle('is-locked', !open);
    link.setAttribute('aria-label', `${regionName(id)} — ${open ? 'Open trail; activate to read a joke' : 'New territory; activate to unlock'}`);
  };

  const refreshCount = () => {
    const found = new Set([...document.querySelectorAll('[data-discovery]')].map((card) => card.dataset.discovery)).size;
    count.textContent = String(found);
  };

  const reveal = (id) => {
    if (!id) return;
    unlocked.add(id);
    regions.forEach((link) => setRegionState(link, unlocked.has(link.dataset.regionId)));
  };

  document.body.addEventListener('htmx:beforeRequest', (event) => {
    if (event.detail?.requestConfig?.path?.startsWith('/unlock')) {
      status.textContent = 'Consulting the joke compass…';
    }
  });

  document.body.addEventListener('htmx:afterSwap', (event) => {
    const target = event.detail?.target;
    const card = target?.matches('[data-discovery]') ? target : target?.querySelector('[data-discovery]');
    if (card) {
      reveal(card.dataset.discovery);
      card.dataset.unlocks?.trim().split(/\s+/).filter(Boolean).forEach(reveal);
      status.textContent = `${regionName(card.dataset.discovery)} added to your field notes.`;
      refreshCount();
      card.classList.add('is-arriving');
      window.setTimeout(() => card.classList.remove('is-arriving'), 700);
    }
  });

  regions.forEach((link) => {
    link.addEventListener('click', () => {
      status.textContent = `${regionName(link.dataset.regionId)} is being charted…`;
    });
  });

  reset.addEventListener('click', () => {
    unlocked.clear();
    unlocked.add(startingRegion);
    regions.forEach((link) => setRegionState(link, link.dataset.regionId === startingRegion));
    discoveries.innerHTML = emptyState;
    count.textContent = '0';
    status.textContent = 'Dad Joke Dunes is your starting point.';
    document.querySelector('#map')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  });

  regions.forEach((link) => setRegionState(link, unlocked.has(link.dataset.regionId)));
  refreshCount();
})();
