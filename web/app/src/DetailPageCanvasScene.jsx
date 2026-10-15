import CanvasSelect from "./CanvasSelect.jsx";
import DetailTemplateLibrary from "./DetailTemplateLibrary.jsx";
import DetailContentPlanner from "./DetailContentPlanner.jsx";
import CanvasTextField from "./CanvasTextField.jsx";
import { CanvasPreviewButton } from "./CanvasImagePreview.jsx";
import { useEffect, useMemo, useState } from "react";
import { ImagePlus } from "lucide-react";
import CanvasBoard from "./CanvasBoard.jsx";
import DownloadableImage from "./DownloadableImage.jsx";
import CanvasReference from "./CanvasReference.jsx";
import CanvasToggle from "./CanvasToggle.jsx";
import { getProductMarkdown, listProductAssets, listDetailImages } from "./api.js";
import { DETAIL_PLATFORMS, DETAIL_TEMPLATES } from "./detailPageBuilder.js";

import { exportDetailImages } from "./detailImageExport.js";
import { detailBriefError, detailBriefFields } from "./detailBrief.js";
import { extractProductDetailFacts, mergeProductDetailFacts } from "./productDetailFacts.js";

function readDraft(key, product) {
  try {
    const saved = JSON.parse(localStorage.getItem(key) || "{}");
    return {
      title: typeof saved.title === "string" ? saved.title : product?.title || "",
      claimsText: typeof saved.claimsText === "string" ? saved.claimsText : "",
      detailText: typeof saved.detailText === "string" ? saved.detailText : "",
      specs: typeof saved.specs === "string" ? saved.specs : "",
      usage: typeof saved.usage === "string" ? saved.usage : "",
      templateId: DETAIL_TEMPLATES.some((item) => item.id === saved.templateId) ? saved.templateId : "fashion",
      platformIds: Array.isArray(saved.platformIds) ? saved.platformIds.filter((id) => DETAIL_PLATFORMS.some((item) => item.id === id)) : ["taobao", "jd", "douyin"],
      assetIds: Array.isArray(saved.assetIds) ? saved.assetIds.filter((id) => typeof id === "string").slice(0, 5) : [],
      referenceId: typeof saved.referenceId === "string" ? saved.referenceId : "",
      referenceAspects: Array.isArray(saved.referenceAspects) ? saved.referenceAspects.filter((item) => ["配色", "构图", "留白"].includes(item)) : ["配色"],
      importedFacts: saved.importedFacts || {},
      factEdits: saved.factEdits || {},
      referenceNote: typeof saved.referenceNote === "string" ? saved.referenceNote.slice(0, 160) : "",
    };
  } catch { return { title: product?.title || "", claimsText: "", detailText: "", specs: "", usage: "", templateId: "fashion", platformIds: ["taobao", "jd", "douyin"], assetIds: [], referenceId: "", referenceAspects: ["配色"], referenceNote: "" }; }
}

export default function DetailPageCanvasScene({ userId, productId, product, initialAssetIds = [], onOpenProducts }) {
  const storageKey = `scriptagent:workflow-detail-draft:${userId}:${productId}`;
  const [draft, setDraft] = useState(() => {
    const value = readDraft(storageKey, product);
    if (initialAssetIds.length) value.assetIds = initialAssetIds.slice(0, 5);
    return value;
  });
  const [assets, setAssets] = useState([]);
  const [references, setReferences] = useState([]);
  const [templateLibraryOpen, setTemplateLibraryOpen] = useState(false);
  const [plannerOpen, setPlannerOpen] = useState(false);
  const [jobs, setJobs] = useState([]);
  const [exporting, setExporting] = useState(false);
  const [error, setError] = useState("");
  const [factsStatus, setFactsStatus] = useState("正在读取产品资料…");
  const [factsReload, setFactsReload] = useState(0);
  const update = (patch) => setDraft((current) => ({ ...current, ...patch }));
  const editFact = (field, value) => { setError(""); setDraft((current) => ({ ...current, [field]: value, factEdits: { ...current.factEdits, [field]: true } })); };
  useEffect(() => {
    let active = true;
    setFactsStatus("正在读取产品资料…");
    getProductMarkdown(productId).then((document) => {
      if (!active) return;
      const facts = extractProductDetailFacts(document.content, document.title);
      setDraft((current) => mergeProductDetailFacts(current, facts));
      const count = [facts.claimsText, facts.detailText, facts.specs, facts.usage].filter(Boolean).length;
      setFactsStatus(count ? `已读取产品资料中的 ${count} 类信息，请核对；手动修改已保留。` : "产品资料尚无卖点、材质、规格或使用说明，请补充资料或在此填写。");
    }).catch((err) => { if (active) setFactsStatus(`产品资料读取失败：${err.message}`); });
    return () => { active = false; };
  }, [productId, factsReload]);
  useEffect(() => {
    let active = true;
    listProductAssets(productId).then((items) => { if (active) { setAssets(items.filter((item) => item.kind === "image")); setReferences(items.filter((item) => item.kind === "reference")); } }).catch((err) => { if (active) setError(err.message); });
    return () => { active = false; };
  }, [productId]);
  useEffect(() => { try { localStorage.setItem(storageKey, JSON.stringify(draft)); } catch {} }, [storageKey, draft]);
  useEffect(() => {
    let active = true, timer;
    async function refresh() {
      try { const items = await listDetailImages(productId); if (active) setJobs(items); }
      catch (err) { if (active) setError(`图片任务读取失败：${err.message}`); }
      finally { if (active) timer = setTimeout(refresh, 4000); }
    }
    refresh();
    return () => { active = false; clearTimeout(timer); };
  }, [productId]);
  function toggleAsset(id) {
    if (draft.assetIds.includes(id)) update({ assetIds: draft.assetIds.filter((item) => item !== id) });
    else if (draft.assetIds.length < 5) update({ assetIds: [...draft.assetIds, id] });
    else setError("详情长图最多选择 5 张图片");
  }
  function togglePlatform(id) { update({ platformIds: draft.platformIds.includes(id) ? draft.platformIds.filter((item) => item !== id) : [...draft.platformIds, id] }); }
  function openPlanner() {
    const problem = detailBriefError(draft, selectedAssets.length);
    if (problem) { setError(problem); return; }
    setError(""); setPlannerOpen(true);
  }
  async function downloadResults() {
    setExporting(true); setError("");
    try { await exportDetailImages(readyJobs, draft.title); }
    catch (err) { setError(err.message); }
    finally { setExporting(false); }
  }
  const readyJobs = jobs.filter((job) => job.status === "ready" && job.output_asset_id);
  const selectedAssets = draft.assetIds.map((id) => assets.find((item) => item.id === id)).filter(Boolean);
  const chosenTemplate = DETAIL_TEMPLATES.find((item) => item.id === draft.templateId);
  const node = (id, x, y, data) => ({ id, position: { x, y }, data });
  const nodes = useMemo(() => [
    node("assets", 0, 15, { kind: "source", step: "01", title: "商品素材", content: <>
      <p>从产品素材库选择最多 5 张；可使用刚生成并核对过的白底图和细节图。</p>
      <div className="workflow-detail-assets nowheel nopan nodrag" role="group" aria-label="选择详情页商品图">{assets.map((asset) => <div className="canvas-preview-asset" key={asset.id}><CanvasPreviewButton src={`/api/assets/${asset.id}/file`} label={asset.original_name} /><button type="button" className={`nodrag ${draft.assetIds.includes(asset.id) ? "selected" : ""}`} aria-pressed={draft.assetIds.includes(asset.id)} key={asset.id} onClick={() => toggleAsset(asset.id)}><img src={`/api/assets/${asset.id}/file`} alt={asset.original_name} /><span title={asset.original_name}>{asset.original_name}</span></button></div>)}</div>
      {!assets.length && <button type="button" className="workflow-text-button nodrag" onClick={() => onOpenProducts?.(productId)}>去产品资料上传图片</button>}
      {!!selectedAssets.length && <p>已选 {selectedAssets.length} 张</p>}
    </> }),
    node("facts", 0, 390, { kind: "source", step: "02", title: "已确认商品信息", content: <>
      <p role="status">{factsStatus}</p>
      <p>完整资料用于 AI 策划，再生成整页画面与文案。每项最多 3000 字。</p>
      <button type="button" className="workflow-text-button nodrag" onClick={() => setFactsReload((value) => value + 1)}>重新读取产品资料</button>
      {detailBriefFields.map(([field, label]) => <label className="workflow-field" key={field}>{label}<CanvasTextField multiline rows={field === "title" ? 1 : 3} value={draft[field]} onValueChange={(value) => editFact(field, value)} /><small>{Array.from(draft[field] || "").length} 字</small></label>)}
    </> }),
    node("reference", 360, 15, { kind: "source", step: "02B", title: "页面视觉参考（可选）", content: <>
      <button type="button" className="workflow-text-button nodrag nopan" onClick={() => setTemplateLibraryOpen(true)}><ImagePlus size={16} />选择详情页参考图</button>
      <CanvasReference productId={productId} references={references} value={{ assetId: draft.referenceId, aspects: draft.referenceAspects, note: draft.referenceNote }} onChange={(next) => update({ referenceId: next.assetId, referenceAspects: next.aspects, referenceNote: next.note })} onUploaded={(asset) => setReferences((current) => [...current, asset])} onError={setError} aspects={["配色", "构图", "留白"]} hint="AI 出图参考配色、构图与留白；商品外观以所选商品图为准。" />
    </> }),
    node("template", 700, 160, { kind: "process", step: "03", title: "创作方向与平台", content: <>
      <label className="workflow-field">商品类型<CanvasSelect className="nodrag" value={draft.templateId} onChange={(event) => update({ templateId: event.target.value })}>{DETAIL_TEMPLATES.map((item) => <option value={item.id} key={item.id}>{item.name}</option>)}</CanvasSelect></label>
      <p>{chosenTemplate?.description}。选择模板参考图后，AI 会结合参考图重新设计。</p>
      <div className="workflow-mode-list workflow-platform-list nowheel nopan nodrag" tabIndex={0} role="group" aria-label="导出平台">{DETAIL_PLATFORMS.map((item) => <CanvasToggle key={item.id} checked={draft.platformIds.includes(item.id)} onChange={() => togglePlatform(item.id)}>{item.name}<small>逐屏：{item.variants ? item.variants.map((variant) => `${variant.width}×${variant.height}`).join(" / ") : `${item.width}×${item.height}`}</small></CanvasToggle>)}</div>
      <p>按平台分别调用 AI 出图，默认一次生成完整详情图；上传前请核对商家后台当前类目要求。</p>
    </> }),
    node("render", 1040, 185, { kind: "process", step: "04", title: "AI 策划与出图", content: <>
      <p>先策划整页内容，核对商品事实、构图和费用后，由图片模型一次生成完整详情图；也可选逐屏生成。</p>
      {error && <p role="alert" className="workflow-error">{error}</p>}
      <button type="button" className="workflow-run nodrag" onClick={openPlanner}><ImagePlus size={16} />策划并用 AI 生成详情图</button>
      <p>策划调用文本模型，出图调用所选图片模型；结果自动保存到作品库。</p>
    </> }),
    node("output", 1390, 130, { kind: "result", step: "05", title: "AI 图片与下载", content: <>
      {jobs.some((job) => ["queued", "generating"].includes(job.status)) && <p role="status">图片正在后台生成…</p>}
      {readyJobs.length ? <><p>当前产品已保存 {readyJobs.length} 张 AI 详情图</p><button type="button" className="workflow-text-button nodrag" disabled={exporting} onClick={downloadResults}>{exporting ? "打包中…" : "下载当前产品全部详情图"}</button>{readyJobs.slice(0, 6).map((job) => <div key={job.id}><p>{job.task.platform_name} · {job.task.headline}</p><DownloadableImage className="workflow-preview" src={`/api/assets/${job.output_asset_id}/file`} alt={job.task.headline} /></div>)}</> : <p>生成后在这里查看与下载。历史任务可在 AI 策划窗口和作品库中查看。</p>}
      <p>下载仅适配尺寸与压缩文件，不重绘商品或文字。</p>
    </> }),
  ], [assets, references, draft, factsStatus, selectedAssets, chosenTemplate, jobs, exporting, error, onOpenProducts, productId]);
  const edges = [
    { id: "assets-render", source: "assets", target: "render" },
    { id: "facts-render", source: "facts", target: "render" },
    ...(draft.referenceId ? [{ id: "reference-render", source: "reference", target: "render" }] : []),
    { id: "template-render", source: "template", target: "render" },
    { id: "render-output", source: "render", target: "output" },
  ];
  return <>
    {templateLibraryOpen && <DetailTemplateLibrary productId={productId} references={references} selectedId={draft.referenceId} onUploaded={asset => setReferences(current => [asset, ...current])} draft={draft} onSelect={(id, templateId, name) => update({ referenceId: id, referenceTemplateId:templateId, referenceTemplateName:name, referenceAspects: ["版式层级", "配色", "构图", "留白"] })} onClose={() => setTemplateLibraryOpen(false)} />}
    {plannerOpen && <DetailContentPlanner userId={userId} productId={productId} draft={draft} assets={selectedAssets} onClose={() => setPlannerOpen(false)} />}
    <CanvasBoard userId={userId} productId={productId} sceneId="detail-page" nodes={nodes} edges={edges} galleryItems={readyJobs.map((job) => ({ src: `/api/assets/${job.output_asset_id}/file`, alt: job.task.headline, filename: `${job.task.platform_id}-${job.id}.png` }))} />

  </>;
}
