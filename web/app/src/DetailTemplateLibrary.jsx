import { useEffect, useRef, useState } from "react";
import { rankDetailTemplates } from "./detailTemplateMatch.js";
import catalog from "./detailReferenceCatalog.json";
import { uploadCanvasReference } from "./api.js";
import CanvasImagePreview from "./CanvasImagePreview.jsx";
import "./detailTemplateLibrary.css";

export default function DetailTemplateLibrary({ productId, draft = {}, references, selectedId, onSelect, onUploaded, onClose }) {
  const [preview, setPreview] = useState(null);
  const [query, setQuery] = useState("");
  const [platform, setPlatform] = useState("");
  const [category, setCategory] = useState("");
  const [region, setRegion] = useState("");
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [target, setTarget] = useState(null);
  const input = useRef(null);
  const root = useRef(null);
  useEffect(() => { const previous = document.activeElement; return () => previous?.focus(); }, []);
  const ranked = rankDetailTemplates(catalog, draft, references);
  const visible = [...catalog].sort((a,b) => (ranked.find(t => t.id === b.id)?.score || 0) - (ranked.find(t => t.id === a.id)?.score || 0)).filter(t => (!platform || t.platform === platform) && (!category || t.category === category) && (!region || t.region === region)).filter(t => `${t.id} ${t.brand || ""} ${t.platform || ""} ${t.name} ${t.fit}`.toLowerCase().includes(query.trim().toLowerCase()));
  const imageFor = t => references.find(r => r.id === selectedId && r.original_name?.toUpperCase().startsWith(`${t.id}-`)) || [...references].sort((a, b) => (b.created_at || "").localeCompare(a.created_at || "")).find(r => r.original_name?.toUpperCase().startsWith(`${t.id}-`)) || (t.preview_url ? { preview_url:t.preview_url } : null);
  async function upload(file) {
    if (!file || !target) return;
    if (!["image/jpeg", "image/png"].includes(file.type) || file.size > 10 * 1024 * 1024) { setError("请上传 10MB 以内的 JPG 或 PNG 模板图"); return; }
    setBusy(target.id); setError("");
    try {
      const renamed = new File([file], `${target.id}-${target.name}.${file.type === "image/png" ? "png" : "jpg"}`, { type: file.type });
      const asset = await uploadCanvasReference(productId, renamed);
      onUploaded(asset); onSelect(asset.id, target.id, target.name);
    } catch (e) { setError(e.message); }
    finally { setBusy(""); }
  }
  function keydown(event) {
    if (preview) return;
    if (event.key === "Escape" && !busy) onClose();
    if (event.key !== "Tab") return;
    const controls = [...root.current.querySelectorAll('button:not(:disabled), input:not([type="file"]), a[href]')].filter(n => n.getClientRects().length);
    const first = controls[0], last = controls[controls.length - 1];
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
  }
  const imageURL = image => image.id ? `/api/assets/${image.id}/file` : image.preview_url;
  const available = visible.filter(imageFor), missing = visible.filter(t => !imageFor(t));
  const custom = references.filter(r => !catalog.some(t => r.original_name?.toUpperCase().startsWith(`${t.id}-`)));
  async function selectTemplate(t, image) {
    if (image.id) { onSelect(image.id, t.id, t.name); return; }
    setBusy(t.id); setError("");
    try {
      const response = await fetch(image.preview_url);
      if (!response.ok) throw new Error("模板图片读取失败，请重试");
      const blob = await response.blob();
      const ext = blob.type === "image/png" ? "png" : "jpg";
      const asset = await uploadCanvasReference(productId, new File([blob], `${t.id}-${t.name}.${ext}`, { type:blob.type }));
      onUploaded(asset); onSelect(asset.id,t.id,t.name);
    } catch (e) { setError(e.message); } finally { setBusy(""); }
  }
  function card(t) {
    const image = imageFor(t), match = ranked.find(r => r.id === t.id);
    return <article key={t.id} className={image?.id && image.id === selectedId ? "selected" : ""}>
      {image ? <><button type="button" className="detail-template-preview" aria-label={`放大预览 ${t.name}`} onClick={() => setPreview({src:imageURL(image),alt:t.name})}><img src={imageURL(image)} alt={`${t.name}模板参考图`} loading="eager" decoding="async" width="300" height="300" /></button><button type="button" disabled={!!busy} aria-pressed={!!image.id && image.id === selectedId} onClick={() => selectTemplate(t,image)}>{busy === t.id ? "正在准备参考图…" : image.id === selectedId ? "已选为 AI 参考" : "使用此模板图"}</button></> : null}
      <strong>{t.brand} · {t.name}</strong><small>{t.platform || "品牌独立站"} · {t.region}品牌 · {t.category}</small>{match && <p>匹配理由：{match.reasons.join("；")}</p>}<p>适用：{t.fit}</p>
      <details><summary>来源与替换</summary><a href={t.source_preview_url} target="_blank" rel="noreferrer">查看来源商品页</a><button type="button" disabled={!!busy} onClick={() => { setTarget(t); input.current?.click(); }}>{image ? "替换参考图" : "补充模板图片"}</button></details>
    </article>;
  }
  return <div className="detail-template-overlay"><section ref={root} className="detail-template-library" role="dialog" aria-modal="true" aria-label="详情页模板参考图库" onKeyDown={keydown}>
    <header><div><h2>选择详情页参考图</h2><p>{catalog.filter(imageFor).length} 套详情页参考可用 · {custom.length} 张我的参考图</p></div><button type="button" disabled={!!busy} onClick={onClose}>关闭</button></header>
    <p>选择参考图后，图片会实际传给 AI 参考版式、配色与留白；商品外观和卖点仍使用你的商品资料。</p>
    <button type="button" disabled={!!busy} onClick={() => { setTarget({ id:"CUSTOM", name:"自定义详情参考" }); input.current?.click(); }}>上传详情页图片或截图</button>
    <input ref={input} hidden type="file" accept="image/jpeg,image/png" onChange={e => { const file=e.target.files?.[0];e.target.value="";upload(file); }} />
    {error && <p role="alert">{error}</p>}
    {custom.length > 0 && <><h3>我的参考图</h3><div className="detail-template-grid">{custom.map(image => <article key={image.id} className={image.id === selectedId ? "selected" : ""}><button type="button" className="detail-template-preview" aria-label={`放大预览 ${image.original_name}`} onClick={() => setPreview({src:imageURL(image),alt:image.original_name})}><img src={imageURL(image)} alt={image.original_name} loading="eager" decoding="async" width="300" height="300" /></button><strong>{image.original_name}</strong><button type="button" aria-pressed={image.id === selectedId} onClick={() => onSelect(image.id,"","")}>{image.id === selectedId ? "已选为 AI 参考" : "使用此参考图"}</button></article>)}</div></>}
    <h3>按品类与卖点推荐</h3><label>搜索模板<input autoFocus type="search" value={query} onChange={e => setQuery(e.target.value)} placeholder="平台、品牌、风格或商品品类" /></label>
    <div className="detail-template-filters">{[["平台",platform,setPlatform,[...new Set(catalog.map(t => t.platform || "品牌独立站"))]],["品类",category,setCategory,["服装","鞋履"]],["品牌地区",region,setRegion,["国内","海外"]]].map(([label,value,setValue,options]) => <div key={label} role="group" aria-label={label}>{["",...options].map(v => <button type="button" key={v} aria-pressed={value === v} onClick={() => setValue(v)}>{v || `全部${label}`}</button>)}</div>)}</div>
    {available.length > 0 ? <div className="detail-template-grid">{available.map(card)}</div> : <p>{query ? "没有匹配的可用模板，可调整关键词或上传参考图。" : "模板图片仍在补齐，可先使用上方已有参考图，或上传商品详情截图。"}</p>}
    {missing.length > 0 && <details><summary>待补充的模板清单 · {missing.length} 套</summary><p>这些条目尚无可用图片，补充后才能作为 AI 图片参考。</p><div className="detail-template-grid">{missing.map(card)}</div></details>}
  </section>{preview && <CanvasImagePreview images={[preview]} initialIndex={0} onClose={() => setPreview(null)} />}</div>;
}
