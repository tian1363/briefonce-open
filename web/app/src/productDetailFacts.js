const fields = ['title', 'claimsText', 'detailText', 'specs', 'usage'];
const clean = (text) => text.replace(/!\[[^\]]*\]\([^)]*\)/g, '').replace(/\[([^\]]+)\]\([^)]*\)/g, '$1').replace(/[*`]/g, '').replace(/^\s*(?:[-+•]|\d+[.)、])\s*/, '').trim();
const missing = /^(?:待补充|待填写|请填写|暂无|未知|未提供|待确认|示例|例如)/;
function category(label) {
  if (/^(?:产品|商品)?名称$|^品名$|^product name$/i.test(label)) return 'title';
  if (/卖点|产品特点|商品特点|核心优势|功能特点|selling points|benefits/i.test(label)) return 'claimsText';
  if (/材质|面料|成分|工艺|做工|细节|material/i.test(label)) return 'detailText';
  if (/规格|参数|尺码|尺寸|容量|净含量|重量|型号|颜色|specifications/i.test(label)) return 'specs';
  if (/使用说明|使用方法|使用步骤|保养|养护|洗护|售后|退换|质保|注意事项|care|warranty/i.test(label)) return 'usage';
  return '';
}

// Extract source text only. Unknown sections and placeholders never become product facts.
export function extractProductDetailFacts(content, title = '') {
  const collected = Object.fromEntries(fields.map((field) => [field, []]));
  let section = '', sectionLevel = 0;
  for (const raw of String(content || '').split(/\r?\n/)) {
    const heading = raw.match(/^\s*(#{1,6})\s+(.+)$/);
    if (heading) {
      const label = clean(heading[2]);
      const next = category(label);
      if (next || heading[1].length <= sectionLevel) { section = next; sectionLevel = heading[1].length; }
      continue;
    }
    const text = clean(raw);
    const labelOnly = text.match(/^([^：:]{1,30})[：:]$/);
    if (labelOnly) { section = category(labelOnly[1]); sectionLevel = 6; continue; }
    if (!text || missing.test(text) || /^[-|:\s]+$/.test(text)) continue;
    const cells = text.startsWith('|') ? text.split('|').map(clean).filter(Boolean) : [];
    if (cells.length && /^(?:项目|字段|属性|参数名称|名称)$/u.test(cells[0]) && /^(?:值|数值|内容|说明|参数值)$/u.test(cells[1] || '')) continue;
    const pair = cells.length >= 2 ? [cells[0], cells.slice(1).join('：')] : text.match(/^([^：:]{1,30})[：:]\s*(.+)$/)?.slice(1);
    const explicit = pair ? category(pair[0]) : '';
    const target = explicit || section;
    if (!target || (pair && missing.test(pair[1]))) continue;
    const value = pair && explicit ? (['specs', 'detailText', 'usage'].includes(target) ? `${pair[0]}：${pair[1]}` : pair[1]) : text;
    if (!collected[target].includes(value)) collected[target].push(value);
  }
  return {
    title: title.trim() || collected.title[0] || '',
    claimsText: collected.claimsText.slice(0, 4).join('\n'),
    detailText: collected.detailText.join('\n'),
    specs: collected.specs.join('\n'),
    usage: collected.usage.join('\n'),
  };
}

export function mergeProductDetailFacts(draft, facts) {
  const next = { ...draft, importedFacts: { ...draft.importedFacts } };
  for (const field of fields) {
    const previous = draft.importedFacts?.[field];
    if (!draft.factEdits?.[field] && (!draft[field]?.trim() || (previous !== undefined && draft[field] === previous))) {
      next[field] = facts[field] || '';
      next.importedFacts[field] = facts[field] || '';
    }
  }
  return next;
}
