import PoseStructureReference, { poseStructurePayload, poseStructureRetry } from "./PoseStructureReference.jsx";
import PoseLibraryPicker from "./PoseLibraryPicker.jsx";
import ImageModelPicker from "./ImageModelPicker.jsx";
import { useEffect, useRef, useState } from "react";
import { ArrowLeft, Loader2, Sparkles, Upload } from "lucide-react";
import CharacterLibrary from "./CharacterLibrary.jsx";
import DownloadableImage from "./DownloadableImage.jsx";
import { listCharacters, listPosePresets, listProductAssets, startPoseGeneration, uploadProductAsset } from "./api.js";

export default function EcommercePoseTool({ products, onBack, onOpenProducts }) {
  const [productId, setProductId] = useState(products[0]?.id || "");
  const [assets, setAssets] = useState([]);
  const [assetId, setAssetId] = useState("");
  const [roles, setRoles] = useState([]);
  const [characterId, setCharacterId] = useState("");
  const [presets, setPresets] = useState([]);
  const [poseIds, setPoseIds] = useState(["xhs-005", "xhs-006", "xhs-007", "xhs-008"]);
  const [category, setCategory] = useState("apparel");
  const [structure, setStructure] = useState({ supporting_asset_id: "", same_style_confirmed: false, scene: "studio" });
  useEffect(() => { setStructure(current => ({ ...current, supporting_asset_id: "", same_style_confirmed: false })); }, [productId]);
  const [name, setName] = useState("");
  const [library, setLibrary] = useState(false);
  const [imageOptions,setImageOptions]=useState({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [results, setResults] = useState([]);
  const uploadRef = useRef(null);
  useEffect(() => { listPosePresets().then(setPresets).catch((e) => setError(e.message)); }, []);
  useEffect(() => {
    let active = true;
    setResults([]);
    listCharacters().then(items => {
      if (!active) return;
      setRoles(items);
      const batchId = items.find(item => item.source === "pose" && item.controls?.product_id === productId && item.controls?.batch_id)?.controls?.batch_id;
      if (batchId) setResults(items.filter(item => item.controls?.product_id === productId && item.controls?.batch_id === batchId).reverse());
    }).catch(e => { if (active) setError(e.message); });
    return () => { active = false; };
  }, [productId]);
  useEffect(() => {
    setAssets([]); setAssetId("");
    if (!productId) return;
    listProductAssets(productId).then((items) => setAssets(items.filter((item) => item.kind === "image" && ["image/png", "image/jpeg"].includes(item.mime_type)))).catch((e) => setError(e.message));
  }, [productId]);
  useEffect(() => {
    if (!results.some((item) => ["queued", "generating"].includes(item.status))) return;
    const timer = setInterval(() => listCharacters().then((items) => {
      setRoles(items);
      setResults((current) => current.map((item) => items.find((next) => next.id === item.id) || item));
    }).catch((e) => setError(e.message)), 2500);
    return () => clearInterval(timer);
  }, [results]);
  async function upload(event) {
    const file = event.target.files?.[0]; event.target.value = "";
    if (!file || !productId) return;
    setBusy(true); setError("");
    try {
      const asset = await uploadProductAsset(productId, file);
      if (asset.kind !== "image" || !["image/png", "image/jpeg"].includes(asset.mime_type)) throw new Error("请选择商品 JPG 或 PNG 图片");
      setAssets((current) => [asset, ...current]); setAssetId(asset.id);
    } catch (e) { setError(e.message); }
    finally { setBusy(false); }
  }
  async function generate(event) {
    event.preventDefault(); setBusy(true); setError("");
    try { setResults(await startPoseGeneration({ ...poseStructurePayload(structure, category), product_id: productId, asset_id: assetId, character_id: characterId, pose_ids: poseIds, category, name, image_options:imageOptions })); }
    catch (e) { setError(e.message); }
    finally { setBusy(false); }
  }
  async function retry(item) {
    setBusy(true); setError("");
    try {
      const replacement = await startPoseGeneration({ ...poseStructureRetry(item.controls), product_id: item.controls?.product_id || productId, asset_ids: item.controls?.asset_ids ? JSON.parse(item.controls.asset_ids) : [item.controls?.asset_id || assetId], character_id: item.reference_id || characterId, pose_id: item.controls?.pose_id, category: item.controls?.category || category, name: item.name, image_options:imageOptions });
      setResults((current) => current.map((row) => row.id === item.id ? replacement : row));
    } catch (e) { setError(e.message); }
    finally { setBusy(false); }
  }
  const readyRoles = roles.filter((item) => item.status === "ready" && item.source !== "pose");
  const character = readyRoles.find((item) => item.id === characterId);
  function togglePose(id) {
    setPoseIds((current) => current.includes(id) ? current.filter((item) => item !== id) : current.length < 6 ? [...current, id] : current);
  }
  return <div className="ecommerce-toolbox tb-pose-workspace">
    <div className="tb-workbench-nav"><button className="tb-text-button" onClick={onBack}><ArrowLeft size={16} />工具箱</button></div>
    <header className="tb-page-header"><div><h1>模特姿势合成</h1><p>组合商品图、模特和姿势，生成可下载的展示图。</p></div></header>
    <div className="tb-extract-grid"><form className="tb-interpret-card" onSubmit={generate}><ImageModelPicker value={imageOptions} onChange={setImageOptions} defaultRatio="3:4" count={poseIds.length} disabled={busy}/>
      <h2>素材与模特</h2>
      <label className="tb-field">产品资料<select value={productId} onChange={(e) => setProductId(e.target.value)}><option value="">请选择产品</option>{products.map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}</select></label>
      {!products.length && <p className="tb-inline-note">请先创建产品资料。{onOpenProducts && <button type="button" className="tb-text-button" onClick={onOpenProducts}>前往产品资料</button>}</p>}
      <div className="tb-pose-assets" role="group" aria-label="商品参考图">{assets.map((asset) => <button type="button" key={asset.id} aria-pressed={assetId === asset.id} className={assetId === asset.id ? "selected" : ""} onClick={() => setAssetId(asset.id)}><img src={`/api/assets/${asset.id}/file`} alt={asset.original_name} /><span>{asset.original_name}</span></button>)}</div>
      <input ref={uploadRef} hidden type="file" accept=".jpg,.jpeg,.png" onChange={upload} />
      <button type="button" className="tb-secondary" disabled={!productId || busy} onClick={() => uploadRef.current?.click()}><Upload size={15} />上传商品图</button>
      <div className="tb-pose-model">{character && <img src={`/api/characters/${character.id}/image`} alt={character.name} />}<div><strong>{character?.name || "尚未选择模特"}</strong><p>上传自己的模特图，或从角色库选用已有、AI 生成的电商模特。</p><button type="button" className="tb-secondary" onClick={() => setLibrary(true)}>选择或创建模特</button></div></div>
      <label className="tb-field">商品类型<select value={category} onChange={(e) => setCategory(e.target.value)}><option value="apparel">服装穿着</option><option value="bag">箱包佩戴</option><option value="handheld">手持商品</option></select></label>
      <PoseStructureReference assets={assets} primaryIds={[assetId]} value={structure} onChange={setStructure} category={category} />
      <label className="tb-field">结果名称（选填）<input maxLength={80} value={name} onChange={(e) => setName(e.target.value)} placeholder="例如：正面站姿 · 秋季外套" /></label>
    </form><section className="tb-interpret-card"><h2>批量选择姿势</h2><p className="tb-muted">选 1–6 个动作，一次提交；每张图单独生成、保存和计费。生成同时使用姿势图和动作描述，仅参考身体动作；人物以模特图为准，商品以商品图为准。</p><PoseLibraryPicker presets={presets} selected={poseIds} onToggle={togglePose} compact={false} /><p className="tb-pose-instruction">已选 {poseIds.length}/6 个姿势：{poseIds.map((id) => presets.find((pose) => pose.id === id)?.name || id).join("、") || "请选择姿势"}</p>
      {error && <p className="tb-inline-note" role="alert">{error}</p>}
      <button className="tb-primary" disabled={busy || !productId || !assetId || !characterId || !poseIds.length} onClick={generate}>{busy ? <Loader2 size={16} className="spin" /> : <Sparkles size={16} />}{busy ? "正在提交…" : `生成 ${poseIds.length} 张姿势图`}</button>
      {results.length > 0 && <div className="tb-pose-results" aria-live="polite"><h3>本批结果 · {results.filter((item) => item.status === "ready").length}/{results.length} 张完成</h3><div className="tb-pose-results-grid">{results.map((item) => <article className="tb-pose-result" key={item.id}><strong>{item.name}</strong><p>{item.controls?.view_status}</p><small>{item.status === "ready" ? "已完成并加入产品素材库" : item.status === "queued" ? "排队中…" : item.status === "generating" ? "生成中…" : "生成失败"}</small>{item.status === "ready" && <DownloadableImage src={`/api/characters/${item.id}/image`} alt={item.name} filename={`${item.name}.png`} />}{item.status === "failed" && <><p role="alert">{item.error}</p><button type="button" className="tb-secondary" disabled={busy} onClick={() => retry(item)}>重试这张</button></>}</article>)}</div></div>}
      <p className="tb-helper">图片由模型根据模特图、商品图和姿势文字生成。请核对商品颜色、版型、标识和人物身份；生成图不能代替真实尺寸或材质证明。</p>
    </section></div>
    {library && <CharacterLibrary onClose={() => setLibrary(false)} onPick={(role) => { setCharacterId(role.character_id); setLibrary(false); listCharacters().then(setRoles).catch((e) => setError(e.message)); }} />}
  </div>;
}
