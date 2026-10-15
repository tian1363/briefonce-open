import CanvasSelect from "./CanvasSelect.jsx";
import ImageModelPicker from "./ImageModelPicker.jsx";
import CanvasTextField from "./CanvasTextField.jsx";
import { useEffect, useMemo, useState } from "react";
import { Loader2, RotateCcw, Sparkles } from "lucide-react";
import DownloadableImage from "./DownloadableImage.jsx";
import CanvasBoard from "./CanvasBoard.jsx";
import CanvasReference, { referenceFocus } from "./CanvasReference.jsx";
import CanvasToggle from "./CanvasToggle.jsx";
import { listImageTransformJobs, listProductAssets, startImageTransformBatch } from "./api.js";

const modes = [
  { id: "studio_white", label: "专业白底图" },
  { id: "visible_detail", label: "可见细节特写" },
];
const statusText = { queued: "排队中", generating: "生成中", ready: "已完成", failed: "生成失败" };
const taskName = (mode) => modes.find((item) => item.id === mode)?.label || "商品图";

export default function ProductImageCanvasScene({ userId, productId, onOpenProducts, onUseAssets }) {
  const [imageOptions,setImageOptions]=useState({});
  const [assets, setAssets] = useState([]);
  const [references, setReferences] = useState([]);
  const [reference, setReference] = useState({ assetId: "", aspects: ["构图"], note: "" });
  const [sourceId, setSourceId] = useState("");
  const [selectedModes, setSelectedModes] = useState(["studio_white"]);
  const [detailFocus, setDetailFocus] = useState("");
  const [results, setResults] = useState([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const storageKey = `scriptagent:workflow-photo-results:${userId}:${productId}`;
  useEffect(() => {
    let active = true;
    listProductAssets(productId).then((items) => {
      if (!active) return;
      const images = items.filter((item) => item.kind === "image" && ["image/jpeg", "image/png"].includes(item.mime_type));
      setAssets(images); setReferences(items.filter((item) => item.kind === "reference")); setSourceId((current) => current || images[0]?.id || "");
    }).catch((err) => { if (active) setError(err.message); });
    return () => { active = false; };
  }, [productId]);
  useEffect(() => {
    let active = true;
    const refresh = () => listImageTransformJobs(productId).then((items) => {
      if (!active) return;
      setResults((current) => {
        if (current.length) return current.map((row) => items.find((item) => item.id === row.id) || row);
        let savedIDs = [];
        try { savedIDs = JSON.parse(localStorage.getItem(storageKey) || "[]"); } catch {}
        if (Array.isArray(savedIDs) && savedIDs.length) {
          const restored = savedIDs.map((id) => items.find((item) => item.id === id)).filter(Boolean);
          if (restored.length) return restored;
        }
        return items[0] ? items.filter((item) => item.batch_id === items[0].batch_id) : [];
      });
    }).catch((err) => { if (active) setError(err.message); });
    refresh();
    const timer = setInterval(refresh, 2500);
    return () => { active = false; clearInterval(timer); };
  }, [productId, storageKey]);
  useEffect(() => {
    if (!results.length) return;
    try { localStorage.setItem(storageKey, JSON.stringify(results.map((item) => item.id))); } catch {}
  }, [results, storageKey]);
  useEffect(() => {
    if (!results.some((item) => item.status === "ready" && item.output_asset_id && !assets.some((asset) => asset.id === item.output_asset_id))) return;
    let active = true;
    listProductAssets(productId).then((items) => { if (active) { setAssets(items.filter((item) => item.kind === "image")); setReferences(items.filter((item) => item.kind === "reference")); } }).catch((err) => { if (active) setError(err.message); });
    return () => { active = false; };
  }, [productId, results, assets]);
  async function start(input) {
    setBusy(true); setError("");
    try { return await startImageTransformBatch(productId, {...input,image_options:imageOptions}); }
    catch (err) { setError(err.message); return null; }
    finally { setBusy(false); }
  }
  async function run() {
    if (busy || !sourceId || !selectedModes.length) return;
    const batch = await start({ source_asset_id: sourceId, modes: selectedModes, detail_focus: detailFocus.trim(), reference_asset_id: reference.assetId, reference_focus: reference.assetId ? referenceFocus(reference.aspects, reference.note) || "构图" : "" });
    if (batch) setResults(batch);
  }
  async function retry(item) {
    if (busy) return;
    const batch = await start({ source_asset_id: item.source_asset_id, modes: [item.mode], detail_focus: item.detail_focus, reference_asset_id: item.reference_asset_id || "", reference_focus: item.reference_focus || "" });
    if (batch?.[0]) setResults((current) => current.map((row) => row.id === item.id ? batch[0] : row));
  }
  const source = assets.find((item) => item.id === sourceId);
  const completedAssetIds = results.filter((item) => item.status === "ready" && item.output_asset_id).map((item) => item.output_asset_id);
  const node = (id, x, y, data) => ({ id, position: { x, y }, data });
  const nodes = useMemo(() => [
    node("source", 0, 175, { kind: "source", step: "01", title: "原始挂拍图", content: <>
      <label className="workflow-field">选择商品图<CanvasSelect className="nodrag" value={sourceId} getOptionImage={id => id ? `/api/assets/${id}/file` : ""} onChange={(event) => setSourceId(event.target.value)}><option value="">请选择</option>{assets.map((item) => <option key={item.id} value={item.id}>{item.original_name}</option>)}</CanvasSelect></label>
      {source ? <DownloadableImage className="workflow-preview" src={`/api/assets/${source.id}/file`} alt={source.original_name} filename={source.original_name} /> : <p>先在产品资料中上传清晰的 JPG 或 PNG 挂拍图。</p>}
      <button type="button" className="workflow-text-button nodrag" onClick={() => onOpenProducts?.(productId)}>打开产品素材库</button>
    </> }),
    node("reference", 0, 495, { kind: "source", step: "01B", title: "视觉参考（可选）", content: <CanvasReference productId={productId} references={references} value={reference} onChange={setReference} onUploaded={(asset) => setReferences((current) => [...current, asset])} onError={setError} /> }),
    node("edit", 340, 160, { kind: "process", step: "02", title: "商品图精修", content: <>
      <p>选择要生成的图，每张独立调用图片模型并计费。</p><ImageModelPicker value={imageOptions} onChange={setImageOptions} count={selectedModes.length} disabled={busy} />
      <div className="workflow-mode-list" role="group" aria-label="商品图处理方式">{modes.map((mode) => <CanvasToggle key={mode.id} checked={selectedModes.includes(mode.id)} onChange={() => setSelectedModes((current) => current.includes(mode.id) ? current.filter((id) => id !== mode.id) : [...current, mode.id])}>{mode.label}</CanvasToggle>)}</div>
      {reference.assetId && <p>已连接视觉参考：{referenceFocus(reference.aspects, reference.note) || "构图"}</p>}
      {selectedModes.includes("visible_detail") && <label className="workflow-field">要突出的位置（选填）<CanvasTextField maxLength={80} value={detailFocus} onValueChange={setDetailFocus} placeholder="例如：领口、拉链；必须在原图中可见" /></label>}
      <p>只依据原图中可见的商品细节；生成后请核对颜色、版型和标识。</p>
      {error && <p role="alert" className="workflow-error">{error}</p>}
      <button type="button" className="workflow-run nodrag" disabled={busy || !sourceId || !selectedModes.length} onClick={run}>{busy ? <Loader2 size={16} className="spin" /> : <Sparkles size={16} />}{busy ? "提交中…" : `生成 ${selectedModes.length} 张`}</button>
    </> }),
    ...(results.length ? results.map((item, index) => node(`result-${item.id}`, 710, 20 + index * 300, { kind: "result", step: String(index + 1).padStart(2, "0"), title: taskName(item.mode), content: <>
      <p className={`workflow-status ${item.status}`}>{statusText[item.status] || item.status}</p>
      {item.reference_asset_id && <p>视觉参考：{item.reference_focus}</p>}
      {item.status === "ready" && <div className="workflow-comparison"><figure><DownloadableImage src={`/api/assets/${item.source_asset_id}/file`} alt="原始商品图" filename={assets.find((asset) => asset.id === item.source_asset_id)?.original_name} /><figcaption>原图</figcaption></figure><figure><DownloadableImage src={`/api/assets/${item.output_asset_id}/file`} alt={taskName(item.mode)} filename={assets.find((asset) => asset.id === item.output_asset_id)?.original_name || `${taskName(item.mode)}.png`} /><figcaption>生成图</figcaption></figure></div>}
      {item.status === "failed" && <><p role="alert" className="workflow-error">{item.error}</p><button type="button" className="workflow-text-button nodrag" disabled={busy} onClick={() => retry(item)}><RotateCcw size={14} />重试这张</button></>}
    </> })) : [node("result-empty", 710, 170, { kind: "result", step: "03", title: "精修结果", content: <p>提交后在这里逐张查看处理状态与原图对比。</p> })]),
    node("detail", 1070, 170, { kind: "next", step: "04", title: "用于详情长图", content: <><p>完成的图片已保存到产品素材库。核对无误后可以直接带入详情页场景。</p><button type="button" className="workflow-text-button nodrag" disabled={!completedAssetIds.length} onClick={() => onUseAssets(completedAssetIds)}>带入详情长图</button></> }),
  ], [imageOptions, assets, references, reference, sourceId, selectedModes, detailFocus, results, busy, error, onOpenProducts, productId, onUseAssets, completedAssetIds]);
  const resultIDs = results.length ? results.map((item) => `result-${item.id}`) : ["result-empty"];
  const edges = [
    { id: "source-edit", source: "source", target: "edit" },
    ...(reference.assetId ? [{ id: "reference-edit", source: "reference", target: "edit" }] : []),
    ...resultIDs.map((id) => ({ id: `edit-${id}`, source: "edit", target: id })),
    ...resultIDs.map((id) => ({ id: `${id}-detail`, source: id, target: "detail" })),
  ];
  return <CanvasBoard userId={userId} productId={productId} sceneId="product-photo" nodes={nodes} edges={edges} />;
}
