import { FileDiff } from '@pierre/diffs';

const token = new URLSearchParams(window.location.hash.slice(1)).get('token') || '';
if (token) {
  window.history.replaceState(null, '', window.location.pathname);
}

const state = {
  summary: null,
  details: null,
  filter: '',
  layout: window.matchMedia('(max-width: 760px)').matches ? 'unified' : 'split',
  instances: [],
};

const elements = {
  status: document.querySelector('#status'),
  source: document.querySelector('#source-label'),
  counts: document.querySelector('#counts'),
  changes: document.querySelector('#changes'),
  filter: document.querySelector('#change-filter'),
  exactButton: document.querySelector('#show-exact'),
  exactPanel: document.querySelector('#exact-panel'),
  exactList: document.querySelector('#exact-list'),
  layoutButtons: [...document.querySelectorAll('[data-layout]')],
};

elements.layoutButtons.forEach((button) => {
  button.setAttribute('aria-pressed', String(button.dataset.layout === state.layout));
});

const labels = {
  entry: 'whole entry',
  schema_version: 'data format',
  name: 'name',
  kind: 'type',
  description: 'description',
  category: 'category',
  tags: 'tags',
  platforms: 'supported computers',
  favorite: 'favorite setting',
  portable: 'portable command',
  'native.bash': 'Bash command',
  'native.zsh': 'Zsh command',
  'when.profiles_any': 'machine profiles',
  'when.profiles_none': 'excluded machine profiles',
  'when.shells': 'supported shells',
};

function request(path) {
  return fetch(path, {
    cache: 'no-store',
    headers: { Authorization: `Bearer ${token}` },
  }).then(async (response) => {
    if (!response.ok) throw new Error(`request failed (${response.status})`);
    return response.json();
  });
}

function changeLabel(kind) {
  if (kind === 'added') return 'Add';
  if (kind === 'deleted') return 'Remove';
  return 'Change';
}

function setStatus(message, tone = '') {
  elements.status.textContent = message;
  elements.status.dataset.tone = tone;
}

function renderCounts(summary) {
  const counts = [
    ['added', summary.added, 'Added'],
    ['deleted', summary.deleted, 'Removed'],
    ['changed', summary.changed, 'Changed'],
  ];
  elements.counts.replaceChildren(...counts.map(([kind, value, label]) => {
    const card = document.createElement('article');
    card.className = `count-card count-${kind}`;
    const number = document.createElement('strong');
    number.textContent = value;
    const text = document.createElement('span');
    text.textContent = label;
    card.append(number, text);
    return card;
  }));
}

function renderChanges() {
  const query = state.filter.toLowerCase();
  const changes = state.summary.changes.filter((change) => {
    return `${change.name} ${change.kind} ${change.path}`.toLowerCase().includes(query);
  });
  if (!changes.length) {
    const empty = document.createElement('p');
    empty.className = 'empty-state';
    empty.textContent = state.summary.changes.length ? 'No changes match that filter.' : 'The catalogs match. Nothing needs attention.';
    elements.changes.replaceChildren(empty);
    return;
  }
  elements.changes.replaceChildren(...changes.map((change, index) => {
    const item = document.createElement('article');
    item.className = `change-row change-${change.kind}`;
    item.style.setProperty('--order', index);
    const marker = document.createElement('span');
    marker.className = 'change-marker';
    marker.textContent = change.kind === 'added' ? '+' : change.kind === 'deleted' ? '−' : '~';
    marker.setAttribute('aria-hidden', 'true');
    const body = document.createElement('div');
    const title = document.createElement('h3');
    title.textContent = change.name || 'Catalog settings';
    const detail = document.createElement('p');
    detail.textContent = `${changeLabel(change.kind)} ${labels[change.path] || 'details'}`;
    body.append(title, detail);
    const badge = document.createElement('span');
    badge.className = 'change-badge';
    badge.textContent = changeLabel(change.kind);
    item.append(marker, body, badge);
    return item;
  }));
}

function cleanDiffs() {
  state.instances.forEach((instance) => instance.cleanUp());
  state.instances = [];
}

function renderExact() {
  cleanDiffs();
  elements.exactList.replaceChildren();
  state.details.files.forEach((file) => {
    const wrapper = document.createElement('article');
    wrapper.className = 'diff-card';
    const heading = document.createElement('div');
    heading.className = 'diff-heading';
    const title = document.createElement('h3');
    title.textContent = file.display_name;
    const badge = document.createElement('span');
    badge.textContent = changeLabel(file.kind);
    heading.append(title, badge);
    const container = document.createElement('div');
    container.className = 'pierre-host';
    wrapper.append(heading, container);
    elements.exactList.append(wrapper);

    const instance = new FileDiff({
      theme: { dark: 'pierre-dark', light: 'pierre-light' },
      themeType: 'system',
      preferredHighlighter: 'shiki-js',
      diffStyle: state.layout,
      diffIndicators: 'classic',
      overflow: 'wrap',
      disableFileHeader: true,
      expandUnchanged: true,
      lineDiffType: 'word',
    });
    instance.render({
      oldFile: file.before_text === null ? null : { name: file.filename, contents: file.before_text, lang: 'json' },
      newFile: file.after_text === null ? null : { name: file.filename, contents: file.after_text, lang: 'json' },
      containerWrapper: container,
    });
    state.instances.push(instance);
  });
}

async function showExactChanges() {
  elements.exactButton.disabled = true;
  elements.exactButton.textContent = 'Opening exact changes…';
  try {
    state.details = await request('/api/catalog-diff/details');
    elements.exactPanel.hidden = false;
    elements.exactButton.hidden = true;
    renderExact();
    elements.exactPanel.focus();
  } catch (error) {
    elements.exactButton.disabled = false;
    elements.exactButton.textContent = 'Try exact changes again';
    setStatus('Exact changes could not be opened. Your files were not changed.', 'error');
  }
}

async function start() {
  if (!token) {
    setStatus('Open the private URL printed by Alias Lens.', 'error');
    elements.exactButton.disabled = true;
    return;
  }
  try {
    state.summary = await request('/api/catalog-diff');
    const total = state.summary.summary.added + state.summary.summary.deleted + state.summary.summary.changed;
    elements.source.textContent = state.summary.source === 'repository' ? 'repository copy → this computer' : `${state.summary.shell || 'saved'} install → this computer`;
    renderCounts(state.summary.summary);
    renderChanges();
    if (state.summary.diagnostics.length) {
      setStatus(state.summary.diagnostics[0].message, 'error');
      elements.exactButton.disabled = true;
    } else if (total === 0) {
      setStatus('Everything matches. Nothing has been changed.', 'quiet');
      elements.exactButton.disabled = true;
    } else {
      setStatus(`${total} ${total === 1 ? 'entry needs' : 'entries need'} review. Nothing has been changed.`, 'ready');
    }
  } catch (error) {
    setStatus('The private comparison could not be loaded. Restart Alias Lens and try again.', 'error');
    elements.exactButton.disabled = true;
  }
}

elements.filter.addEventListener('input', () => {
  state.filter = elements.filter.value.trim();
  if (state.summary) renderChanges();
});
elements.exactButton.addEventListener('click', showExactChanges);
elements.layoutButtons.forEach((button) => button.addEventListener('click', () => {
  state.layout = button.dataset.layout;
  elements.layoutButtons.forEach((item) => item.setAttribute('aria-pressed', String(item === button)));
  if (state.details) renderExact();
}));
window.addEventListener('pagehide', cleanDiffs);
start();
