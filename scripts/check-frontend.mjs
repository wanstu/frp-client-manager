import fs from 'node:fs';

const jsPath = 'cmd/frp-client-desktop/frontend/app.js';
const htmlPath = 'cmd/frp-client-desktop/frontend/index.html';

const js = fs.readFileSync(jsPath, 'utf8');
const html = fs.readFileSync(htmlPath, 'utf8');

const ids = new Set([...html.matchAll(/id="([^"]+)"/g)].map((match) => match[1]));
const refs = [...js.matchAll(/\$\('([^']+)'\)/g)].map((match) => match[1]);
const missing = [...new Set(refs.filter((id) => !ids.has(id)))];

if (missing.length > 0) {
  console.error('Missing DOM ids:', missing.join(', '));
  process.exit(1);
}

console.log('Frontend DOM references OK: ' + new Set(refs).size + ' ids');
