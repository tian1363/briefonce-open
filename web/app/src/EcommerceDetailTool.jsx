import DetailTemplateRecommendation from "./DetailTemplateRecommendation.jsx";
import { useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import DetailContentPlanner from "./DetailContentPlanner.jsx";
import DetailTemplateLibrary from "./DetailTemplateLibrary.jsx";
import { getProductMarkdown, listProductAssets } from "./api.js";
import { DETAIL_PLATFORMS, DETAIL_TEMPLATES } from "./detailPageBuilder.js";
import { detailBriefError, detailBriefFields } from "./detailBrief.js";
import { extractProductDetailFacts, mergeProductDetailFacts } from "./productDetailFacts.js";
import CanvasReference from "./CanvasReference.jsx";

function DetailForm({ userId, product, onOpenProducts }) {
  const key = `scriptagent:workflow-detail-draft:${userId}:${product.id}`;
  const [draft, setDraft] = useState(() => {
    let saved = {}; try { saved = JSON.parse(localStorage.getItem(key) || "{}"); } catch {}
    return { title: product.title, claimsText: "", detailText: "", specs: "", usage: "", templateId: "fashion", platformIds: ["taobao"], assetIds: [], referenceId: "", referenceAspects: ["配色", "构图", "留白"], referenceNote: "", ...saved };
  });
  const [assets, setAssets] = useState([]), [references, setReferences] = useState([]);
  const [error, setError] = useState(""), [planner, setPlanner] = useState(false), [templates, setTemplates] = useState(false);
  const update = patch => setDraft(current => ({ ...current, ...patch }));
  useEffect(() => {
    let active = true;
    listProductAssets(product.id).then(items => { if (active) { setAssets(items.filter(a => a.kind === "image")); setReferences(items.filter(a => a.kind === "reference")); } }).catch(e => { if (active) setError(e.message); });
    getProductMarkdown(product.id).then(doc => { if (active) setDraft(current => mergeProductDetailFacts(current, extractProductDetailFacts(doc.content, doc.title))); }).catch(e => { if (active) setError(e.message); });
    return () => { active = false; };
  }, [product.id]);
  useEffect(() => { try { localStorage.setItem(key, JSON.stringify(draft)); } catch {} }, [key, draft]);
  const selectedAssets = draft.assetIds.map(id => assets.find(a => a.id === id)).filter(Boolean);
  function selectAsset(id) {
    if (draft.assetIds.includes(id)) update({ assetIds: draft.assetIds.filter(value => value !== id) });
    else if (draft.assetIds.length < 5) update({ assetIds: [...draft.assetIds, id] });
    else setError("最多选择五张商品图");
  }
  function open() { const problem = detailBriefError(draft, selectedAssets.length); setError(problem); if (!problem) setPlanner(true); }
  return <>
    <div className="tb-extract-grid"><section className="tb-interpret-card"><h2>商品素材与事实</h2><p>按顺序选择 1–5 张商品图，再核对完整资料。缺少的事实可留空，AI 策划时会提示核对。</p>
      <div className="tb-pose-assets" role="group" aria-label="详情页商品图">{assets.map(a => <button type="button" key={a.id} aria-pressed={draft.assetIds.includes(a.id)} className={draft.assetIds.includes(a.id) ? "selected" : ""} onClick={() => selectAsset(a.id)}><img src={`/api/assets/${a.id}/file`} alt={a.original_name} /><span>{a.original_name}</span></button>)}</div>
      <button type="button" className="tb-text-button" onClick={() => onOpenProducts?.(product.id)}>前往产品资料添加图片</button>
      {detailBriefFields.map(([field, label]) => <label className="tb-field" key={field}>{label}<textarea rows={field === "title" ? 1 : 3} value={draft[field]} onChange={e => { update({ [field]: e.target.value, factEdits: { ...draft.factEdits, [field]: true } }); setError(""); }} /><small>{Array.from(draft[field] || "").length} 字 · 每项最多 3000 字</small></label>)}
    </section><section className="tb-interpret-card"><h2>模板参考与平台</h2><DetailTemplateRecommendation draft={draft} references={references} onChange={update} onOpen={() => setTemplates(true)} />
      <button type="button" className="tb-secondary" onClick={() => setTemplates(true)}>选择详情页参考图</button>
      <CanvasReference productId={product.id} references={references} value={{assetId:draft.referenceId,aspects:draft.referenceAspects,note:draft.referenceNote}} onChange={value => update({referenceId:value.assetId,referenceTemplateId:value.assetId === draft.referenceId ? draft.referenceTemplateId : "",referenceTemplateName:value.assetId === draft.referenceId ? draft.referenceTemplateName : "",referenceAspects:value.aspects,referenceNote:value.note})} onUploaded={a => setReferences(current => [a,...current])} onError={setError} aspects={["配色","构图","留白"]} hint="模板图会传给 AI 参考视觉风格，商品外观以商品图为准。" />
      <label className="tb-field">商品类型<select value={draft.templateId} onChange={e => update({templateId:e.target.value})}>{DETAIL_TEMPLATES.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}</select></label>
      <div role="group" aria-label="目标平台">{DETAIL_PLATFORMS.map(p => <label className="tb-field" key={p.id}><span><input type="checkbox" checked={draft.platformIds.includes(p.id)} onChange={() => update({platformIds:draft.platformIds.includes(p.id)?draft.platformIds.filter(id=>id!==p.id):[...draft.platformIds,p.id]})} />{p.name}</span></label>)}</div>
      <p>默认每个平台一次生成一张完整详情图；进入后可核对方案、模型、提示词和费用。</p>
      {error && <p role="alert" className="tb-inline-note">{error}</p>}
      <button type="button" className="tb-primary" onClick={open}>打开 AI 策划与出图</button>
    </section></div>
    {planner && <DetailContentPlanner userId={userId} productId={product.id} draft={draft} assets={selectedAssets} onClose={() => setPlanner(false)} />}
    {templates && <DetailTemplateLibrary productId={product.id} references={references} selectedId={draft.referenceId} onUploaded={a => setReferences(current => [a,...current])} draft={draft} onSelect={(id, templateId, name) => update({referenceId:id,referenceTemplateId:templateId,referenceTemplateName:name,referenceAspects:["版式层级","配色","构图","留白"]})} onClose={() => setTemplates(false)} />}
  </>;
}
export default function EcommerceDetailTool({ userId, products, onBack, onOpenProducts }) {
  const [productId, setProductId] = useState(products[0]?.id || "");
  const product = products.find(p => p.id === productId);
  return <div className="ecommerce-toolbox"><div className="tb-workbench-nav"><button type="button" className="tb-text-button" onClick={onBack}><ArrowLeft size={16} />工具箱</button></div><header className="tb-page-header"><div><h1>AI 商品详情图</h1><p>完整商品资料 → AI 策划 → 整页出图。</p></div></header><label className="tb-field">产品资料<select value={productId} onChange={e => setProductId(e.target.value)}><option value="">请选择产品</option>{products.map(p => <option value={p.id} key={p.id}>{p.title}</option>)}</select></label>{product ? <DetailForm key={`${userId}:${productId}`} userId={userId} product={product} onOpenProducts={onOpenProducts} /> : <p>请先选择产品资料。</p>}</div>;
}
