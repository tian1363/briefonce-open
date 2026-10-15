import { analyticsContext, trackEvent } from "./analytics.js";
import CreativeInterpretTool from "./CreativeInterpretTool.jsx";
import ImageModelPicker from "./ImageModelPicker.jsx";
import { useEffect, useRef, useState } from "react";
import DownloadableImage from "./DownloadableImage.jsx";
import { ArrowLeft, ArrowUpRight, Sparkles, Check, ChevronDown, Copy, Download, FileText, Film, FolderOpen, History, ImagePlus, Layers, Loader2, Network, Package, Plus, Save, ScanFace, Trash2, Upload, X } from "lucide-react";
import { listProductAssets, listVideos, interpretCreative, uploadProductAsset, createProduct, createRenderBatch, getRenderBatch, listRenderBatches, retryRenderVariant, transcribeRenderSource, scanRenderCaptions, listAssetExtractions, getAssetExtraction, startAssetExtractionBatch, startUploadedAssetExtractionBatch, importAssetExtraction, acceptAssetExtractionReview } from "./api.js";
import { captionOverlay } from "./captionOverlay.js";
import { toolboxDrafts } from "./toolboxStorage.js";
import "./EcommerceToolbox.css";
import CharacterLibrary from "./CharacterLibrary.jsx";
import WorkflowCanvas from "./WorkflowCanvas.jsx";
import { CANVAS_SCENES } from "./canvasScenes.js";
import ToolboxAI from "./ToolboxAI.jsx";
import { aiCandidatePatch } from "./toolboxAI.js";
import { MAX_VERSIONS, MODE_LABELS, createVersion, initialVersions, versionIssues, batchSummary, restoreBatch, mergeImportedVersions, parseBulkCopies, splitRecognizedSegments, compareVisualCaptions } from "./toolboxBatch.js";

const tools = [
  { id: "canvas", title: "工作流", Icon: Network, description: "在画布中制作姿势图、白底与细节图、商品详情长图。" },
  { id: "extract", title: "视频素材提取", Icon: ImagePlus, description: "上传视频，提取人物、商品、标识和镜头参考。" },
  { id: "interpret", title: "创意解读", Icon: Sparkles, description: "拆解小红书素材的创意表达。" },
];
const assetURL = (asset) => asset?.source_batch_id ? `/api/render-batches/${asset.source_batch_id}/variants/${asset.source_variant_id}/file` : asset?.source_video_id ? `/api/videos/${asset.source_video_id}/file` : asset?.character_id ? `/api/characters/${asset.character_id}/image` : asset?.id ? `/api/assets/${asset.id}/file` : "";
const mediaName = (media) => media?.file?.name || media?.original_name || media?.name || "素材";

function useMediaURL(media) {
  const [url, setURL] = useState("");
  useEffect(() => {
    if (!media?.file) { setURL(assetURL(media)); return; }
    const next = URL.createObjectURL(media.file);
    setURL(next);
    return () => URL.revokeObjectURL(next);
  }, [media]);
  return url;
}

function MediaUpload({ label, kind, value, onChange, onLibrary, onGenerated, onError }) {
  const input = useRef(null);
  const [dragging, setDragging] = useState(false);
  const preview = useMediaURL(value);
  function receive(file) {
    if (!file) return;
    const valid = kind === "video" ? /\.(mp4|mov)$/i.test(file.name) : /\.(png|jpe?g|webp)$/i.test(file.name);
    const limit = kind === "video" ? 100 : 20;
    if (!valid) { onError(kind === "video" ? "请选择 MP4 或 MOV 视频。" : "请选择 JPG、PNG 或 WebP 图片。"); return; }
    if (file.size > limit * 1024 * 1024 || file.size === 0) { onError(`请选择非空且小于 ${limit}MB 的文件。`); return; }
    onError(""); onChange({ file, kind });
  }
  return <div className="tb-upload-wrap">
    <input ref={input} type="file" hidden accept={kind === "video" ? ".mp4,.mov" : ".jpg,.jpeg,.png,.webp"} aria-label={label} onChange={(e) => { receive(e.target.files?.[0]); e.target.value = ""; }} />
    {value ? <div className="tb-selected-media">
      {kind === "image" ? <img src={preview} alt={label} /> : <span className="tb-file-icon"><Film size={22} /></span>}
      <div><strong>{mediaName(value)}</strong><small>{value.file ? "本地素材" : value.source_video_id || value.source_batch_id ? "已生成视频" : "产品资料"}</small></div>
      <button className="tb-icon-button" type="button" aria-label={`移除${label}`} onClick={() => onChange(null)}><X size={16} /></button>
    </div> : <button type="button" className={`tb-dropzone ${dragging ? "dragging" : ""}`} onClick={() => input.current?.click()} onDragOver={(e) => { e.preventDefault(); setDragging(true); }} onDragLeave={() => setDragging(false)} onDrop={(e) => { e.preventDefault(); setDragging(false); receive(e.dataTransfer.files[0]); }}>
      <span className="tb-upload-icon">{kind === "video" ? <Upload size={21} /> : <ImagePlus size={21} />}</span>
      <strong>{kind === "video" ? "点击上传或拖入原视频" : `上传${label}参考图`}</strong>
      <small>{kind === "video" ? "MP4 / MOV · 最大 100MB" : "JPG / PNG / WebP · 最大 20MB"}</small>
    </button>}
    <div className="tb-upload-actions">{value && <button type="button" onClick={() => input.current?.click()}>重新上传</button>}<button type="button" onClick={onLibrary}><FolderOpen size={14} />从产品资料选择</button>{kind === "video" && onGenerated && <button type="button" onClick={onGenerated}><Film size={14} />从已生成视频选择</button>}</div>
  </div>;
}

function GeneratedVideoPicker({ products, productId, onPick, onClose }) {
  const [videos, setVideos] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    Promise.all([listVideos(), listRenderBatches()]).then(([items, batches]) => {
      if (!active) return;
      const generated = (items || []).filter((v) => v.status === "completed").map((v) => ({ source_video_id: v.id, product_id: v.product_id, name: `UGC 成片 · ${new Date(v.created_at).toLocaleString("zh-CN")}`, url: `/api/videos/${v.id}/file`, date: v.created_at }));
      const rendered = (batches || []).flatMap((batch) => (batch.variants || []).filter((v) => v.status === "completed").map((v) => ({ source_batch_id: batch.id, source_variant_id: v.id, product_id: batch.product_id, name: `${batch.title} · ${v.title}`, url: `/api/render-batches/${batch.id}/variants/${v.id}/file`, date: batch.created_at })));
      setVideos([...generated, ...rendered].sort((a, b) => b.date.localeCompare(a.date)));
    }).catch((e) => { if (active) setError(e.message); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, []);
  return <BatchDialog title="从已生成视频选择" onClose={onClose}>{loading ? <p role="status">正在读取成片…</p> : error ? <p role="alert">{error}</p> : videos.length ? <div className="tb-generated-list">{videos.map((v) => <button type="button" key={v.url} onClick={() => onPick(v)}><video src={v.url} preload="metadata" muted /><span>{v.name}<small>{v.product_id && v.product_id !== productId ? `来源产品：${products.find((item) => item.id === v.product_id)?.title || "其他产品"}` : "已完成视频"}</small></span></button>)}</div> : <p className="tb-muted">暂无已完成的视频。可以上传原视频，或先在创意空间生成视频。</p>}</BatchDialog>;
}

function AssetPicker({ products, kind, onPick, onClose }) {
  const dialog = useRef(null);
  const [productId, setProductId] = useState(products[0]?.id || "");
  const [assets, setAssets] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => { dialog.current.showModal(); }, []);
  useEffect(() => {
    let active = true;
    setAssets([]); setError("");
    if (!productId) return;
    setLoading(true);
    listProductAssets(productId).then((items) => { if (active) setAssets(items.filter((item) => item.kind === kind)); })
      .catch((e) => { if (active) setError(e.message); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [productId, kind]);
  return <dialog ref={dialog} className="tb-dialog" onCancel={onClose} onClick={(e) => { if (e.target === dialog.current) onClose(); }} aria-labelledby="tb-library-title">
    <header><div><small>素材库</small><h2 id="tb-library-title">从产品资料选择{kind === "video" ? "视频" : "图片"}</h2></div><button className="tb-icon-button" onClick={onClose} aria-label="关闭素材库"><X size={20} /></button></header>
    {products.length ? <><label className="tb-field">产品<select value={productId} onChange={(e) => setProductId(e.target.value)}>{products.map((p) => <option key={p.id} value={p.id}>{p.title || p.name}</option>)}</select></label>
      {loading ? <p role="status">正在读取素材…</p> : error ? <p role="alert">{error}</p> : assets.length ? <div className="tb-asset-grid">{assets.map((asset) => <button key={asset.id} onClick={() => onPick(asset)}>{kind === "image" ? <img src={assetURL(asset)} alt="" /> : <video src={assetURL(asset)} preload="metadata" muted />}<span>{mediaName(asset)}</span></button>)}</div> : <p className="tb-muted">这个产品还没有可选的{kind === "video" ? "视频" : "图片"}。你也可以关闭窗口，直接上传。</p>}</> : <p className="tb-muted">还没有产品资料。关闭窗口后可直接上传，无需先创建产品。</p>}
  </dialog>;
}

function VersionThumbnail({ row, mode }) {
  const url = useMediaURL(row.target);
  return url && mode !== "copy" ? <img className="tb-row-thumbnail" src={url} alt="目标参考图" /> : <span className="tb-row-thumbnail">{mode === "copy" ? <FileText size={18} /> : <ImagePlus size={18} />}</span>;
}

function BatchDialog({ title, children, onClose }) {
  const ref = useRef(null);
  useEffect(() => { ref.current.showModal(); }, []);
  return <dialog ref={ref} className="tb-dialog" onCancel={onClose} aria-label={title} onClick={(e) => { if (e.target === ref.current) onClose(); }}>
    <header><h2>{title}</h2><button className="tb-icon-button" onClick={onClose} aria-label="关闭窗口"><X size={20} /></button></header>{children}
  </dialog>;
}

const extractionTypes = [
  ["person", "人物角色"], ["product", "商品图"], ["brand_logo", "品牌 Logo"], ["video_watermark", "视频水印"],
  ["scene", "场景"], ["pose", "角色姿势"], ["action", "动作片段"], ["camera", "镜头片段"],
];
const extractionTypeName = Object.fromEntries(extractionTypes);
const extractionFailureMessage = (item) => {
  const message = item?.error || "请重试";
  if (message.includes("生成图与原视频不一致") || message.includes("已阻止导入")) return "生成图与原画面存在差异，已拦截。请用原视频重新提取，或选择更清晰的时间点。";
  return message.length > 180 ? `${message.slice(0, 160)}…` : message;
};

function AssetExtractionTool({ products, onBack, onOpenProducts }) {
  const [imageOptions,setImageOptions]=useState({});
  const [source, setSource] = useState(null);
  const [picker, setPicker] = useState("");
  const [knownProducts, setKnownProducts] = useState(products);
  const [importMode, setImportMode] = useState(products.length ? "existing" : "new");
  const [importProductId, setImportProductId] = useState(products[0]?.id || "");
  const [newProductTitle, setNewProductTitle] = useState("");
  const [importing, setImporting] = useState(false);
  const [importError, setImportError] = useState("");
  const [importNotice, setImportNotice] = useState("");
  const [inspectOpen, setInspectOpen] = useState(false);
  const [reviewing, setReviewing] = useState(false);
  const [reviewConfirmed, setReviewConfirmed] = useState(false);
  const [kinds, setKinds] = useState(["person"]);
  const [label, setLabel] = useState("");
  const [atSeconds, setAtSeconds] = useState(0);
  const [manualAnchor, setManualAnchor] = useState(false);
  const [duration, setDuration] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [items, setItems] = useState([]);
  const [batchIds, setBatchIds] = useState([]);
  const [selected, setSelected] = useState("");
  const [selectedView, setSelectedView] = useState(0);
  const videoRef = useRef(null);
  const sourceURL = useMediaURL(source);
  useEffect(() => { setReviewConfirmed(false); }, [selected]);
  useEffect(() => { setKnownProducts((current) => [...products, ...current.filter((item) => !products.some((next) => next.id === item.id))]); }, [products]);
  useEffect(() => { listAssetExtractions().then(setItems).catch((e) => setError(e.message)); }, []);
  useEffect(() => {
    const pending = items.filter((item) => item.status === "generating");
    if (!pending.length) return;
    const timer = setInterval(async () => {
      const results = await Promise.allSettled(pending.map((item) => getAssetExtraction(item.id)));
      setItems((current) => current.map((item) => {
        const index = pending.findIndex((p) => p.id === item.id);
        return index >= 0 && results[index].status === "fulfilled" ? results[index].value : item;
      }));
    }, 2500);
    return () => clearInterval(timer);
  }, [items]);
  function chooseSource(value) { setSource(value); setAtSeconds(0); setManualAnchor(false); setDuration(0); setError(""); }
  function toggleKind(kind) { setKinds((current) => current.includes(kind) ? current.filter((item) => item !== kind) : [...current, kind]); }
  async function startExtraction(selectedKinds, seconds, title) {
    setError(""); setBusy(true);
    try {
      if (!source) throw new Error("请先选择参考视频");
      if (!selectedKinds.length) throw new Error("请至少选择一种资产类型");
      let input = {image_options:imageOptions, kinds: selectedKinds, label: title.trim(), at_seconds: seconds };
      if (source.source_video_id) input.source_video_id = source.source_video_id;
      else if (source.source_batch_id) { input.source_batch_id = source.source_batch_id; input.source_variant_id = source.source_variant_id; }
      else if (!source.file) input.source_asset_id = source.id;
      const result = source.file ? await startUploadedAssetExtractionBatch(source.file, input) : await startAssetExtractionBatch(input);
      setItems((current) => [...result.items, ...current]); setSelected(result.items[0]?.id || "");
      setBatchIds(result.items.map((item) => item.id));
      if (result.errors?.length) setError(`部分类型提交失败：${result.errors.map((item) => `${extractionTypeName[item.kind] || item.kind}：${item.message}`).join("；")}`);
    } catch (e) { setError(e.message); }
    finally { setBusy(false); }
  }
  function submit(event) {
    event.preventDefault();
    startExtraction(kinds, manualAnchor ? atSeconds : duration / 2, label);
  }
  const active = items.find((item) => item.id === selected) || items[0];
  const activeVariants = active?.variants?.length ? active.variants : ["ready", "needs_review"].includes(active?.status) ? [{ view: "main", at_seconds: active.at_seconds }] : [];
  const activeViewIndex = Math.min(selectedView, Math.max(0, activeVariants.length - 1));
  const viewLabels = { main: "主视角", side: "侧面", back: "背面", detail: "细节" };
  const activeIsClip = ["action", "camera"].includes(active?.kind);
  const activeImageURL = active?.variants?.length ? `/api/asset-extractions/${active.id}/variants/${activeViewIndex}/image` : `/api/asset-extractions/${active?.id}/image`;
  const activeImageName = `${active?.label || "提取资产"}-${active?.kind === "brand_logo" ? "透明Logo.png" : active?.kind === "video_watermark" ? "透明水印.png" : "参考图.jpg"}`;
  const readyItems = items.filter(item => item.status === "ready");
  const importableItems = readyItems.filter(item => !item.product_id);
  const successfulAssets = readyItems.flatMap(item => (item.variants?.length ? item.variants : [{view:"main",at_seconds:item.at_seconds}]).map((variant,index) => ({item,variant,index})));
  async function importResult() {
    if (!importableItems.length || importing) return;
    setImporting(true); setImportError(""); setImportNotice("");
    try {
      let destination = importProductId;
      if (importMode === "new") {
        const title = newProductTitle.trim();
        if (!title) throw new Error("请填写新产品名称");
        const form = new FormData();
        form.append("title", title);
        form.append("product_md", new File([`# ${title}\n\n待补充产品事实。\n`], "product.md", { type: "text/markdown" }));
        const created = await createProduct(form);
        destination = created.id;
        setKnownProducts((current) => [...current, created]);
        setImportProductId(created.id);
        setImportMode("existing");
      }
      if (!destination) throw new Error("请选择目标产品");
      let completed = 0;
      const failures = [];
      for (const item of importableItems) {
        try {
          const updated = await importAssetExtraction(item.id, destination);
          setItems(current => current.map(row => row.id === updated.id ? updated : row));
          completed += item.variants?.length || 1;
          setImportNotice(`已导入 ${completed} 项资产…`);
        } catch (e) { failures.push(`${item.label || extractionTypeName[item.kind]}：${e.message}`); }
      }
      setImportNotice(`已导入 ${completed} 项资产，图片或片段与解析文字已保存到产品资料。`);
      if (failures.length) setImportError(`部分导入失败，可再次点击重试未导入项：${failures.join("；")}`);
    } catch (e) { setImportError(e.message); }
    finally { setImporting(false); }
  }
  async function acceptReview() {
    if (!active || active.status !== "needs_review" || !reviewConfirmed) return;
    setReviewing(true); setImportError("");
    try {
      const updated = await acceptAssetExtractionReview(active.id);
      setItems((current) => current.map((item) => item.id === updated.id ? updated : item));
      setReviewConfirmed(false);
    } catch (e) { setImportError(e.message); }
    finally { setReviewing(false); }
  }
  return <div className="ecommerce-toolbox tb-extract-workspace">
    <div className="tb-workbench-nav"><button className="tb-text-button" onClick={onBack}><ArrowLeft size={16} />工具箱</button></div>
    <header className="tb-page-header"><div><h1>视频素材提取</h1><p>筛选关键帧，提取人物、商品、标识与镜头参考。</p></div></header>
    <div className="tb-extract-grid"><form className="tb-interpret-card" onSubmit={submit}>
      <h2>参考视频与画面</h2>
      <fieldset className="tb-extract-types"><legend>资产类型 <span>可多选 · 已选 {kinds.length} 种</span></legend><div className="tb-extract-type-tabs">{extractionTypes.map(([id, name]) => <button key={id} type="button" aria-pressed={kinds.includes(id)} onClick={() => toggleKind(id)}>{name}</button>)}</div></fieldset>
      <MediaUpload label="参考视频" kind="video" value={source} onChange={chooseSource} onLibrary={() => setPicker("library")} onGenerated={() => setPicker("generated")} onError={setError} />
      {sourceURL && <div className="tb-source-video"><video key={sourceURL} ref={videoRef} src={sourceURL} controls preload="metadata" onLoadedMetadata={(e) => setDuration(e.currentTarget.duration)} onTimeUpdate={(e) => { if (manualAnchor) setAtSeconds(e.currentTarget.currentTime); }} onError={() => setError("视频无法预览，请换一条 MP4 视频")} /></div>}
      {source && <div className="tb-field"><label><input type="checkbox" checked={manualAnchor} onChange={(e) => { setManualAnchor(e.target.checked); if (e.target.checked) setAtSeconds(videoRef.current?.currentTime || duration / 2); }} /> 指定优先参考位置（可选）</label>{manualAnchor && <label>优先参考位置：{atSeconds.toFixed(1)} 秒<input type="range" min="0" max={Math.max(0, duration - 0.1)} step="0.1" value={Math.min(atSeconds, Math.max(0, duration - 0.1))} disabled={!duration} onChange={(e) => { const value = Number(e.target.value); setAtSeconds(value); if (videoRef.current) videoRef.current.currentTime = value; }} /></label>}</div>}
      <label className="tb-field">资产名称（选填）<input maxLength={80} value={label} onChange={(e) => setLabel(e.target.value)} placeholder="例如：秋冬外套参考视频" /></label>
      {error && <p role="alert" className="tb-inline-note">{error}</p>}
<ImageModelPicker value={imageOptions} onChange={setImageOptions} count={kinds.filter(k=>["person","product","scene","brand_logo","video_watermark"].includes(k)).length} disabled={busy}/><p className="tb-muted">提取可能包含多视角出图、重试与多模态质检，每次实际调用单独计费。</p>
      <button className="tb-primary" disabled={busy || !source || !duration || !kinds.length}>{busy ? <Loader2 size={16} className="spin" /> : <ImagePlus size={16} />}{busy ? "正在提交…" : `提取 ${kinds.length} 种资产并解析`}</button>
      <p className="tb-helper">时间点用作优先搜索位置。商品仅在原视频确实出现相应视角时生成侧面、背面或细节图。品牌 Logo 从原帧可确认的实体标识中分离为透明 PNG，并检查是否与原标识一致；视频水印也从确认的屏幕叠加标识中分离为透明 PNG，核对原帧，不补造模糊文字。场景图会清除人物、商品及画面文字，只保留背景；遮挡区域由模型补全。姿势保留参考帧，动作、镜头保存短片。自动重试可能增加图片模型调用；请人工核对结果。</p>
    </form><section className="tb-interpret-card"><h2>提取结果</h2>
      <p className="tb-muted">已成功解析 {successfulAssets.length} 项资产，全部展示在下方。已导入 {readyItems.filter(item=>item.product_id).length} 条提取记录。</p>
      {readyItems.length > 0 && <><div className="tb-extract-import"><h3>导入产品资料</h3><p className="tb-muted">所有成功资产的图片、全部视角、片段与解析文字一起导入；自动跳过已导入记录。</p><div className="tb-upload-actions"><button type="button" aria-pressed={importMode === "existing"} disabled={!knownProducts.length || importing} onClick={() => setImportMode("existing")}>导入已有产品</button><button type="button" aria-pressed={importMode === "new"} disabled={importing} onClick={() => setImportMode("new")}>新建产品并导入</button></div>{importMode === "existing" ? <label className="tb-field">目标产品<select disabled={importing} value={importProductId} onChange={(e) => setImportProductId(e.target.value)}><option value="">选择产品</option>{knownProducts.map((product) => <option key={product.id} value={product.id}>{product.title}</option>)}</select></label> : <label className="tb-field">新产品名称<input disabled={importing} maxLength={80} value={newProductTitle} onChange={(e) => setNewProductTitle(e.target.value)} placeholder="例如：新款背包" /></label>}{importNotice && <p role="status" className="tb-inline-note">{importNotice}</p>}{importError && <p role="alert" className="tb-inline-note">{importError}</p>}<button type="button" className="tb-primary" disabled={importing || !importableItems.length || (importMode === "existing" ? !importProductId : !newProductTitle.trim())} onClick={importResult}>{importing ? <Loader2 size={16} className="spin" /> : <FolderOpen size={16} />}{importing ? "正在导入…" : `一键全部导入产品资料 · ${importableItems.reduce((count,item) => count + (item.variants?.length || 1),0)} 项`}</button></div><div className="tb-extracted-assets" aria-label="全部成功资产">{successfulAssets.map(({item,variant,index}) => {
        const base = `/api/asset-extractions/${item.id}`;
        const imageURL = item.variants?.length ? `${base}/variants/${index}/image` : `${base}/image`;
        const title = `${item.label || extractionTypeName[item.kind]} · ${viewLabels[variant.view] || "主视角"}`;
        return <article key={`${item.id}-${index}`}><strong>{title}</strong><small>{extractionTypeName[item.kind]} · {Number(variant.at_seconds || 0).toFixed(1)} 秒</small>{["action","camera"].includes(item.kind) && item.variants?.length ? <video controls preload="metadata" poster={imageURL} src={`${base}/variants/${index}/clip`} /> : <DownloadableImage src={imageURL} alt={title} filename={`${title}.png`} />}<p className="tb-helper">{item.product_id ? `已导入：${knownProducts.find(p=>p.id===item.product_id)?.title || "产品资料"}` : "可导入产品资料"}</p><button type="button" className="tb-text-button" onClick={()=>{setSelected(item.id);setSelectedView(index);setInspectOpen(true);}}>查看原画面与解析</button></article>;
      })}</div></>}
      {!readyItems.length && <p className="tb-muted">成功解析的资产会自动集中显示在这里。</p>}
      <details className="tb-extraction-inspector" open={inspectOpen} onToggle={e=>setInspectOpen(e.currentTarget.open)}><summary>任务记录、原画面对照与待核对结果</summary>

      {items.length > 0 && <label className="tb-field">历史任务<select value={active?.id || ""} onChange={(e) => setSelected(e.target.value)}>{items.map((item) => <option key={item.id} value={item.id}>{extractionTypeName[item.kind] || item.kind} · {item.label} · {item.status === "needs_review" ? "待核对 · " : item.status === "failed" ? "失败 · " : ""}{new Date(item.created_at).toLocaleString("zh-CN")}</option>)}</select></label>}
      {batchIds.length > 1 && <div className="tb-extract-result-tabs" aria-label="本次提取的资产">{batchIds.map((id) => { const item = items.find((entry) => entry.id === id); return item && <button key={id} type="button" aria-pressed={active?.id === id} onClick={() => { setSelected(id); setSelectedView(0); }}>{extractionTypeName[item.kind] || item.kind}<small>{item.status === "ready" ? "已完成" : item.status === "needs_review" ? "待核对" : item.status === "failed" ? "失败" : "处理中"}</small></button>; })}</div>}
      {active ? <><p className="tb-muted">{active.status === "ready" ? `提取完成，共 ${activeVariants.length} 项；全部成功资产已展示在上方` : active.status === "needs_review" ? `已保留生成图，待人工核对：${active.error || "自动质检未通过"}` : active.status === "generating" ? "正在筛选关键帧并整理资产…" : `处理失败：${extractionFailureMessage(active)}`}</p>{["ready", "needs_review"].includes(active.status) && activeVariants.length > 1 && <div className="tb-detail-preview-tabs" role="tablist" aria-label="提取视角">{activeVariants.map((variant, index) => <button type="button" role="tab" aria-selected={activeViewIndex === index} key={`${variant.view}-${index}`} onClick={() => setSelectedView(index)}>{viewLabels[variant.view] || "视角"} · {variant.at_seconds.toFixed(1)} 秒</button>)}</div>}<div className="tb-extract-comparison"><figure><DownloadableImage src={["ready", "needs_review"].includes(active.status) && active.variants?.length ? `/api/asset-extractions/${active.id}/variants/${activeViewIndex}/frame` : `/api/asset-extractions/${active.id}/frame`} alt="参考视频关键帧" filename={`${active.label}-参考画面.jpg`} /><figcaption>参考画面</figcaption></figure>{["ready", "needs_review"].includes(active.status) && <figure>{["action", "camera"].includes(active.kind) && active.variants?.length ? <video controls preload="metadata" poster={`/api/asset-extractions/${active.id}/variants/${activeViewIndex}/image`} src={`/api/asset-extractions/${active.id}/variants/${activeViewIndex}/clip`} /> : <DownloadableImage src={activeImageURL} alt="提取结果" filename={activeImageName} />}<figcaption>{["action", "camera"].includes(active.kind) ? "动态参考片段" : ["scene", "pose"].includes(active.kind) ? "参考图" : active.kind === "brand_logo" ? "透明品牌 Logo" : active.kind === "video_watermark" ? "透明视频水印" : active.status === "needs_review" ? "待核对生成图" : `${viewLabels[activeVariants[activeViewIndex]?.view] || "生成"}白底图`}</figcaption></figure>}</div>{active.status === "needs_review" && <div className="tb-extract-review"><strong>请对照两张图确认</strong><p>自动质检发现可能新增的标识或外观差异。生成图已保留，但确认前不会进入角色库或产品素材库。</p><details><summary>查看质检说明</summary><pre className="tb-interpret-output">{active.analysis}</pre></details><label><input type="checkbox" checked={reviewConfirmed} onChange={(e) => setReviewConfirmed(e.target.checked)} />我已核对人物、商品和画面文字，确认使用这张图</label><div className="tb-upload-actions"><button type="button" className="tb-primary" disabled={!reviewConfirmed || reviewing} onClick={acceptReview}>{reviewing ? "正在确认…" : "确认使用"}</button><button type="button" disabled={busy || !source || !duration} onClick={() => startExtraction([active.kind], Math.min(active.at_seconds, Math.max(0, duration - 0.1)), active.label)}>用左侧视频重新提取</button></div>{importError && <p role="alert" className="tb-inline-note">{importError}</p>}</div>}{active.status === "failed" && <div className="tb-extract-review"><p>这次没有生成可核对的图片。可调整左侧视频或时间点，再试一次当前类型。</p>{active.error && active.error !== extractionFailureMessage(active) && <details><summary>查看完整失败原因</summary><p>{active.error}</p></details>}<button type="button" disabled={busy || !source || !duration} onClick={() => startExtraction([active.kind], manualAnchor ? atSeconds : duration / 2, active.label)}>用左侧视频重试当前类型</button></div>}{active.status === "ready" && <><p className="tb-helper">{active.kind === "person" && active.target_id ? "主视角人物图已保存到角色库。" : "资产已保存在提取记录中。"}{activeIsClip && <a href={`/api/asset-extractions/${active.id}/variants/${activeViewIndex}/clip`} download={`${active.label}-参考片段.mp4`}>下载当前片段</a>}</p><pre className="tb-interpret-output">{active.analysis}</pre>{active.product_id ? <p className="tb-inline-note">已导入「{knownProducts.find((item) => item.id === active.product_id)?.title || "产品资料"}」的素材库和文档。{onOpenProducts && <button type="button" className="tb-text-button" onClick={() => onOpenProducts(active.product_id)}>查看产品资料</button>}</p> : <p className="tb-muted">可使用上方按钮将全部成功资产一起导入。</p>}</>}</> : <p className="tb-muted">提取结果会显示在这里。完成后可导入已有或新建产品。</p>}
      </details>
    </section></div>
    {picker === "library" && <AssetPicker products={knownProducts} kind="video" onClose={() => setPicker("")} onPick={(asset) => { chooseSource(asset); setPicker(""); }} />}
    {picker === "generated" && <GeneratedVideoPicker products={knownProducts} productId="" onClose={() => setPicker("")} onPick={(video) => { chooseSource(video); setPicker(""); }} />}
  </div>;
}

export function ToolboxLauncher({ onChoose }) {
  function card(item,scene) {
    const Icon = item.Icon || ({pose:ScanFace,"product-photo":ImagePlus,"detail-page":Layers}[item.id]);
    return <button key={item.id} className="tb-tool-choice" onClick={()=>onChoose(scene ? "canvas" : item.id,scene ? item.id : undefined)} aria-label={`${item.title}，${scene ? "进入工作流" : "进入操作台"}`}><span className="tb-tool-icon"><Icon size={23} strokeWidth={1.7}/></span><span className="tb-tool-copy"><strong>{item.title}</strong><small>{item.description}</small></span><ArrowUpRight className="tb-tool-arrow" size={18}/></button>;
  }
  return <div className="tb-launcher"><div className="tb-launcher-heading"><span>电商工具箱</span><h1>从素材到成品，选择创作方式</h1></div>
    <section className="tb-launcher-section" aria-labelledby="tb-tools-heading"><header><h2 id="tb-tools-heading">工具</h2><p>提取视频素材，解读创意表达。</p></header><nav className="tb-tool-choices" aria-label="工具">{tools.filter(item=>item.id!=="canvas").map(item=>card(item,false))}</nav></section>
    <section className="tb-launcher-section" aria-labelledby="tb-canvas-heading"><header><h2 id="tb-canvas-heading">工作流</h2><p>选择场景，直接进入对应工作流。</p></header><nav className="tb-tool-choices tb-canvas-choices" aria-label="工作流场景">{CANVAS_SCENES.map(item=>card(item,true))}</nav></section>
  </div>;
}

export default function EcommerceToolbox({ products = [], userId, active = true, entry, onOpenRadar, onOpenProducts }) {
  const [entered, setEntered] = useState(false);
  const [renderProductId, setRenderProductId] = useState(products[0]?.id || "");
  const [renderBatch, setRenderBatch] = useState(null);
  const [serverBatches, setServerBatches] = useState([]);
  const [renderBusy, setRenderBusy] = useState(false);
  const [transcriptionBusy, setTranscriptionBusy] = useState(false);
  const [scanBusy, setScanBusy] = useState(false);
  useEffect(() => { if (!renderProductId && products[0]?.id) setRenderProductId(products[0].id); }, [products, renderProductId]);
  const [ai, setAI] = useState(null);
  const [characters, setCharacters] = useState(null);
  const settingsRef = useRef(null);
  const [canvasStartScene,setCanvasStartScene] = useState(undefined);
  const [mode, setMode] = useState(() => {
    try { const saved = localStorage.getItem(`scriptagent:toolbox-mode:${userId}`); return MODE_LABELS[saved] && tools.some((item) => item.id === saved) ? saved : "interpret"; } catch { return "interpret"; }
  });
  useEffect(() => {
    if (!entry) return;
    if (entry.mode && tools.some((item) => item.id === entry.mode)) {
      setCanvasStartScene(entry.scene);
      setMode(entry.mode);
      setActiveId(null);
      setEntered(true);
    } else {
      setEntered(false);
    }
    setError("");
  }, [entry]);
  const [source, setSource] = useState(null);
  const [versionsByMode, setVersions] = useState(initialVersions);
  const [activeId, setActiveId] = useState(null);
  const [batchName, setBatchName] = useState("");
  const [resolution, setResolution] = useState("720P");
  const [duration, setDuration] = useState(0);
  const [mediaError, setMediaError] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [drafts, setDrafts] = useState([]);
  const [draftId, setDraftId] = useState(null);
  const [saving, setSaving] = useState(false);
  const [dialog, setDialog] = useState(null);
  const [picker, setPicker] = useState(null);
  const [bulkCopy, setBulkCopy] = useState("");
  const videoRef = useRef(null);
  const importRef = useRef(null);
  const sourceURL = useMediaURL(source);
  const rows = versionsByMode[mode] || [];
  const current = rows.find((row) => row.id === activeId) || rows[0];
  const currentIndex = rows.findIndex((row) => row.id === current?.id);
  const summary = batchSummary(mode, rows);
  const tool = tools.find((item) => item.id === mode);
  const canReview = Boolean(source && !mediaError && summary.selected.length > 0);
  const ready = canReview && !summary.incomplete && !summary.duplicateIds.size;
  const selectedAll = rows.length > 0 && rows.every((row) => row.selected);

  useEffect(() => { if (!active || !entered) videoRef.current?.pause(); }, [active, entered]);
  useEffect(() => {
    let mounted = true;
    listRenderBatches().then((items) => { if (mounted) { setServerBatches(items || []); if (items?.length) setRenderBatch(items[0]); } }).catch(() => {});
    return () => { mounted = false; };
  }, [userId]);
  useEffect(() => {
    if (!renderBatch?.id || (!renderBatch.variants?.some((variant) => variant.status === "pending" || variant.status === "running") && renderBatch.status !== "running")) return;
    const timer = setInterval(() => getRenderBatch(renderBatch.id).then(setRenderBatch).catch(() => {}), 3000);
    return () => clearInterval(timer);
  }, [renderBatch?.id, renderBatch?.status]);
  useEffect(() => { if (!active) { setEntered(false); setAI(null); setCharacters(null); setDialog(null); setPicker(null); } }, [active]);
  useEffect(() => { try { localStorage.setItem(`scriptagent:toolbox-mode:${userId}`, mode); } catch {} }, [mode, userId]);
  useEffect(() => {
    let mounted = true;
    toolboxDrafts(userId, "list").then((items) => { if (mounted) setDrafts(items); }).catch(() => { if (mounted) setError("无法读取本地批次，请检查浏览器存储权限。"); });
    return () => { mounted = false; };
  }, [userId]);

  function updateRows(fn) { setVersions((all) => ({ ...all, [mode]: fn(all[mode]) })); setNotice(""); }
  function updateRow(id, patch) { updateRows((items) => items.map((row) => row.id === id ? { ...row, ...patch } : row)); }
  function changeSource(value) { setSource(value); setDuration(0); setMediaError(""); setNotice(value && versionsByMode.copy.some((row) => row.segments?.length) ? "已更换原视频，请重新核对各版本的字幕时间点。" : ""); }
  async function sourceAnalysisInput() {
    let selected = source;
    if (source.file) { selected = await uploadProductAsset(renderProductId, source.file); setSource(selected); }
    if (selected.id && selected.product_id !== renderProductId) throw new Error("原视频与所选产品不一致。");
    return { product_id: renderProductId, source_asset_id: selected.id || "", source_video_id: selected.source_video_id || "", source_batch_id: selected.source_batch_id || "", source_variant_id: selected.source_variant_id || "" };
  }
  async function recognizeSpeech() {
    if (!source || !renderProductId || transcriptionBusy) return;
    setTranscriptionBusy(true); setError("");
    try {
      const result = await transcribeRenderSource(await sourceAnalysisInput());
      const recognized = splitRecognizedSegments(result.segments).slice(0, 20);
      setVersions((all) => ({ ...all, copy: all.copy.map((row) => {
        if (row.segments?.length && row.segments.every((segment) => segment.source === "visual")) return { ...row, segments: compareVisualCaptions(recognized, row.segments) };
        return row.segments?.length ? row : { ...row, segments: recognized.map((segment) => ({ ...segment })) };
      }) }));
      setNotice(`已识别 ${recognized.length} 句语音；请逐句核对时间和文案${result.segments.length > 20 ? "（只取前 20 句）" : ""}。`);
    } catch (err) { setError(err.message); }
    finally { setTranscriptionBusy(false); }
  }
  async function scanCaptions() {
    if (!source || !renderProductId || scanBusy) return;
    setScanBusy(true); setError("");
    try {
      const result = await scanRenderCaptions(await sourceAnalysisInput());
      const visual = splitRecognizedSegments(result.segments).slice(0, 20);
      setVersions((all) => ({ ...all, copy: all.copy.map((row) => ({ ...row, segments: compareVisualCaptions(row.segments, visual) })) }));
      setNotice(`已扫描 ${visual.length} 段底部画面文字；请核对位置、内容及疑似不一致的句子。`);
    } catch (err) { setError(err.message); }
    finally { setScanBusy(false); }
  }
  function updateSegment(row, index, patch) { updateRow(row.id, { segments: row.segments.map((segment, i) => i === index ? { ...segment, ...patch } : segment) }); }
  function addSegment(row) {
    const last = row.segments?.at(-1);
    const start = last ? Number(last.end) : Math.max(0, Math.min(videoRef.current?.currentTime || 0, (duration || 30) - 1));
    const end = Math.min(duration || 30, start + 3);
    if (end <= start) { setError("视频末尾没有可用时间，请先调整上一句的结束时间。"); return; }
    updateRow(row.id, { segments: [...(row.segments || []), { start, end, text: row.segments?.length ? "" : row.text, needs_review: false }], text: "" });
  }
  function addVersion(base = {}) {
    if (rows.length >= MAX_VERSIONS) return;
    const row = createVersion(base); updateRows((items) => [...items, row]); setActiveId(row.id);
  }
  function removeVersion(id) {
    const index = rows.findIndex((row) => row.id === id);
    const next = rows.filter((row) => row.id !== id);
    if (current?.id === id) setActiveId(next[Math.min(index, next.length - 1)]?.id || null);
    updateRows(() => next);
    setDialog(null);
  }
  function addImported(imported) {
    let next;
    try { next = mergeImportedVersions(rows, imported); }
    catch (e) { setError(e.message); return false; }
    updateRows(() => next);
    setActiveId(next[0]?.id);
    setNotice(`已导入 ${imported.length} 个差异化版本，请检查并确认要纳入的版本。`);
    return true;
  }
  function importImages(files) {
    const items = Array.from(files || []);
    if (!items.length) return;
    if (items.some((file) => !/\.(jpe?g|png|webp)$/i.test(file.name) || file.size === 0 || file.size > 20 * 1024 * 1024)) { setError("请上传 JPG、PNG 或 WebP 图片，每张非空且不超过 20MB。此次未导入任何图片。"); return; }
    addImported(items.map((file) => ({ target: { file, kind: "image" }, title: file.name.replace(/\.[^.]+$/, "") })));
  }
  async function save() {
    if (saving) return;
    setSaving(true); setError("");
    const draft = { id: draftId || crypto.randomUUID(), schemaVersion: 2, mode, source, batchName, versionsByMode, resolution, updatedAt: Date.now() };
    try { await toolboxDrafts(userId, "save", draft); setDraftId(draft.id); setDrafts(await toolboxDrafts(userId, "list")); setNotice("整批配置与素材已保存到当前浏览器。"); return true; }
    catch { setError("保存失败，请检查浏览器存储空间。当前输入仍保留。"); return false; }
    finally { setSaving(false); }
  }
  function restore(draft) {
    setMode(draft.mode); setVersions(restoreBatch(draft)); changeSource(draft.source); setBatchName(draft.batchName || "");
    setResolution(draft.resolution || "720P"); setDraftId(draft.id); setActiveId(null); setDialog(null); setError("");
  }
  function download() {
    const text = [`# ${batchName || `${tool.title}批量素材`}`, `原视频：${mediaName(source)}\n\n规格：${resolution}\n\n选中版本：${summary.selected.length}`, ...summary.selected.map((row, i) => `## ${i + 1}. ${row.title || `版本 ${i + 1}`}\n\n${mode === "copy" ? `修改范围：仅字幕\n\n${row.segments?.length ? row.segments.map((segment) => `${segment.start}–${segment.end} 秒：${segment.text}`).join("\n") : row.text}` : `参考图：${row.target ? mediaName(row.target) : "未选择"}\n\n${mode === "model" ? row.notes : `产品：${row.name}\n卖点：${row.claims}\n文案：${row.adapt ? row.text : "沿用原文案，需核对产品信息"}`}`}\n\n检查：${versionIssues(mode, row).join("、") || "输入已齐全"}${summary.duplicateIds.has(row.id) ? "；与其他选中版本重复" : ""}`), "状态：配置草稿，尚未提交生成。\n参考原片生成新版本，其他画面不保证逐帧一致。"].join("\n\n");
    const url = URL.createObjectURL(new Blob([text], { type: "text/markdown;charset=utf-8" }));
    const a = document.createElement("a"); a.href = url; a.download = "批量素材方案.md"; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  async function renderCaptions() {
    if (renderBusy || !ready || mode !== "copy") return;
    if (!renderProductId) { setError("请先选择产品资料。"); return; }
    if (summary.selected.length > 5) { setError("首版每批最多制作 5 条字幕成片，请取消多余版本。"); return; }
    setRenderBusy(true); setError("");
    try {
      let sourceID = source?.id;
      if (sourceID && source.product_id !== renderProductId) throw new Error("原视频与所选产品不一致。");
      if (!sourceID && source?.file) { const uploaded = await uploadProductAsset(renderProductId, source.file); sourceID = uploaded.id; setSource(uploaded); }
      if (!sourceID && !source?.source_video_id && !source?.source_batch_id) throw new Error("请先选择原视频。");
      const form = new FormData();
      form.append("product_id", renderProductId);
      if (sourceID) form.append("source_asset_id", sourceID);
      if (source?.source_video_id) form.append("source_video_id", source.source_video_id);
      if (source?.source_batch_id) { form.append("source_batch_id", source.source_batch_id); form.append("source_variant_id", source.source_variant_id); }
      form.append("title", batchName || "广告字幕批次");
      form.append("variants", JSON.stringify(summary.selected.map((row, index) => ({ title: row.title || `版本 ${index + 1}`, copy: row.text, segments: row.segments || [], cover_existing: !!row.coverExisting }))));
      for (let i = 0; i < summary.selected.length; i++) {
        const row = summary.selected[i];
        if (row.segments?.length) {
          for (let j = 0; j < row.segments.length; j++) {
            const blob = await captionOverlay(row.segments[j].text, row.coverExisting);
            form.append(`overlay_${i}_${j}`, blob, `caption-${i}-${j}.png`);
          }
        } else {
          const blob = await captionOverlay(row.text);
          form.append(`overlay_${i}`, blob, `caption-${i}.png`);
        }
      }
      const created = await createRenderBatch(form);
      setRenderBatch(created);
      setServerBatches((items) => [created, ...items]);
      setDialog(null);
      setNotice("批次已提交，正在逐条合成成片。");
    } catch (err) { setError(err.message); }
    finally { setRenderBusy(false); }
  }

  function applyAI(action, candidates, append) {
    if (!ai || ai.mode !== mode || !rows.some((row) => row.id === ai.row.id)) { setAI(null); return; }
    const patches = candidates.map((candidate) => aiCandidatePatch(mode, action, candidate));
    if (mode === "copy" && Number.isInteger(ai.segmentIndex)) {
      if (append) {
        if (rows.length + candidates.length > MAX_VERSIONS) { setError("本批次最多 30 个版本。"); return; }
        const additions = candidates.map((candidate, index) => createVersion({ ...ai.row, id: crypto.randomUUID(), selected: true, title: candidate.title, segments: ai.row.segments.map((segment, i) => i === ai.segmentIndex ? { ...segment, text: patches[index].text, needs_review: false } : { ...segment }) }));
        updateRows((items) => [...items, ...additions]); setActiveId(additions[0].id);
      } else {
        updateRow(ai.row.id, { segments: ai.row.segments.map((segment, i) => i === ai.segmentIndex ? { ...segment, text: patches[0].text, needs_review: false } : segment) });
      }
      setAI(null); setNotice("已采用 AI 文案，请核对商品事实、句长和时间点。"); return;
    }
    if (append) {
      if (rows.length + candidates.length > MAX_VERSIONS) { setError("本批次最多 30 个版本。"); return; }
      const additions = candidates.map((candidate, index) => createVersion({ ...ai.row, ...patches[index], id: crypto.randomUUID(), selected: true, title: candidate.title }));
      updateRows((items) => [...items, ...additions]); setActiveId(additions[0].id);
    } else {
      updateRow(ai.row.id, patches[0]);
      if (action === "direction" || action === "claims") { if (settingsRef.current) settingsRef.current.open = true; }
    }
    setAI(null); setNotice(append ? `已添加 ${candidates.length} 个 AI 版本，原版本已保留。` : "已采用 AI 建议，请检查内容后保存草稿。");
  }

  useEffect(() => { if (active && entered && mode !== "canvas") { analyticsContext("toolbox"); trackEvent("feature_open", mode === "interpret" ? "interpret" : "extract", "toolbox"); } }, [active, entered, mode]);
  function chooseTool(id,scene) { setCanvasStartScene(scene);setMode(id);setActiveId(null);setEntered(true);setError(""); }
  if (!entered) return <ToolboxLauncher onChoose={chooseTool} />;
  if (mode === "interpret") return <CreativeInterpretTool onOpenProducts={onOpenProducts} products={products} userId={userId} onBack={() => setEntered(false)} />;
  if (mode === "extract") return <AssetExtractionTool products={products} onBack={() => setEntered(false)} onOpenProducts={onOpenProducts} />;
  if (mode === "pose") return <ToolboxLauncher onChoose={chooseTool} />;
  if (mode === "canvas") return <WorkflowCanvas initialScene={canvasStartScene} products={products} userId={userId} onBack={() => setEntered(false)} onOpenProducts={onOpenProducts} />;
  if (mode === "detail") return <ToolboxLauncher onChoose={chooseTool} />;

  return <div className="ecommerce-toolbox tb-batch-workspace">
    <div className="tb-workbench-nav"><button className="tb-text-button" onClick={() => setEntered(false)}><ArrowLeft size={16} />工具箱</button>{onOpenRadar && <button className="tb-text-button" onClick={onOpenRadar}>创意雷达<ArrowUpRight size={14} /></button>}</div>
    <header className="tb-page-header"><div><h1>{tool.title}</h1><p>{mode === "copy" ? "上传原视频，写多版画面字幕；原声保留，完成后可逐条下载。" : tool.description}</p></div><div className="tb-header-actions"><button className="tb-secondary" onClick={() => setDialog("drafts")}><History size={16} />已存批次{drafts.length ? ` (${drafts.length})` : ""}</button><button className="tb-secondary" disabled={saving} onClick={save}>{saving ? <Loader2 size={16} className="spin" /> : <Save size={16} />}保存草稿</button></div></header>
    <div className="tb-batch-grid" id="tb-batch-panel">
      <aside className="tb-source-column" aria-label="共用原素材">
        <div className="tb-column-title"><h2><span className="tb-step-number">1</span>原视频</h2></div>
        {mode === "copy" && <label className="tb-field">关联产品<select value={renderProductId} onChange={(e) => setRenderProductId(e.target.value)}><option value="">选择产品</option>{products.map((product) => <option key={product.id} value={product.id}>{product.title}</option>)}</select></label>}
        {mode === "copy" && !products.length && <p className="tb-inline-note">先建立一份产品资料，再回来选择视频和字幕。{onOpenProducts && <button className="tb-text-button" onClick={onOpenProducts}>前往产品资料</button>}</p>}
        {sourceURL && <div className="tb-source-video"><video ref={videoRef} key={sourceURL} src={sourceURL} controls preload="metadata" onLoadedMetadata={(e) => { setDuration(Number.isFinite(e.currentTarget.duration) ? e.currentTarget.duration : 0); setMediaError(""); }} onError={() => setMediaError("无法预览，请换用可播放的 MP4 视频。")} /></div>}
        <MediaUpload label="原视频" kind="video" value={source} onChange={changeSource} onLibrary={() => setPicker("source")} onGenerated={() => setPicker("generated")} onError={setError} />
        {mode === "copy" && source && <div className="tb-recognize"><button className="tb-secondary" type="button" disabled={transcriptionBusy || scanBusy || !renderProductId} onClick={recognizeSpeech}>{transcriptionBusy ? <Loader2 size={16} className="spin" /> : <FileText size={16} />}{transcriptionBusy ? "正在识别语音…" : "识别语音与时间点"}</button><button className="tb-secondary" type="button" disabled={transcriptionBusy || scanBusy || !renderProductId} onClick={scanCaptions}>{scanBusy ? <Loader2 size={16} className="spin" /> : <Film size={16} />}{scanBusy ? "正在扫描画面文字…" : "扫描底部画面字幕"}</button><p className="tb-helper">语音识别需先在设置中填写 API Key，点击后会向该服务发送音轨；画面扫描在服务器本地处理。两者结果都需逐句核对。</p></div>}
        {mediaError && <p className="tb-inline-note" role="alert">{mediaError}</p>}
        {duration > 0 && <p className="tb-helper">原片时长 {Math.round(duration)} 秒</p>}
        <details className="tb-details tb-shared-details"><summary>通用设置 <span>{resolution}<ChevronDown size={16} /></span></summary>
          <label className="tb-field">批次名称<input value={batchName} onChange={(e) => setBatchName(e.target.value)} placeholder="未命名批次" maxLength={80} /></label>
          {mode === "copy" ? <p className="tb-helper">字幕成片当前固定为竖版 720P。</p> : <label className="tb-field">输出分辨率<select value={resolution} onChange={(e) => setResolution(e.target.value)}><option>480P</option><option>720P</option><option>1080P</option></select></label>}
          <div className="tb-preserve"><h3>默认保留</h3><div>{(mode === "model" ? ["产品", "文案", "场景"] : mode === "copy" ? ["人物", "产品", "场景"] : ["人物", "视频创意"]).map((item) => <span key={item}><Check size={13} />{item}</span>)}</div></div>
          <p className="tb-helper">{mode === "copy" ? "所有版本保留原视频画面与原声，只叠加确认后的字幕图层。" : "所有版本共用原视频和输出规格。画面细节可能变化。"}</p>
        </details>
      </aside>
      <section className="tb-version-column" aria-label="差异化版本清单">
        <div className="tb-column-title"><h2><span className="tb-step-number">2</span>{tool.title}</h2><span>{rows.length} 个版本</span></div>
        <div className="tb-list-toolbar"><span>选择版本编辑，并决定是否纳入检查</span><div><button className="tb-text-button" onClick={() => mode === "copy" ? (setError(""), setDialog("import")) : importRef.current?.click()}><Upload size={14} />{mode === "copy" ? "批量粘贴" : "批量导入"}</button><button className="tb-text-button" disabled={!rows.length} onClick={() => updateRows((items) => items.map((row) => ({ ...row, selected: !selectedAll })))}>{selectedAll ? "全部移出" : "全部纳入"}</button></div><input ref={importRef} type="file" hidden multiple accept=".jpg,.jpeg,.png,.webp" aria-label="批量导入参考图" onChange={(e) => { importImages(e.target.files); e.target.value = ""; }} /></div>
        <div className="tb-version-list">{rows.map((row, index) => {
          const issues = versionIssues(mode, row);
          const duplicate = summary.duplicateIds.has(row.id);
          return <article className={`tb-version-row ${current?.id === row.id ? "active" : ""}`} key={row.id}>
            <button className="tb-version-select" aria-label={`编辑版本 ${index + 1}`} aria-pressed={current?.id === row.id} onClick={() => setActiveId(row.id)}><VersionThumbnail row={row} mode={mode} /><span className="tb-row-copy"><strong><span className="tb-version-number">{String(index + 1).padStart(2, "0")}</span>{row.title || `版本 ${index + 1}`}</strong><span>{mode === "copy" ? row.segments?.length ? `${row.segments.length} 句逐句字幕` : row.text || "添加一版独立文案" : row.target ? mediaName(row.target) : mode === "model" ? "添加不同的模特参考图" : "添加目标商品参考图"}</span><small className={duplicate ? "tb-duplicate" : ""}>{!row.selected ? "未纳入本批次" : duplicate ? "内容重复，请调整差异" : issues.length ? "待完善" : "已配置"}</small></span></button>
            <div className="tb-row-actions"><button className={`tb-include-button ${row.selected ? "included" : ""}`} aria-label={`${row.selected ? "移出" : "纳入"}版本 ${index + 1}`} aria-pressed={row.selected} onClick={() => updateRow(row.id, { selected: !row.selected })}>{row.selected ? <><Check size={13} />已纳入</> : "纳入"}</button><button className="tb-icon-button" aria-label={`复制版本 ${index + 1}`} disabled={rows.length >= MAX_VERSIONS} onClick={() => addVersion({ ...row, id: crypto.randomUUID(), title: `${row.title || `版本 ${index + 1}`} 副本`, selected: true })}><Copy size={14} /></button><button className="tb-icon-button" aria-label={`移除版本 ${index + 1}`} onClick={() => setDialog({ type: "remove", id: row.id, name: row.title || `版本 ${index + 1}` })}><Trash2 size={14} /></button></div>
          </article>;
        })}</div><button className="tb-new-version" disabled={rows.length >= MAX_VERSIONS} onClick={() => addVersion()}><Plus size={15} />添加版本 <span>{rows.length}/{MAX_VERSIONS}</span></button>
        {summary.duplicateIds.size > 0 && <p className="tb-inline-note" role="status">{summary.duplicateIds.size} 个选中版本内容重复。修改差异或取消勾选，避免重复生成。</p>}
      </section>
      <aside className="tb-editor-column" aria-label="当前版本设置">
        {current ? <div key={current.id} className="tb-current-editor"><div className="tb-editor-heading"><h3>{mode === "copy" ? "写下新文案" : mode === "model" ? "添加模特参考图" : "添加商品参考图"}</h3><span>{current.title || `版本 ${currentIndex + 1}`}</span></div>
          {mode === "copy" ? <div className="tb-caption-editor">{current.segments?.length ? <><div className="tb-caption-heading"><strong>逐句字幕时间轴</strong><span>点击时间可定位原片，直接修改有误句子</span></div><div className="tb-caption-segments">{current.segments.map((segment, index) => <div className="tb-caption-segment" key={index}><div className="tb-caption-segment-top"><button type="button" className="tb-text-button" onClick={() => { if (videoRef.current) { videoRef.current.currentTime = Number(segment.start) || 0; videoRef.current.pause(); } }}>{Number(segment.start).toFixed(1)}–{Number(segment.end).toFixed(1)} 秒</button>{segment.needs_review && <small>{segment.review_reason || "识别不确定 · 请核对"}</small>}<button type="button" className="tb-icon-button" aria-label={`移除第 ${index + 1} 句`} onClick={() => updateRow(current.id, { segments: current.segments.filter((_, i) => i !== index) })}><Trash2 size={14} /></button></div><div className="tb-caption-times"><label>开始<input type="number" min="0" max="30" step="0.1" value={segment.start} onChange={(e) => updateSegment(current, index, { start: e.target.value })} /></label><label>结束<input type="number" min="0" max="30" step="0.1" value={segment.end} onChange={(e) => updateSegment(current, index, { end: e.target.value })} /></label></div>{segment.visual_text && <p className="tb-caption-observation">画面识别：{segment.visual_text}</p>}<label className="tb-field">第 {index + 1} 句<textarea rows={2} maxLength={60} value={segment.text} onChange={(e) => updateSegment(current, index, { text: e.target.value, needs_review: false })} /></label><button type="button" className="tb-text-button" onClick={() => setAI({ mode, row: { ...current, text: segment.text }, segmentIndex: index })}><Sparkles size={14} />AI 改写这一句</button>{segment.needs_review && <button type="button" className="tb-secondary tb-confirm-caption" onClick={() => updateSegment(current, index, { needs_review: false, review_reason: "" })}><Check size={14} />确认这句已核对</button>}</div>)}</div></> : <><label className="tb-field">字幕文案<textarea rows={5} value={current.text} onChange={(e) => updateRow(current.id, { text: e.target.value })} maxLength={60} placeholder="可先写文案，再转为逐句字幕…" /></label><div className="tb-input-meta"><span>{Array.from(current.text).length}/60</span></div><p className="tb-helper">未设时间点时，文案会持续显示整条视频。</p></>}<div className="tb-caption-actions"><button type="button" className="tb-secondary" disabled={(current.segments?.length || 0) >= 20} onClick={() => addSegment(current)}><Plus size={14} />{current.segments?.length ? "添加一句" : "转为逐句字幕"}</button></div>{current.segments?.length > 0 && <label className="tb-check"><input type="checkbox" checked={!!current.coverExisting} onChange={(e) => updateRow(current.id, { coverExisting: e.target.checked })} />遮盖画面底部旧字幕区域</label>}{current.coverExisting && <p className="tb-helper">用黑色区域覆盖底部约 23% 画面；不是无痕消除。请预览确认不会遮挡商品或人物。</p>}</div> : <><MediaUpload label={tool.target} kind="image" value={current.target} onChange={(target) => updateRow(current.id, { target })} onLibrary={() => setPicker(current.id)} onError={setError} />
            {mode === "product" && <><button className={`tb-option-button ${current.adapt ? "active" : ""}`} aria-pressed={current.adapt} onClick={() => updateRow(current.id, { adapt: !current.adapt })}><span><strong>同时替换商品文案</strong><small>{current.adapt ? "为新商品填写对应的口播或字幕" : "沿用原文案，请核对规格和功效"}</small></span><span className="tb-option-state">{current.adapt ? "已开启" : "未开启"}</span></button>{current.adapt && <label className="tb-field">商品文案<textarea rows={4} value={current.text} onChange={(e) => updateRow(current.id, { text: e.target.value })} placeholder="确认这版商品对应的口播／字幕" maxLength={3000} /></label>}</>}
          </>}
          {mode === "model" && <div className="tb-character-actions"><button className="tb-secondary" onClick={() => setCharacters({id:current.id,tab:"library"})}><FolderOpen size={16}/>从角色库选择</button><button className="tb-secondary" onClick={() => setCharacters({id:current.id,tab:"generate"})}><Sparkles size={16}/>AI 生成角色图</button></div>}
          {(mode !== "copy" || !current.segments?.length) && <div className="tb-ai-entry"><button className="tb-ai-button" onClick={() => setAI({ mode, row: { ...current } })}><Sparkles size={16} />{mode === "model" ? "AI 生成表现要求" : mode === "copy" ? "AI 写文案" : "AI 写商品文案"}</button></div>}
          <details ref={settingsRef} className="tb-details tb-version-details"><summary>更多设置 <span>{mode === "model" ? "补充要求、命名" : mode === "copy" ? "修改范围、命名" : "商品信息、命名"}<ChevronDown size={16} /></span></summary>
            <label className="tb-field">版本名称<input value={current.title} onChange={(e) => updateRow(current.id, { title: e.target.value })} placeholder={`版本 ${currentIndex + 1}`} maxLength={80} /></label>
            {mode === "copy" && <p className="tb-helper">当前只修改画面字幕，原声保留。</p>}
            {mode === "model" && <label className="tb-field">补充要求（选填）<textarea rows={3} value={current.notes} onChange={(e) => updateRow(current.id, { notes: e.target.value })} placeholder="例如：自然分享语气、居家穿搭" maxLength={2000} /></label>}
            {mode === "product" && <><label className="tb-field">商品名称（选填）<input value={current.name} onChange={(e) => updateRow(current.id, { name: e.target.value })} placeholder="目标商品名称" maxLength={100} /></label><label className="tb-field">已确认的卖点（选填）<textarea rows={2} value={current.claims} onChange={(e) => updateRow(current.id, { claims: e.target.value })} placeholder="材质、规格、功能等" maxLength={2000} /></label></>}
          </details>
          <p className="tb-helper">{mode === "copy" ? "原声保留；逐句字幕按设定时间显示" : mode === "model" ? "默认保留产品、文案和场景" : "默认保留人物和视频创意"}{!current.selected ? " · 此版本未纳入本批次，不会纳入检查" : ""}</p>
          {!current.selected && <button className="tb-text-button" onClick={() => updateRow(current.id, { selected: true })}>将此版本加入本批次</button>}
        </div> : <div className="tb-empty-editor"><Layers size={28} /><h2>添加第一个版本</h2><p>也可以一次导入多个参考图或多版文案。</p><button className="tb-secondary" onClick={() => addVersion()}><Plus size={16} />添加版本</button></div>}
      </aside>
    </div>
    <footer className="tb-batch-footer"><div className="tb-batch-total"><Layers size={20} /><div><strong>本批次 <b>{summary.selected.length}</b> 个版本</strong><small>{!source ? "先上传原视频" : `${summary.complete.length} 个已配置 · ${summary.incomplete} 个待完善`}{summary.duplicateIds.size ? ` · ${summary.duplicateIds.size} 个重复` : ""}</small></div></div><div className="tb-footer-actions"><button className="tb-primary" disabled={!canReview} onClick={() => setDialog("review")}>检查配置</button></div><p>{mode === "copy" ? "逐句字幕按时间点显示，原声保留。提交前请核对识别、文案和遮盖范围。" : "当前支持保存方案，视频生成暂未开放。"}</p></footer>
    {renderBatch && <section className="tb-render-results" aria-live="polite"><h2>{renderBatch.title} · {renderBatch.status === "completed" ? "已完成" : renderBatch.status === "needs_review" ? "部分失败" : "制作中"}</h2>{serverBatches.length > 1 && <label className="tb-field">查看批次<select value={renderBatch.id} onChange={async (e) => { try { setRenderBatch(await getRenderBatch(e.target.value)); } catch (err) { setError(err.message); } }}>{serverBatches.map((batch) => <option key={batch.id} value={batch.id}>{batch.title} · {new Date(batch.created_at).toLocaleString("zh-CN")}</option>)}</select></label>}<div className="tb-render-grid">{renderBatch.variants?.map((variant) => <article key={variant.id}><strong>{variant.title}</strong><p>{variant.status === "completed" ? "成片已完成" : variant.status === "failed" ? `制作失败：${variant.error_message || "未知错误"}` : "正在制作"}</p>{variant.status === "completed" && <><video controls preload="metadata" src={`/api/render-batches/${renderBatch.id}/variants/${variant.id}/file`} /><a className="tb-secondary" href={`/api/render-batches/${renderBatch.id}/variants/${variant.id}/file`} download={`ad-${variant.id}.mp4`}>下载成片</a></>}{variant.status === "failed" && <button className="tb-secondary" onClick={async () => { try { await retryRenderVariant(renderBatch.id, variant.id); setRenderBatch(await getRenderBatch(renderBatch.id)); } catch (err) { setError(err.message); } }}>重试这一条</button>}</article>)}</div></section>}
    {error && <div className="tb-feedback tb-error" role="alert"><span>{error}</span><button className="tb-icon-button" aria-label="关闭提示" onClick={() => setError("")}><X size={16} /></button></div>}
    {notice && <div className="tb-feedback" role="status"><Check size={16} /><span>{notice}</span><button className="tb-icon-button" aria-label="关闭保存提示" onClick={() => setNotice("")}><X size={16} /></button></div>}
    {characters && <CharacterLibrary initialTab={characters.tab} onClose={() => setCharacters(null)} onPick={(target) => { updateRow(characters.id,{target}); setCharacters(null); }} />}
    {ai && <ToolboxAI mode={ai.mode} row={ai.row} remaining={MAX_VERSIONS - rows.length} onApply={applyAI} onClose={() => setAI(null)} />}
    {picker === "generated" && <GeneratedVideoPicker products={products} productId={renderProductId} onClose={() => setPicker(null)} onPick={(video) => { changeSource(video); setPicker(null); }} />}
    {picker && picker !== "generated" && <AssetPicker products={products} kind={picker === "source" ? "video" : "image"} onClose={() => setPicker(null)} onPick={(asset) => { if (picker === "source") changeSource(asset); else updateRow(picker, { target: asset }); setPicker(null); }} />}
    {dialog?.type === "remove" && <BatchDialog title={`移除${dialog.name}？`} onClose={() => setDialog(null)}><p className="tb-muted">此版本的输入将从当前批次移除，其他版本不受影响。</p><div className="tb-plan-actions"><button className="tb-secondary" onClick={() => setDialog(null)}>保留版本</button><button className="tb-primary" onClick={() => removeVersion(dialog.id)}>移除版本</button></div></BatchDialog>}
    {dialog === "import" && <BatchDialog title="批量粘贴文案" onClose={() => setDialog(null)}>{error && <p className="tb-inline-note" role="alert">{error}</p>}<p className="tb-muted">每个版本之间用单独一行 --- 分隔。段内换行会保留。</p><label className="tb-field">多版文案<textarea autoFocus rows={12} value={bulkCopy} onChange={(e) => setBulkCopy(e.target.value)} placeholder={"第一版文案\n---\n第二版文案\n---\n第三版文案"} /></label><button className="tb-primary" disabled={!bulkCopy.trim()} onClick={() => { let imported; try { imported = parseBulkCopies(bulkCopy); } catch (e) { setError(e.message); return; } if (addImported(imported)) { setDialog(null); setBulkCopy(""); } }}>导入 {bulkCopy.split(/^\s*---\s*$/m).filter((text) => text.trim()).length} 个版本</button></BatchDialog>}
    {dialog === "drafts" && <BatchDialog title="已存批次" onClose={() => setDialog(null)}><p className="tb-muted">保存在当前浏览器，包含原素材及各工具的版本配置。</p><div className="tb-draft-list">{drafts.length ? drafts.map((draft) => <article key={draft.id}><button onClick={() => restore(draft)}><Layers size={18} /><span><strong>{draft.batchName || `${MODE_LABELS[draft.mode]}批次`}</strong><small>{draft.versionsByMode?.[draft.mode]?.length || 1} 个版本 · {new Date(draft.updatedAt).toLocaleString("zh-CN")}</small></span></button></article>) : <p className="tb-muted">暂无保存的批次。</p>}</div></BatchDialog>}
    {dialog === "review" && <BatchDialog title={`检查 ${summary.selected.length} 个版本`} onClose={() => setDialog(null)}>{error && <p className="tb-inline-note" role="alert">{error}</p>}<p className="tb-muted">原视频：{mediaName(source)} · {mode === "copy" ? "竖版 720P" : resolution}</p><div className="tb-review-list">{summary.selected.map((row) => <div key={row.id}><strong>{row.title || `版本 ${rows.indexOf(row) + 1}`}</strong><span>{versionIssues(mode, row).join("、") || (summary.duplicateIds.has(row.id) ? "内容重复，请调整" : row.segments?.length ? `${row.segments.length} 句逐句字幕` : "全片字幕")}</span></div>)}</div><p className="tb-inline-note">{ready ? mode === "copy" ? "逐句字幕按时间点显示；未设置时间点的版本会全片显示。原声保留。请核对文案与原声。" : "整批配置已就绪。批量生成服务尚未接入，可先保存。" : "请返回补齐缺项，或移出重复版本。"}</p>{mode === "copy" && summary.selected.length > 5 && <p className="tb-inline-note">首版每批最多制作 5 条，请取消多余版本。</p>}{mode === "copy" && !renderProductId && <p className="tb-inline-note">请先在左侧选择产品资料。</p>}<div className="tb-plan-actions"><button className="tb-secondary" onClick={() => setDialog(null)}>返回编辑</button><button className="tb-primary" disabled={saving} onClick={async () => { if (await save()) setDialog(null); }}><Save size={16} />保存整批配置</button><button className="tb-secondary" onClick={download}><Download size={15} />导出清单</button>{mode === "copy" && <button className="tb-primary" disabled={!ready || renderBusy || !renderProductId || summary.selected.length > 5} onClick={renderCaptions}>{renderBusy ? "提交中…" : "制作字幕成片"}</button>}</div></BatchDialog>}
  </div>;
}
