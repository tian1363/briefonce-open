import { readFileSync, writeFileSync, readdirSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const site = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const root = path.resolve(site, '../..');
const output = path.join(site, 'dist/client');
const origin = 'https://tian1363.github.io/briefonce-open/';
const verification = process.env.GOOGLE_SITE_VERIFICATION?.trim();
if (verification && !/^[\w-]{10,256}$/.test(verification)) throw new Error('Use the verification content value, not the entire HTML tag.');
const escape = value => value.replaceAll('&', '&amp;').replaceAll('"', '&quot;').replaceAll('<', '&lt;');
const json = value => JSON.stringify(value).replaceAll('<', '\\u003c');
const plain = value => value.replace(/<[^>]*>/g, '').replaceAll('&amp;', '&').trim();
const pages = [];
function collect(dir) {
  for (const file of readdirSync(dir, { withFileTypes: true })) {
    if (file.isDirectory()) collect(path.join(dir, file.name));
    else if (file.name === 'index.html') pages.push(path.join(dir, file.name));
  }
}
collect(output);
const urls = [];
for (const file of pages) {
  let html = readFileSync(file, 'utf8');
  const canonical = html.match(/<link rel="canonical" href="([^"]+)"/)[1];
  if (!canonical.startsWith(origin)) throw new Error('Unexpected canonical outside the product website.');
  const title = plain(html.match(/<title>([\s\S]*?)<\/title>/)[1]);
  const description = html.match(/<meta name="description" content="([^"]+)"/)[1];
  const slug = canonical.slice(origin.length);
  const language = slug === 'en/' ? 'en' : 'zh-CN';
  html = html.replace(/(<meta name="robots" content=")[^"]+"/, '$1index,follow,max-image-preview:large"');
  const metadata = [];
  if (!html.includes('property="og:site_name"')) metadata.push('<meta property="og:site_name" content="BriefOnce">');
  if (!html.includes('name="twitter:card"')) metadata.push(`<meta name="twitter:card" content="summary_large_image"><meta name="twitter:title" content="${escape(title)}"><meta name="twitter:description" content="${description}"><meta name="twitter:image" content="${origin}og-cover.jpg">`);
  if (verification && !slug) metadata.push(`<meta name="google-site-verification" content="${verification}">`);
  const graph = [{ '@type': 'WebPage', '@id': canonical+'#page', url: canonical, name: title, description: plain(description), inLanguage: language }];
  if (slug && slug !== 'en/') {
    const parts = slug.split('/').filter(Boolean);
    const crumbs = [{ '@type': 'ListItem', position: 1, name: 'BriefOnce', item: origin }];
    if (parts.length > 1) crumbs.push({ '@type': 'ListItem', position: 2, name: parts[0] === 'skills' ? '创意 Skill' : '使用案例', item: origin+parts[0]+'/' });
    crumbs.push({ '@type': 'ListItem', position: crumbs.length+1, name: title.replace(/ · BriefOnce$/, ''), item: canonical });
    graph.push({ '@type': 'BreadcrumbList', itemListElement: crumbs });
  }
  metadata.push(`<script type="application/ld+json">${json({ '@context': 'https://schema.org', '@graph': graph })}</script>`);
  html = html.replace('</head>', metadata.join('\n')+'\n</head>');
  writeFileSync(file, html);
  const images = [...new Set([...html.matchAll(/<img[^>]+src="([^"]+)"/g)].map(([,src]) => src).filter(src => /assets\/cases\/(?:pose-[1-4]|jacket-input)\.png$/.test(src)).map(src => origin+src.slice(src.indexOf('assets/'))))];
  let sources;
  if (slug.startsWith('examples/') && slug !== 'examples/') sources = [`docs/${slug.slice(0,-1)}.md`, 'web/website/scripts/generate-public-examples.mjs'];
  else if (slug.startsWith('skills/')) sources = ['internal/chat/service.go', 'web/website/scripts/generate-public-skills.mjs'];
  else sources = ['web/website/src', 'web/website/index.html', 'web/website/scripts'];
  let modified = '';
  try { modified = execFileSync('git', ['log','-1','--format=%cI','--',...sources], {cwd:root,encoding:'utf8'}).trim(); } catch {}
  urls.push(`  <url><loc>${canonical}</loc>${modified ? `<lastmod>${modified}</lastmod>` : ''}${images.map(src=>`<image:image><image:loc>${escape(src)}</image:loc></image:image>`).join('')}</url>`);
}
writeFileSync(path.join(output,'sitemap.xml'), `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:image="http://www.google.com/schemas/sitemap-image/1.1">\n${urls.sort().join('\n')}\n</urlset>\n`);
writeFileSync(path.join(output,'404.html'), `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="robots" content="noindex,follow"><title>页面未找到 · BriefOnce</title></head><body><main><h1>这个页面暂时找不到了。</h1><a href="${origin}">返回 BriefOnce 官网 / Back to BriefOnce</a></main></body></html>`);
console.log(`Finalized SEO for ${urls.length} static pages. Google verification ${verification ? 'configured' : 'awaiting owner token'}.`);
