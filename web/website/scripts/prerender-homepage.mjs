import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import { createElement } from 'react';
import { renderToString } from 'react-dom/server';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const server = await createServer({ root, server: { middlewareMode: true }, appType: 'custom' });
try {
  const { App } = await server.ssrLoadModule('/src/App.jsx');
  const markup = renderToString(createElement(App));
  const index = path.join(root, 'dist/client/index.html');
  const template = readFileSync(index, 'utf8');
  if (!template.includes('<div id="root"></div>')) throw new Error('Homepage root missing');
  writeFileSync(index, template.replace('<div id="root"></div>', `<div id="root">${markup}</div>`));
  console.log('Prerendered homepage content for visitors and crawlers.');
} finally {
  await server.close();
}
