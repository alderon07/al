import { build } from 'esbuild';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.dirname(fileURLToPath(import.meta.url));

await build({
  entryPoints: [path.join(root, 'src/diff.js')],
  bundle: true,
  minify: true,
  format: 'esm',
  target: 'es2020',
  outfile: path.join(root, '../web/diff.js'),
  legalComments: 'linked',
  plugins: [{
    name: 'json-only-shiki',
    setup(buildContext) {
      buildContext.onResolve({ filter: /^shiki$/ }, () => ({
        path: path.join(root, 'src/shiki-json.js'),
      }));
    },
  }],
});
