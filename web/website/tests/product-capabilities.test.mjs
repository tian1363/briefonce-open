import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

test('website introduces the current tool and workflow entrypoints in both languages', async () => {
  const site = await readFile(new URL('../src/App.jsx', import.meta.url), 'utf8');
  const toolbox = await readFile(new URL('../../app/src/EcommerceToolbox.jsx', import.meta.url), 'utf8');
  const scenes = await readFile(new URL('../../app/src/canvasScenes.js', import.meta.url), 'utf8');
  const sourceTools=toolbox.match(/const tools = \[([\s\S]*?)\];/)[1];
  const toolNames=[...sourceTools.matchAll(/id: "([^"]+)", title: "([^"]+)"/g)].filter(match=>match[1]!=='canvas').map(match=>match[2]);
  for (const name of toolNames) assert.ok(site.includes(name), `Website missing tool: ${name}`);
  const titles = [...scenes.matchAll(/title: "([^"]+)"/g)].map(match => match[1]);
  assert.equal(titles.length,3);
  for (const title of titles) assert.ok(site.includes(title), `Website missing workflow: ${title}`);
  assert.match(site,/AI 策划内容后生成新的详情图/);
  assert.match(site,/selected pose produces one image/);
  assert.doesNotMatch(site,/视频字幕换版|在浏览器里排版并导出|换模特与换产品的批量配置/);
});

test('public marketing omits internal operations and removed status paragraphs', async () => {
  const site = await readFile(new URL('../src/App.jsx', import.meta.url), 'utf8');
  const zh = await readFile(new URL('../../../README.md', import.meta.url), 'utf8');
  const en = await readFile(new URL('../../../README.en.md', import.meta.url), 'utf8');
  for (const content of [site, zh, en]) {
    assert.doesNotMatch(content, /只读查看用户|只读用户工作台|只读视图|read-only (?:view|user workspace)|真实报价与出片验收|live output validation/);
  }
  assert.doesNotMatch(site, /toolbox-extra|toolbox-boundary/);
});
