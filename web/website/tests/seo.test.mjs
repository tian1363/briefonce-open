import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
const site = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const output = path.join(site, 'dist/client');
const origin = 'https://tian1363.github.io/briefonce-open/';

test('language variants have static content, independent canonicals and reciprocal language links', () => {
  for (const [slug,language,text] of [['','zh-CN','一件外套，四个姿势。'],['en/','en','One jacket. Four poses.']]) {
    const html = readFileSync(path.join(output,slug,'index.html'),'utf8');
    assert.ok(html.includes(`<html lang="${language}">`));
    assert.ok(html.includes(`rel="canonical" href="${origin}${slug}"`));
    assert.ok(html.includes(text));
    for (const [lang,target] of [['zh-CN',''],['en','en/'],['x-default','']]) assert.ok(html.includes(`hreflang="${lang}" href="${origin}${target}"`));
    for (const [,src] of html.matchAll(/(?:src|href)="([^\"]+)"/g)) {
      if (src.startsWith('/briefonce-open/assets/')) assert.ok(existsSync(path.join(output,src.slice('/briefonce-open/'.length))),src);
    }
    assert.doesNotMatch(html, /(?:src|href)="assets\//);
    assert.match(html,/max-image-preview:large/);
    assert.doesNotMatch(html, /google-site-verification[^>]+(?:placeholder|undefined)/);
    assert.equal([...html.matchAll(/<h1[ >]/g)].length,1);
  }
});

test('sitemap matches accessible, indexable HTML and real case images', () => {
  const map = readFileSync(path.join(output,'sitemap.xml'),'utf8');
  const urls = [...map.matchAll(/<loc>([^<]+)<\/loc>/g)].map(m=>m[1]);
  assert.ok(urls.includes(origin+'en/'));
  assert.equal(new Set(urls).size,urls.length);
  assert.equal(urls.length,12);
  assert.ok(map.includes('<image:loc>'+origin+'assets/cases/pose-1.png</image:loc>'));
  for (const url of urls) {
    const html=readFileSync(path.join(output,url.slice(origin.length),'index.html'),'utf8');
    assert.ok(html.includes(`rel="canonical" href="${url}"`));
    assert.doesNotMatch(html,/<meta name="robots" content="noindex/);
    for(const [,schema] of html.matchAll(/<script type="application\/ld\+json">([\s\S]*?)<\/script>/g)) assert.doesNotThrow(()=>JSON.parse(schema));
  }
  assert.match(readFileSync(path.join(output,'404.html'),'utf8'),/noindex,follow/);
});
