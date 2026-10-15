import { useEffect } from 'react';
import catalog from './detailReferenceCatalog.json';
import { rankDetailTemplates } from './detailTemplateMatch.js';
export default function DetailTemplateRecommendation({draft,references,onChange,onOpen}) {
  const ranked = rankDetailTemplates(catalog,draft,references);
  const available = ranked.find(t => t.image);
  useEffect(() => {
    if (!draft.referenceId && available) onChange({referenceId:available.image.id,referenceTemplateId:available.id,referenceTemplateName:available.name,referenceAspects:['版式层级','构图','配色','留白']});
  }, [draft.referenceId, available?.id, available?.image?.id]);
  return <section aria-label="按品类与卖点推荐模板"><h3>按品类与卖点匹配模板</h3><p>根据品牌商品页的品类和风格推荐；填写具体品类和卖点可改善匹配。</p>
    {!ranked.length && <p>暂未匹配到服装或鞋履模板，请补充品类或手动选图。</p>}
    {ranked.slice(0,3).map(t => <article key={t.id}><strong>{t.id} · {t.name}</strong><p>{t.reasons.join('；')}。适用：{t.fit}</p>{t.image ? <button type="button" onClick={() => onChange({referenceId:t.image.id,referenceTemplateId:t.id,referenceTemplateName:t.name,referenceAspects:['版式层级','构图','配色','留白']})}>{draft.referenceId === t.image.id ? '已选为图片参考' : '使用这张模板图'}</button> : <button type="button" onClick={onOpen}>查看品牌参考图并选择</button>}</article>)}
    {draft.referenceTemplateId && <p>当前模板：{draft.referenceTemplateId} · {draft.referenceTemplateName}</p>}
    <button type="button" onClick={onOpen}>浏览品牌参考图库</button>
  </section>;
}
