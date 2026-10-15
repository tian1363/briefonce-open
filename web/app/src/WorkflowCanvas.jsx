import { analyticsContext, trackEvent } from "./analytics.js";
import CanvasSelect from "./CanvasSelect.jsx";
import PoseStructureReference, { poseStructurePayload, poseStructureRetry } from "./PoseStructureReference.jsx";
import ImageModelPicker from "./ImageModelPicker.jsx";
import ArtworkLibrary from "./ArtworkLibrary.jsx";
import ArtworkLibraryEntry from "./ArtworkLibraryEntry.jsx";
import { CanvasPreviewButton } from "./CanvasImagePreview.jsx";
import { useEffect, useMemo, useState } from "react";
import { ArrowLeft, Loader2, RotateCcw, Sparkles, ScanFace, ImagePlus, Layers } from "lucide-react";
import DownloadableImage from "./DownloadableImage.jsx";
import CharacterLibrary from "./CharacterLibrary.jsx";
import { listCharacters, listPosePresets, listProductAssets, startPoseGeneration } from "./api.js";
import CanvasBoard, { CanvasToolbarContext } from "./CanvasBoard.jsx";
import ProductImageCanvasScene from "./ProductImageCanvasScene.jsx";
import DetailPageCanvasScene from "./DetailPageCanvasScene.jsx";
import CanvasPosePicker from "./CanvasPosePicker.jsx";
import ProductPicker from "./ProductPicker.jsx";
import { CANVAS_SCENES, canvasScene } from "./canvasScenes.js";
import "./WorkflowCanvas.css";

const sceneIcons = { pose: ScanFace, "product-photo": ImagePlus, "detail-page": Layers };

const defaultPos = { product: { x: 0, y: 80 }, character: { x: 0, y: 390 }, batch: { x: 360, y: 215 }, detail: { x: 1120, y: 180 } };
const initialPoseIds = ["xhs-005", "xhs-006", "xhs-007", "xhs-008"];

function CanvasGraph({ structure, onStructure, imageOptions,onImageOptions,userId, productId, productAssets, assetIds, onAssetToggle, roles, characterId, onCharacter, onLibrary, presets, poseIds, onPose, category, onCategory, results, busy, error, onRun, onRetry, onDetail, onProduct }) {
  const chosenAssets = productAssets.filter((item) => assetIds.includes(item.id));
  const plannedCount = poseIds.length;
  const chosenRole = roles.find((item) => item.id === characterId);
  const statusLabel = (status) => ({ ready: "已完成", failed: "生成失败", generating: "生成中", queued: "排队中" })[status] || status;
  const node = (id, x, y, data) => ({ id, position: { x, y }, data });
  const nodes = useMemo(() => [
    node("product", defaultPos.product.x, defaultPos.product.y, { kind: "source", step: "01", title: "商品素材", content: <>
      <div className="workflow-field"><span>选择商品图（可多选）</span><details className="workflow-asset-picker nodrag"><summary className="nodrag" title={chosenAssets.length === 1 ? chosenAssets[0].original_name : undefined}>{chosenAssets.length === 1 ? chosenAssets[0].original_name : chosenAssets.length ? `已选 ${chosenAssets.length} 张 · 点击更改` : "从产品库选择"}</summary><div className="workflow-asset-options nowheel" role="group" aria-label="产品库商品图">{productAssets.map((item) => <div className="canvas-preview-asset" key={item.id}><CanvasPreviewButton src={`/api/assets/${item.id}/file`} label={item.original_name} /><button type="button" className={`workflow-asset-option nodrag ${assetIds.includes(item.id) ? "selected" : ""}`} aria-pressed={assetIds.includes(item.id)} disabled={!assetIds.includes(item.id) && assetIds.length >= 6} key={item.id} onClick={() => onAssetToggle(item.id)}><img src={`/api/assets/${item.id}/file`} alt={item.original_name} /><span title={item.original_name}>{item.original_name}</span><b aria-hidden="true">{assetIds.includes(item.id) ? "✓" : ""}</b></button></div>)}</div></details></div>
      {chosenAssets.length ? <div className={`workflow-selected-assets nowheel nopan nodrag ${chosenAssets.length === 1 ? "single" : ""}`}>{chosenAssets.map((item) => <figure key={item.id}><DownloadableImage wrapperClassName="downloadable-compact" src={`/api/assets/${item.id}/file`} alt={item.original_name} filename={item.original_name} /><figcaption title={item.original_name}>{item.original_name}</figcaption></figure>)}</div> : <p>{productAssets.length ? "请选至少一张商品图。" : "当前产品还没有可用的 JPG 或 PNG 商品图。"}</p>}
      <button type="button" className="workflow-text-button nodrag" onClick={onProduct}>查看产品资料</button>
    </> }),
    node("character", defaultPos.character.x, defaultPos.character.y, { kind: "source", step: "02", title: "模特参考", content: <>
      <label className="workflow-field">选择模特<CanvasSelect className="nodrag" value={characterId} getOptionImage={id => id ? `/api/characters/${id}/image` : ""} onChange={(event) => onCharacter(event.target.value)}><option value="">请选择</option>{roles.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</CanvasSelect></label>
      {chosenRole ? <DownloadableImage className="workflow-preview" src={`/api/characters/${chosenRole.id}/image`} alt={chosenRole.name} filename={chosenRole.name} /> : <p>请先选择或创建模特。</p>}
      <button type="button" className="workflow-text-button nodrag" onClick={onLibrary}>打开角色库</button>
    </> }),
    node("batch", defaultPos.batch.x, defaultPos.batch.y, { kind: "process", step: "03", title: "批量生成姿势", content: <><ImageModelPicker value={imageOptions} onChange={onImageOptions} defaultRatio="3:4" count={plannedCount} disabled={busy}/>
      <p>多张商品图共同作为参考；每个姿势生成 1 张，按姿势数量计费。</p>
      <label className="workflow-field">商品类型<CanvasSelect className="nodrag" value={category} onChange={(event) => onCategory(event.target.value)}><option value="apparel">服装</option><option value="bag">箱包</option><option value="handheld">手持商品</option></CanvasSelect></label>
      <PoseStructureReference assets={productAssets} primaryIds={assetIds} value={structure} onChange={onStructure} category={category} />
      <CanvasPosePicker presets={presets} selected={poseIds} onToggle={onPose} />
      {plannedCount > 6 && <p role="alert" className="workflow-error">已选 {plannedCount} 张结果；请选择最多 6 个姿势。</p>}
      {error && <p role="alert" className="workflow-error">{error}</p>}
      <button type="button" className="workflow-run nodrag" disabled={busy || !assetIds.length || !characterId || !poseIds.length || plannedCount > 6} onClick={onRun}>{busy ? <Loader2 size={16} className="spin" /> : <Sparkles size={16} />}{busy ? "提交中…" : `生成 ${plannedCount} 张`}</button>
    </> }),
    ...(results.length ? results.map((item, index) => node(`result-${item.id}`, 760 + (index % 3) * 280, 20 + Math.floor(index / 3) * 290, { kind: "result", step: String(index + 1).padStart(2, "0"), title: item.name, content: <>
      <p>{item.controls?.view_status}</p><p className={`workflow-status ${item.status}`}>{statusLabel(item.status)}</p>
      {item.controls?.asset_id && <p className="workflow-result-source" title={productAssets.find((asset) => asset.id === item.controls.asset_id)?.original_name || ""}>商品图：{productAssets.find((asset) => asset.id === item.controls.asset_id)?.original_name || "产品素材"}</p>}
      {item.status === "ready" && <DownloadableImage className="workflow-preview" src={`/api/characters/${item.id}/image`} alt={item.name} filename={`${item.name}.png`} />}
      {item.status === "failed" && <><p role="alert" className="workflow-error">{item.error}</p><button type="button" className="workflow-text-button nodrag" disabled={busy} onClick={() => onRetry(item)}><RotateCcw size={14} />重试这张</button></>}
    </> })) : [node("result-empty", 760, 170, { kind: "result", step: "04", title: "姿势结果", content: <p>提交后，逐张显示生成状态和图片。</p> })]),
    node("detail", 1640, defaultPos.detail.y, { kind: "next", step: "05", title: "继续创作", content: <><p>完成的姿势图可在这里查看和下载。核对后可继续制作商品详情长图。</p><button type="button" className="workflow-text-button nodrag" onClick={onDetail}>打开商品详情长图</button></> }),
  ], [structure, onStructure, imageOptions,onImageOptions,productAssets, assetIds, roles, characterId, presets, poseIds, category, results, busy, error, onAssetToggle, onCharacter, onLibrary, onPose, onCategory, onRun, onRetry, onDetail, onProduct]);
  const resultIDs = results.length ? results.map((item) => `result-${item.id}`) : ["result-empty"];
  const edges = [
    { id: "product-batch", source: "product", target: "batch" },
    { id: "character-batch", source: "character", target: "batch" },
    ...resultIDs.map((id) => ({ id: `batch-${id}`, source: "batch", target: id })),
    ...resultIDs.map((id) => ({ id: `${id}-detail`, source: id, target: "detail" })),
  ];
  return <CanvasBoard userId={userId} productId={productId} sceneId="pose" nodes={nodes} edges={edges} />;
}

function PoseCanvasScene({ productId, userId, onOpenProducts, onOpenDetail }) {
  const [imageOptions, setImageOptions] = useState({});
  const [assets, setAssets] = useState([]);
  const [assetIds, setAssetIds] = useState([]);
  const [roles, setRoles] = useState([]);
  const [characterId, setCharacterId] = useState("");
  const [presets, setPresets] = useState([]);
  const [poseIds, setPoseIds] = useState(initialPoseIds);
  const [category, setCategory] = useState("apparel");
  const [structure, setStructure] = useState({ supporting_asset_id: "", same_style_confirmed: false, scene: "studio" });
  useEffect(() => { setStructure(current => ({ ...current, supporting_asset_id: "", same_style_confirmed: false })); }, [productId]);
  const [results, setResults] = useState([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [library, setLibrary] = useState(false);
  const resultStorageKey = `scriptagent:workflow-results:${userId}:${productId}`;
  useEffect(() => {
    if (!productId || !results.length) return;
    try { localStorage.setItem(resultStorageKey, JSON.stringify(results.map((item) => item.id))); } catch {}
  }, [productId, resultStorageKey, results]);
  useEffect(() => { listPosePresets().then(setPresets).catch((err) => setError(err.message)); }, []);
  useEffect(() => { let active = true; listCharacters().then((items) => { if (active) { setRoles(items); setCharacterId((id) => id || items.find((item) => item.status === "ready" && item.source !== "pose")?.id || ""); } }).catch((err) => { if (active) setError(err.message); }); return () => { active = false; }; }, []);
  useEffect(() => {
    let active = true;
    setAssets([]); setAssetIds([]); setResults([]); setError("");
    if (productId) listProductAssets(productId).then((items) => {
      if (!active) return;
      const images = items.filter((item) => item.kind === "image" && ["image/jpeg", "image/png"].includes(item.mime_type));
      setAssets(images); setAssetIds(images[0] ? [images[0].id] : []);
    }).catch((err) => { if (active) setError(err.message); });
    return () => { active = false; };
  }, [productId]);
  useEffect(() => {
    if (!productId) return undefined;
    let active = true;
    const refresh = () => listCharacters().then((items) => {
      if (!active) return;
      setRoles(items);
      setResults((current) => {
        const latest = items.find((item) => item.source === "pose" && item.controls?.product_id === productId && item.controls?.batch_id);
        if (current.length && (!latest || current.some((item) => item.id === latest.id))) return current.map((item) => items.find((next) => next.id === item.id) || item);
        let savedIDs = [];
        try { savedIDs = JSON.parse(localStorage.getItem(resultStorageKey) || "[]"); } catch {}
        if (Array.isArray(savedIDs) && savedIDs.length && savedIDs.includes(latest?.id)) {
          const restored = savedIDs.map((id) => items.find((item) => item.id === id)).filter((item) => item?.source === "pose" && item.controls?.product_id === productId);
          if (restored.length) return restored;
        }
        return latest ? items.filter((item) => item.controls?.batch_id === latest.controls.batch_id).reverse() : [];
      });
    }).catch((err) => { if (active) setError(err.message); });
    refresh();
    const timer = setInterval(refresh, 2500);
    return () => { active = false; clearInterval(timer); };
  }, [productId, resultStorageKey]);
  const readyRoles = roles.filter((item) => item.status === "ready" && item.source !== "pose");
  function toggleAsset(id) {
    if (assetIds.includes(id)) setAssetIds(assetIds.filter((item) => item !== id));
    else if (assetIds.length < 6) setAssetIds([...assetIds, id]);
    else setError("每批最多选择 6 张商品图");
  }
  async function run() {
    if (busy || !productId || !assetIds.length || !characterId || !poseIds.length) return;
    if (poseIds.length > 6) { setError("每批最多选择 6 个姿势"); return; }
    setBusy(true); setError("");
    try { setResults(await startPoseGeneration({ ...poseStructurePayload(structure, category), image_options:imageOptions, product_id: productId, asset_ids: assetIds, character_id: characterId, pose_ids: poseIds, category })); }
    catch (err) { setError(err.message); }
    finally { setBusy(false); }
  }
  async function retry(item) {
    setBusy(true); setError("");
    try {
      const replacement = await startPoseGeneration({ ...poseStructureRetry(item.controls), image_options:imageOptions, product_id: item.controls?.product_id || productId, asset_ids: item.controls?.asset_ids ? JSON.parse(item.controls.asset_ids) : [item.controls?.asset_id || assetIds[0]], character_id: item.reference_id || characterId, pose_id: item.controls?.pose_id, category: item.controls?.category || "apparel", name: item.name });
      setResults((current) => current.map((row) => row.id === item.id ? replacement : row));
    } catch (err) { setError(err.message); }
    finally { setBusy(false); }
  }
  return <>
    <CanvasGraph structure={structure} onStructure={setStructure} imageOptions={imageOptions} onImageOptions={setImageOptions} userId={userId} productId={productId} productAssets={assets} assetIds={assetIds} onAssetToggle={toggleAsset} roles={readyRoles} characterId={characterId} onCharacter={setCharacterId} onLibrary={() => setLibrary(true)} presets={presets} poseIds={poseIds} onPose={(id) => setPoseIds((current) => current.includes(id) ? current.filter((item) => item !== id) : current.length < 6 ? [...current, id] : current)} category={category} onCategory={setCategory} results={results} busy={busy} error={error} onRun={run} onRetry={retry} onDetail={onOpenDetail} onProduct={() => onOpenProducts?.(productId)} />
    {library && <CharacterLibrary onClose={() => setLibrary(false)} onPick={(role) => { setCharacterId(role.character_id); setLibrary(false); listCharacters().then(setRoles).catch((err) => setError(err.message)); }} />}
  </>;
}

export default function WorkflowCanvas({ products, userId, onBack, onOpenProducts, initialScene }) {
  const [productId, setProductId] = useState(() => {
    try { return localStorage.getItem(`scriptagent:canvas-product:${userId}`) || products[0]?.id || ""; } catch { return products[0]?.id || ""; }
  });
  const [sceneId, setSceneId] = useState(() => {
    if (initialScene) return canvasScene(initialScene).id;
    try { return canvasScene(localStorage.getItem(`scriptagent:canvas-scene:${userId}`)).id; } catch { return "pose"; }
  });
  useEffect(() => { analyticsContext("canvas"); trackEvent("feature_open", ({pose:"pose", "product-photo":"refine", "detail-page":"detail"})[sceneId], "canvas"); }, [sceneId]);
  useEffect(() => {if(initialScene) setSceneId(canvasScene(initialScene).id);},[initialScene]);
  const [detailAssetIds, setDetailAssetIds] = useState([]);
  const [libraryOpen, setLibraryOpen] = useState(false);
  const [libraryUseVersion, setLibraryUseVersion] = useState(0);
  useEffect(() => { if (products.length && !products.some((item) => item.id === productId)) setProductId(products[0].id); }, [products, productId]);
  useEffect(() => { if (productId) try { localStorage.setItem(`scriptagent:canvas-product:${userId}`, productId); } catch {} }, [productId, userId]);
  useEffect(() => { try { localStorage.setItem(`scriptagent:canvas-scene:${userId}`, sceneId); } catch {} }, [sceneId, userId]);
  const toolbar = <><div className="workflow-toolbar-scenes"><span className="product-picker-label">创作场景</span><nav className="workflow-scenes" aria-label="创作场景">{CANVAS_SCENES.map(item => { const Icon = sceneIcons[item.id]; return <button type="button" key={item.id} title={item.description} aria-current={sceneId===item.id?"page":undefined} onClick={()=>setSceneId(item.id)}><Icon size={16} strokeWidth={1.8} aria-hidden="true"/><span>{item.title}</span></button>; })}</nav></div><div className="workflow-toolbar-context"><ProductPicker products={products} value={productId} onChange={nextId=>{setDetailAssetIds([]);setProductId(nextId);}} /><ArtworkLibraryEntry productId={productId} refreshKey={libraryOpen} onOpen={()=>setLibraryOpen(true)} /></div></>;
  return <div className="ecommerce-toolbox workflow-page">
    <div className="tb-workbench-nav"><button type="button" className="tb-text-button" onClick={onBack}><ArrowLeft size={16} />工具箱</button></div>
    {libraryOpen && <ArtworkLibrary products={products} productId={productId} onClose={() => setLibraryOpen(false)} onUse={(item) => { setProductId(item.product_id); setDetailAssetIds([item.asset_id]); setSceneId("detail-page"); setLibraryUseVersion((value) => value + 1); setLibraryOpen(false); }} />}
    {!productId && <div className="workflow-canvas-toolbar">{toolbar}</div>}
    <CanvasToolbarContext.Provider value={toolbar}>{productId ? <div key={`${userId}:${productId}:${sceneId}:${libraryUseVersion}`}>
      {sceneId === "pose" && <PoseCanvasScene userId={userId} productId={productId} onOpenProducts={onOpenProducts} onOpenDetail={() => setSceneId("detail-page")} />}
      {sceneId === "product-photo" && <ProductImageCanvasScene userId={userId} productId={productId} onOpenProducts={onOpenProducts} onUseAssets={(ids) => { setDetailAssetIds(ids); setSceneId("detail-page"); }} />}
      {sceneId === "detail-page" && <DetailPageCanvasScene userId={userId} productId={productId} product={products.find((item) => item.id === productId)} initialAssetIds={detailAssetIds} onOpenProducts={onOpenProducts} />}
    </div> : <p className="tb-muted">先选择产品资料，画布会展示对应素材和任务。{onOpenProducts && <button type="button" className="tb-text-button" onClick={() => onOpenProducts()}>前往产品资料</button>}</p>}</CanvasToolbarContext.Provider>
  </div>;
}
