// Recommendations use catalog metadata, not an inspection of unavailable images.
const groups = [
  ['鞋','鞋履','运动鞋','跑鞋','越野鞋','靴'], ['羽绒','羽绒服'], ['衬衫'], ['牛仔','单宁'], ['针织','毛衣','开衫'], ['裙','连衣裙'],
  ['短外套','短款外套','短夹克','短款'], ['大衣','长外套','毛呢'],
  ['冲锋衣','户外夹克','功能外套'], ['运动','瑜伽'], ['卫衣'], ['背心'],
];
const styles = [['机能','防风','防水','户外'],['通勤','简约','基础'],['温柔','柔软','奶油'],['复古','法式'],['学院'],['街头'],['垂坠','缎面']];
export function templateImage(template, references = [], selectedId = '') {
  const matches = references.filter(r => r.original_name?.toUpperCase().startsWith(`${template.id}-`));
  return matches.find(r => r.id === selectedId) || matches.sort((a,b) => (b.created_at || '').localeCompare(a.created_at || ''))[0];
}
export function rankDetailTemplates(catalog, draft, references = []) {
  const category = `${draft.title || ''} ${draft.detailText || ''} ${draft.claimsText || ''}`;
  const selling = `${draft.claimsText || ''} ${category}`;
  return catalog.map(t => {
    let score = 0; const reasons = [];
    for (const group of groups) {
      const hit = group.find(k => category.includes(k));
      if (hit && group.some(k => t.fit.includes(k))) { score += 10; reasons.push(`品类：${hit}`); }
    }
    // A generic jacket can match short-jacket layouts without inventing technical performance.
    if (!score && /外套|夹克/.test(category) && /短外套/.test(t.fit)) { score += 3; reasons.push('外套方向，需核对长短款'); }
    for (const group of styles) {
      const hit = group.find(k => selling.includes(k));
      if (hit && group.some(k => `${t.name} ${t.fit}`.includes(k))) { score += 2; reasons.push(`风格或卖点：${hit}`); }
    }
    return {...t, score, reasons, image:templateImage(t,references,draft.referenceId)};
  }).filter(t => t.score > 0 && t.reasons.some(r => r.startsWith('品类') || r.startsWith('外套')))
    .sort((a,b) => b.score-a.score || a.id.localeCompare(b.id));
}
