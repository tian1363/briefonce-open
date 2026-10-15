import { readFileSync, writeFileSync, mkdirSync, rmSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const site = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const root = path.resolve(site, '../..');
const origin = 'https://tian1363.github.io/briefonce-open/';
const repository = 'https://github.com/tian1363/briefonce-open';
const base = '/' + (process.env.SITE_BASE_PATH || '/').split('/').filter(Boolean).join('/');
const local = value => `${base === '/' ? '/' : `${base}/`}${value}`;
const examples = [
  { slug: 'apparel-images', title: 'AI 服装模特图与多姿势商品图', description: '用同款外套的商品资料与姿势参考，准备一组模特展示图。多张商品图共享参考，每个姿势生成一张。' },
  { slug: 'reference-recreation', title: '参考图复刻：替换商品与模特的实际案例', description: '开发预览：以三栏服装展示为参考，替换商品与模特并保留构图。包含实际输入、原始生成结果和工作流截图，尚未包含在当前公开版。' },
  { slug: 'product-detail', title: '服装与鞋履 AI 商品详情页', description: '从商品卖点、材质与品类出发，选择实际详情页参考图，策划并生成商品详情长图。' },
  { slug: 'video-extraction', title: '视频素材提取与创意解读', description: '预览参考视频中解析成功的素材，导入产品资料，再继续写开头、脚本与分镜。' },
];
const escape = value => value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;');
function layout(title, description, url, body) {
  return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${escape(title)} · BriefOnce</title><meta name="description" content="${escape(description)}"><link rel="canonical" href="${url}"><meta name="robots" content="index,follow"><meta property="og:type" content="article"><meta property="og:title" content="${escape(title)} · BriefOnce"><meta property="og:description" content="${escape(description)}"><meta property="og:url" content="${url}"><meta property="og:image" content="${origin}og-cover.jpg"><link rel="stylesheet" href="${local('skills/style.css')}"><link rel="icon" href="${local('assets/briefonce-mark.png')}"><script type="application/ld+json">${JSON.stringify({'@context':'https://schema.org','@type':'WebPage',name:title,description,url}).replaceAll('<','\\u003c')}</script></head><body><header class="site-header"><a class="brand" href="${local('')}"><img src="${local('assets/briefonce-mark.png')}" alt=""><strong>BriefOnce</strong></a><nav><a href="${local('examples/')}">使用示例</a><a href="${local('skills/')}">Skill 库</a><a href="${repository}">GitHub ↗</a></nav></header>${body}<footer><a href="${local('')}">BriefOnce 官网</a><a href="${repository}">查看源码与本地运行方法 ↗</a></footer></body></html>`;
}
const output = path.join(site, 'public/examples');
rmSync(output, { recursive: true, force: true });
mkdirSync(output, { recursive: true });
writeFileSync(path.join(output, 'index.html'), layout('商品图与创作工作流示例', '从服装模特图、商品详情页到视频素材提取，查看实际案例，跟着步骤使用 BriefOnce。', `${origin}examples/`, `<main class="library"><p class="eyebrow">BRIEFONCE / EXAMPLES</p><h1>从一个例子开始。</h1><p class="lead">看看真实输入和生成结果，再开始自己的创作。</p><div class="skill-grid">${examples.map((item, index) => `<a class="skill-card" href="${local(`examples/${item.slug}/`)}">${['apparel-images','reference-recreation'].includes(item.slug) ? `<img src="${local(`assets/cases/${item.slug==='apparel-images'?'pose-1.png':'reference-result.png'}`)}" alt="${escape(item.title)}实际生成结果" loading="lazy" style="width:100%;height:180px;object-fit:contain;background:#f3eee8;border-radius:8px;margin-bottom:18px">` : ''}<span class="card-index">0${index + 1} / ${item.slug==='reference-recreation'?'DEVELOPMENT PREVIEW':'WORKFLOW'}</span><h2>${escape(item.title)}</h2><p>${escape(item.description)}</p><span class="card-link">查看步骤 <b>↗</b></span></a>`).join('')}</div></main>`));
for (const item of examples) {
  const markdown = readFileSync(path.join(root, `docs/examples/${item.slug}.md`), 'utf8');
  const blocks = markdown.split('\n\n').filter(Boolean).slice(1).filter(block => !block.startsWith('[查看'));
  const content = blocks.map(block => {
    if (block.startsWith('![')) {
      const match = block.match(/^!\[([^\]]*)\]\(([^)]+)\)$/);
      if (!match) throw new Error('Invalid example image');
      const asset = match[2].replace('../../web/website/public/', '');
      return `<figure><a href="${local(asset)}" target="_blank" rel="noopener noreferrer"><img src="${local(asset)}" alt="${escape(match[1])}" loading="lazy" decoding="async" style="width:100%;max-height:620px;object-fit:contain"></a></figure>`;
    }
    if (/^\d+\. /.test(block)) return `<ol>${block.split('\n').map(line => `<li style="margin:16px 0">${escape(line.replace(/^\d+\. /,''))}</li>`).join('')}</ol>`;
    return `<p>${escape(block.replace(/^\*|\*$/g,''))}</p>`;
  }).join('');
  const dir = path.join(output, item.slug); mkdirSync(dir, { recursive: true });
  writeFileSync(path.join(dir,'index.html'), layout(item.title,item.description,`${origin}examples/${item.slug}/`,`<main class="detail"><a class="back" href="${local('examples/')}">← 所有示例</a><div class="detail-heading"><p class="eyebrow">BRIEFONCE / WORKFLOW</p><h1>${escape(item.title)}</h1><p class="lead">${escape(item.description)}</p></div><article class="lead" style="max-width:900px">${content}</article><a class="source-link" href="${repository}#本地体验">本地体验与完整源码 ↗</a><a class="next" href="${local('skills/')}">探索可复用的创作 Skill <span>↗</span></a></main>`));
}
const sitemap = path.join(site, 'public/sitemap.xml');
const urls = ['examples/', ...examples.map(item => `examples/${item.slug}/`)];
writeFileSync(sitemap, readFileSync(sitemap,'utf8').replace('</urlset>', `${urls.map(value => `  <url><loc>${origin}${value}</loc></url>`).join('\n')}\n</urlset>`));
console.log(`Generated ${examples.length} source-backed workflow examples.`);
