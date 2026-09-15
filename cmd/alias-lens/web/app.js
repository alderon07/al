const search = document.querySelector('#search');
const results = document.querySelector('#results');
const suggestion = document.querySelector('#suggestion');
const resultCount = document.querySelector('#result-count');
const fileCount = document.querySelector('#file-count');
const template = document.querySelector('#alias-card');
let aliases = [];
let selected = 0;

const normalize = (value) => value.toLowerCase().replace(/[^a-z0-9]/g, '');
const distance = (left, right) => {
  const rows = Array.from({ length: left.length + 1 }, (_, i) => [i]);
  for (let j = 0; j <= right.length; j++) rows[0][j] = j;
  for (let i = 1; i <= left.length; i++) for (let j = 1; j <= right.length; j++) rows[i][j] = Math.min(rows[i - 1][j] + 1, rows[i][j - 1] + 1, rows[i - 1][j - 1] + (left[i - 1] === right[j - 1] ? 0 : 1));
  return rows[left.length][right.length];
};
function matches(alias, query) {
  const name = normalize(alias.name); const q = normalize(query);
  if (!q) return true;
	if (q.length === 1) return name.startsWith(q);
  return name.includes(q);
}
function render() {
  const query = search.value.trim();
  const found = aliases.filter((alias) => matches(alias, query));
  results.innerHTML = '';
  selected = Math.min(selected, Math.max(0, found.length - 1));
	if (query.length < 1) {
		resultCount.textContent = `${aliases.length} aliases ready · type to search`;
    suggestion.hidden = true;
    return;
  }
  resultCount.textContent = found.length ? `${found.length} match${found.length === 1 ? '' : 'es'}` : 'No direct match';
  found.forEach((alias, index) => {
    const card = template.content.cloneNode(true);
    const article = card.querySelector('.alias-card');
    article.dataset.command = alias.command;
    article.classList.toggle('active', index === selected);
    card.querySelector('.alias-name').textContent = alias.name;
    card.querySelector('.category').textContent = alias.category;
    card.querySelector('.description').textContent = alias.description;
    card.querySelector('.command').textContent = alias.command;
    card.querySelector('.copy').addEventListener('click', () => copy(alias.command));
    article.addEventListener('click', () => copy(alias.command));
    results.append(card);
  });
  const q = normalize(query);
  const close = aliases.filter((alias) => distance(normalize(alias.name), q) <= Math.max(1, Math.floor(q.length / 3))).slice(0, 4);
  if (!found.length && close.length) {
    suggestion.hidden = false;
    suggestion.innerHTML = `Did you mean ${close.map((alias) => `<button type="button" data-alias="${alias.name}">${alias.name}</button>`).join('')}?`;
    suggestion.querySelectorAll('button').forEach((button) => button.addEventListener('click', () => { search.value = button.dataset.alias; render(); search.focus(); }));
  } else suggestion.hidden = true;
}
function copy(command) { navigator.clipboard.writeText(command); resultCount.textContent = 'Command copied to clipboard'; }
async function load() {
  resultCount.textContent = 'Reading ~/.bash_aliases…';
  const response = await fetch('/api/aliases', { cache: 'no-store' });
  aliases = await response.json();
  fileCount.textContent = `${aliases.length} aliases loaded`;
  render();
}
search.addEventListener('input', () => { selected = 0; render(); });
search.addEventListener('keydown', (event) => {
  const cards = [...document.querySelectorAll('.alias-card')];
  if (event.key === 'Escape') { search.value = ''; render(); }
  if (event.key === 'ArrowDown' && cards.length) { event.preventDefault(); selected = Math.min(selected + 1, cards.length - 1); render(); }
  if (event.key === 'ArrowUp' && cards.length) { event.preventDefault(); selected = Math.max(selected - 1, 0); render(); }
  if (event.key === 'Enter' && cards[selected]) copy(cards[selected].dataset.command);
});
document.querySelector('#reload').addEventListener('click', load);
load();
