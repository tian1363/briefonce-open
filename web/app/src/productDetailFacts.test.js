import { test } from 'node:test';
import assert from 'node:assert/strict';
import { extractProductDetailFacts, mergeProductDetailFacts } from './productDetailFacts.js';

test('extracts source sections and tables without mixing audiences or missing facts', () => {
  const facts = extractProductDetailFacts('# 外套\n## 核心卖点\n- 可拆卸帽子\n- 双向拉链\n## 目标用户\n户外爱好者\n## 材质与细节\n面料：聚酯纤维\n## 规格参数\n| 项目 | 数值 |\n| --- | --- |\n| 尺码 | M、L |\n## 洗护\n- 冷水轻柔洗涤\n## 表达边界\n不要宣称防水', '户外外套');
  assert.equal(facts.title, '户外外套');
  assert.equal(facts.claimsText, '可拆卸帽子\n双向拉链');
  assert.equal(facts.detailText, '面料：聚酯纤维');
  assert.match(facts.specs, /尺码：M、L/);
  assert.equal(facts.usage, '冷水轻柔洗涤');
  const empty = extractProductDetailFacts('# 外套\n待补充产品事实。\n## 核心卖点\n## 使用场景\n户外', '外套');
  assert.equal(empty.claimsText + empty.detailText + empty.specs + empty.usage, '');
});

test('refresh updates imported fields and preserves user edits including deliberate clearing', () => {
  const draft = { title: '外套', claimsText: '旧卖点', specs: '用户填写', usage: '', importedFacts: { claimsText: '旧卖点' }, factEdits: { usage: true } };
  const next = mergeProductDetailFacts(draft, { title: '外套', claimsText: '新卖点', detailText: '棉', specs: '自动规格', usage: '冷水洗' });
  assert.equal(next.claimsText, '新卖点');
  assert.equal(next.detailText, '棉');
  assert.equal(next.specs, '用户填写');
  assert.equal(next.usage, '');
});
