import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.dirname(fileURLToPath(import.meta.url));
const lock = JSON.parse(fs.readFileSync(path.join(root, 'package-lock.json'), 'utf8'));
const sections = [];

for (const [location, metadata] of Object.entries(lock.packages).sort(([left], [right]) => left.localeCompare(right))) {
  if (!location.startsWith('node_modules/') || metadata.dev) continue;
  const packageRoot = path.join(root, location);
  let files = fs.readdirSync(packageRoot)
    .filter((name) => /^(license|licence|notice)(\.|$)/i.test(name))
    .sort();
  if (files.length === 0 && location === 'node_modules/lru_map') {
    files = ['README.md'];
  }
  if (files.length === 0) {
    throw new Error(`No license or notice file found for ${metadata.name || location}`);
  }
  const name = metadata.name || location.slice('node_modules/'.length);
  const texts = files.map((file) => {
    let contents = fs.readFileSync(path.join(packageRoot, file), 'utf8').trimEnd();
    if (location === 'node_modules/lru_map' && file === 'README.md') {
      contents = contents.slice(contents.indexOf('# MIT license')).trimEnd();
    }
    return `--- ${file} ---\n${contents}`;
  });
  sections.push(`${name} ${metadata.version}\n${'='.repeat(name.length + String(metadata.version).length + 1)}\n\n${texts.join('\n\n')}`);
}

const heading = 'Alias Lens browser bundle: third-party license texts\n=======================================================\n\n';
fs.writeFileSync(path.join(root, '../../../THIRD_PARTY_LICENSES.txt'), `${heading}${sections.join('\n\n\n')}\n`);
