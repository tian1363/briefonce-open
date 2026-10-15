import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const website = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

test('publishes source-backed, indexable Skill pages', () => {
  execFileSync('node', ['scripts/generate-public-skills.mjs'], {
    cwd: website,
    env: { ...process.env, SITE_BASE_PATH: '/briefonce-open/' },
  });
  const sitemap = readFileSync(path.join(website, 'public/sitemap.xml'), 'utf8');
  const listing = readFileSync(path.join(website, 'public/skills/index.html'), 'utf8');
  assert.match(listing, /href="\/briefonce-open\/skills\/ugc-hook-writer\/"/);
  assert.match(listing, /href="\/briefonce-open\/skills\/hook-replication\/"/);
  assert.match(listing, /★ 去 GitHub 点亮 Star/);
  assert.match(listing, /href="https:\/\/github.com\/tian1363\/briefonce-open"/);
  assert.doesNotMatch(listing, /fission-strategy|创意裂变策略/);
  assert.doesNotMatch(sitemap, /fission-strategy/);
  assert.equal(existsSync(path.join(website, 'public/skills/fission-strategy/index.html')), false);
  for (const slug of ['hook-replication', 'ugc-hook-writer', 'product-selling-point-writer', 'script-review']) {
    const page = readFileSync(path.join(website, `public/skills/${slug}/index.html`), 'utf8');
    assert.match(page, new RegExp(`rel="canonical" href="https://tian1363.github.io/briefonce-open/skills/${slug}/"`));
    assert.match(page, /完整工作流/);
    assert.match(page, /在 GitHub 给项目一个 Star/);
    assert.match(page, /github.com\/tian1363\/briefonce-open\/blob\/main\/internal\/chat\/service.go/);
    assert.match(sitemap, new RegExp(`<loc>https://tian1363.github.io/briefonce-open/skills/${slug}/</loc>`));
  }
  assert.match(readFileSync(path.join(website, 'public/skills/ugc-hook-writer/index.html'), 'utf8'), /首帧画面/);
  assert.match(readFileSync(path.join(website, 'public/skills/hook-replication/index.html'), 'utf8'), /锚点衔接/);
  assert.match(readFileSync(path.join(website, 'public/skills/product-selling-point-writer/index.html'), 'utf8'), /待核实/);
});

test('workflow examples have static content, working local links and sitemap entries', () => {
  execFileSync('node', ['scripts/generate-public-examples.mjs'], {
    cwd: website,
    env: { ...process.env, SITE_BASE_PATH: '/briefonce-open/' },
  });
  const sitemap = readFileSync(path.join(website, 'public/sitemap.xml'), 'utf8');
  for (const slug of ['apparel-images', 'product-detail', 'video-extraction']) {
    const html = readFileSync(path.join(website, `public/examples/${slug}/index.html`), 'utf8');
    assert.match(html, /<ol><li/);
    assert.match(html, /<h1>/);
    assert.ok(html.includes(`rel="canonical" href="https://tian1363.github.io/briefonce-open/examples/${slug}/"`));
    assert.ok(sitemap.includes(`<loc>https://tian1363.github.io/briefonce-open/examples/${slug}/</loc>`));
    for (const [,asset] of html.matchAll(/src="\/briefonce-open\/([^\"]+)"/g)) {
      assert.ok(existsSync(path.join(website,'public',asset)), `Missing asset ${asset}`);
    }
    assert.doesNotMatch(html, /\/admin|运营后台|供应商对账|利润/);
  }
});
