import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import { createElement } from 'react';
import { renderToString } from 'react-dom/server';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const origin = 'https://tian1363.github.io/briefonce-open/';
// SSR rendering does not need the client dependency scanner or browser warmup.
const server = await createServer({
  root,
  optimizeDeps: { noDiscovery: true, include: [] },
  server: { middlewareMode: true, warmup: { clientFiles: [] } },
  appType: 'custom',
});
try {
  const { App } = await server.ssrLoadModule('/src/App.jsx');
  const index = path.join(root, 'dist/client/index.html');
  const template = readFileSync(index, 'utf8');
  if (!template.includes('<div id="root"></div>')) throw new Error('Homepage root missing');
  for (const language of ['zh', 'en']) {
    const markup = renderToString(createElement(App, { initialLang: language }));
    let html = template.replace('<div id="root"></div>', `<div id="root">${markup}</div>`);
    let output = index;
    if (language === 'en') {
      const title = 'BriefOnce | AI Product Photography & Ecommerce Creative Workflows';
      const description = 'Create AI model pose images, refine product photos and generate ecommerce detail images. Explore real apparel examples, reusable creative Skills and a self-hosted workspace.';
      html = html.replace('<html lang="zh-CN">', '<html lang="en">')
        .replace(/<title>[^<]*<\/title>/, `<title>${title.replaceAll('&', '&amp;')}</title>`)
        .replace(/(<link rel="canonical" href=")[^"]+/, `$1${origin}en/`)
        .replace(/(<meta property="og:url" content=")[^"]+/, `$1${origin}en/`)
        .replace(/(<meta property="og:locale" content=")[^"]+/, '$1en_US')
        .replace(/(<meta (?:name="description"|property="og:description"|name="twitter:description") content=")[^"]+"/g, `$1${description}"`)
        .replace(/(<meta (?:property="og:title"|name="twitter:title") content=")[^"]+"/g, `$1${title.replaceAll('&', '&amp;')}"`);
      output = path.join(root, 'dist/client/en/index.html');
      mkdirSync(path.dirname(output), { recursive: true });
    }
    writeFileSync(output, html);
  }
  console.log('Prerendered separate Chinese and English homepages.');
} finally {
  await server.close();
}
