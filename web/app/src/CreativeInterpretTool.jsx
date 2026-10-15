import { useEffect, useRef, useState } from "react";
import { ArrowLeft, Loader2, Sparkles } from "lucide-react";
import { connectCreativeSource, disconnectCreativeSource, getCreativeConnection, interpretCreative, readCreativeNote, uploadProductAsset, createProduct, appendProductReference } from "./api.js";
import "./CreativeInterpretTool.css";
import CanvasToggle from "./CanvasToggle.jsx";
import CanvasSelect from "./CanvasSelect.jsx";
import CanvasImagePreview from "./CanvasImagePreview.jsx";

const noteImageURL = url => `/api/creative/image?url=${encodeURIComponent(url)}`;
function NoteImage({ url, index, onPreview }) {
  const [failed, setFailed] = useState(false);
  const [retry, setRetry] = useState(0);
  return <div className="xhs-image-card">{failed ? <div className="xhs-image-error"><span>图片 {index + 1} 加载失败</span><button type="button" className="tb-secondary" onClick={() => { setFailed(false); setRetry(n => n + 1); }}>重试加载</button><small>仍无法加载时，请重新读取笔记。</small></div> : <button type="button" className="xhs-image-button" onClick={onPreview}><img src={`${noteImageURL(url)}${retry ? `&retry=${retry}` : ""}`} alt={`笔记图片 ${index + 1}`} loading="lazy" onError={() => setFailed(true)} /><span>图片 {index + 1} · 点击放大</span></button>}</div>;
}

export default function CreativeInterpretTool({ onBack, userId, products = [], onOpenProducts }) {
  const key = `briefonce:xhs-draft:${userId}`;
  const [sourceURL, setSourceURL] = useState(() => { try { return sessionStorage.getItem(key) || ""; } catch { return ""; } });
  const [materialNote, setMaterialNote] = useState("");
  const [comments, setComments] = useState("");
  const [connection, setConnection] = useState({ status: "login_required", message: "尚未连接小红书" });
  const [snapshot, setSnapshot] = useState(null);
  const [result, setResult] = useState(null);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [qr, setQR] = useState(null);
  const [previewIndex, setPreviewIndex] = useState(null);
  const [importProductId, setImportProductId] = useState("");
  const [importNotice, setImportNotice] = useState("");
  const [importProgress, setImportProgress] = useState(0);
  const imported = useRef(new Set());
  const [createdProducts, setCreatedProducts] = useState([]);
  const [importMode, setImportMode] = useState("existing");
  const [newProductTitle, setNewProductTitle] = useState("");
  const [saveImages, setSaveImages] = useState(true);
  const [saveOriginal, setSaveOriginal] = useState(true);
  const [saveAnalysis, setSaveAnalysis] = useState(true);
  const [savedProductId, setSavedProductId] = useState("");
  const productOptions = [...products, ...createdProducts.filter(p => !products.some(item => item.id === p.id))];
  const pending = useRef("");
  const mounted = useRef(true);
  const statusRequest = useRef(null);
  const connectionActivated = useRef(false);
  const connectionVersion = useRef(0);
  const resumeRead = useRef(null);
  const sourceRef = useRef(sourceURL);
  sourceRef.current = sourceURL;
  useEffect(() => { try { sessionStorage.setItem(key, sourceURL); } catch {} }, [key, sourceURL]);
  async function checkConnection() {
    connectionActivated.current = true;
    if (statusRequest.current) return statusRequest.current;
    const version = connectionVersion.current;
    const request = getCreativeConnection().then((next) => {
      if (mounted.current && version === connectionVersion.current) setConnection(next);
      return next;
    }).catch(() => {
      const next = { status: "unavailable", message: "暂时无法检查连接，正在重试" };
      if (mounted.current && version === connectionVersion.current) setConnection(next);
      return next;
    }).finally(() => { if (statusRequest.current === request) statusRequest.current = null; });
    statusRequest.current = request;
    return request;
  }
  useEffect(() => {
    mounted.current = true;
    const check = () => { if (connectionActivated.current && document.visibilityState !== "hidden") checkConnection(); };
    window.addEventListener("focus", check);
    document.addEventListener("visibilitychange", check);
    return () => { mounted.current = false; connectionVersion.current++; window.removeEventListener("focus", check); document.removeEventListener("visibilitychange", check); };
  }, [userId]);
  // Every successful check (poll, focus, or manual) completes the same login flow.
  useEffect(() => {
    if (connection.status !== "connected") return;
    setQR(null); setError("");
    const source = pending.current; pending.current = "";
    if (source && source === sourceRef.current) resumeRead.current?.(source);
  }, [connection.status]);
  async function read(source = sourceRef.current) {
    setBusy("read"); setError(""); setSnapshot(null); setResult(null); setPreviewIndex(null);
    try {
      const next = await readCreativeNote(source);
      if (!mounted.current || source !== sourceRef.current) return;
      setSnapshot(next);
      if (next.status === "parsed") { connectionActivated.current = true; setConnection({status:"connected", message:"已连接小红书"}); }
      if (next.status === "login_required") { pending.current = source; await connect(); }
      else if (next.status !== "parsed") setError(next.warning || "暂时无法读取这篇笔记");
    } catch (err) { if (mounted.current) setError(err.message); }
    finally { if (mounted.current) setBusy(""); }
  }
  resumeRead.current = read;
  async function connect() {
    connectionActivated.current = true;
    connectionVersion.current++; statusRequest.current = null;
    setBusy("connect"); setError("");
    try {
      const next = await connectCreativeSource();
      if (!mounted.current) return;
      setConnection(next);
      if (next.status === "awaiting_login") setQR({ image: next.qr_code, deadline: Date.now() + 240000 });
      else if (next.status !== "connected") setError(next.message);

    } catch (err) { if (mounted.current) setError(err.message); }
    finally { if (mounted.current) setBusy(""); }
  }
  useEffect(() => {
    if (!qr) return;
    let stopped = false;
    let timer;
    async function poll() {
      if (Date.now() >= qr.deadline) {
        setQR(null);
        setConnection({ status: "login_required", message: "登录二维码已过期，请重新连接" });
        setError("登录二维码已过期，请重新连接；笔记链接已保留。"); return;
      }
      const next = await checkConnection();
      if (stopped || next.status === "connected") return;
      // A temporary network failure must not cancel the user's ongoing scan.
      timer = setTimeout(poll, 2000);
    }
    timer = setTimeout(poll, 1000);
    return () => { stopped = true; clearTimeout(timer); };
  }, [qr]);
  async function disconnect() {
    connectionVersion.current++; statusRequest.current = null;
    setBusy("disconnect"); setError(""); pending.current = ""; setQR(null);
    try { const next = await disconnectCreativeSource(); if (mounted.current) { setConnection(next); setSnapshot(null); } }
    catch (err) { if (mounted.current) setError(err.message); }
    finally { if (mounted.current) setBusy(""); }
  }
  async function interpret(event) {
    event.preventDefault(); setBusy("interpret"); setError("");
    try { const next = await interpretCreative({ source_url: sourceURL, material_note: materialNote, comments }); if (mounted.current) { setResult(next); setSnapshot(next.snapshot); } }
    catch (err) { if (mounted.current) setError(err.message); }
    finally { if (mounted.current) setBusy(""); }
  }
  async function saveToProduct() {
    if (busy) return;
    setBusy("import"); setImportNotice(""); setImportProgress(0); setError("");
    try {
      const urls = saveImages ? snapshot?.images || [] : [];
      const sections = [];
      if (saveOriginal && snapshot?.description) sections.push(`### 笔记原文（外部参考，待核对）\n\n${snapshot.description}`);
      if (saveOriginal && materialNote.trim()) sections.push(`### 用户补充内容\n\n${materialNote.trim()}`);
      if (saveOriginal && comments.trim()) sections.push(`### 用户提供的评论\n\n${comments.trim()}`);
      if (saveAnalysis && result?.markdown) sections.push(`### 创意解读（AI 生成，待核对）\n\n${result.markdown}`);
      if (!urls.length && !sections.length) throw new Error("请选择要保存的图片或文字内容");
      let target = importProductId;
      if (importMode === "new") {
        const title = newProductTitle.trim();
        if (!title) throw new Error("请填写新产品名称");
        const form = new FormData(); form.append("title", title);
        form.append("product_md", new File([`# ${title}\n\n待补充产品事实。\n`], "product.md", {type:"text/markdown"}));
        const product = await createProduct(form); target = product.id;
        setCreatedProducts(items => [...items, product]); setImportProductId(target); setImportMode("existing");
      }
      if (!target) throw new Error("请选择目标产品");
      setSavedProductId(target);
      let textSaved = false; const failures = [];
      if (sections.length) {
        const content = `## 小红书参考素材：${snapshot?.title || "创意参考"}\n\n来源：${snapshot?.requested_url || sourceURL}\n\n参考内容不自动视为本产品事实。\n\n${sections.join("\n\n")}`;
        try { await appendProductReference(target, content); textSaved = true; }
        catch (err) { failures.push(`文字保存失败：${err.message}`); }
      }
      let successful = 0; let skipped = 0;
      for (let index = 0; index < urls.length; index++) {
        const id = `${target}:${urls[index]}`;
        if (imported.current.has(id)) { skipped++; continue; }
        try {
          const response = await fetch(noteImageURL(urls[index]));
          if (!response.ok) throw new Error("图片加载失败");
          const blob = await response.blob();
          const extension = { "image/jpeg": "jpg", "image/png": "png", "image/webp": "webp", "image/gif": "gif" }[blob.type];
          if (!extension) throw new Error("图片格式不支持");
          await uploadProductAsset(target, new File([blob], `小红书笔记-图片${index + 1}.${extension}`, { type: blob.type }));
          imported.current.add(id); successful++;
        } catch { failures.push(`图片 ${index + 1} 导入失败`); }
        if (mounted.current) setImportProgress(index + 1);
      }
      if (mounted.current) setImportNotice(`${textSaved ? "文字已保存；" : ""}已导入 ${successful} 张图片${skipped ? `，跳过已导入的 ${skipped} 张` : ""}。${failures.length ? failures.join("；") + "。可再次点击重试。" : "可在产品资料中查看。"}`);
    } catch (err) { if (mounted.current) setError(err.message); }
    finally { if (mounted.current) setBusy(""); }
  }
  const ready = snapshot?.status === "parsed" || Boolean(materialNote.trim() || comments.trim());
  return <div className="ecommerce-toolbox tb-interpret-workspace">
    <div className="tb-workbench-nav"><button className="tb-text-button" type="button" onClick={onBack}><ArrowLeft size={16} />工具箱</button></div>
    <header className="tb-page-header"><div><h1>创意解读</h1><p>读取小红书图文，拆解内容表达与创意手法。</p></div></header>
    <section className="xhs-connection" aria-label="小红书连接" aria-live="polite">
      <div><strong>{connection.message}</strong>{connection.username && <small>当前账号：{connection.username}</small>}<small>登录会话由你的独立连接助手保存，重新进入时检查有效性。</small><small>当前使用独立网页登录，可能与常用浏览器的小红书登录冲突。建议先复制链接再连接。</small></div>
      <div className="xhs-actions">
        <button type="button" className="tb-secondary" disabled={!!busy || connection.status === "checking"} onClick={checkConnection}>检查连接</button>
        {connection.status === "connected" ? <button type="button" className="tb-text-button" disabled={!!busy} onClick={disconnect}>断开连接</button> : <button type="button" className="tb-primary" disabled={!!busy || !!qr || connection.status === "checking" || connection.status === "not_configured"} onClick={connect}>{busy === "connect" ? "正在连接…" : "连接小红书"}</button>}
      </div>
    </section>
    {qr && <section className="xhs-login" aria-label="小红书扫码登录"><img src={qr.image} alt="使用小红书扫描登录二维码" /><div><strong>使用小红书扫码登录</strong><p role="status"><Loader2 size={14} className="spin" /> 等待小红书确认登录，正在自动检查连接状态…</p><p>连接成功后自动收起二维码并继续读取。若小红书提示二维码失效，请刷新二维码。</p><button type="button" className="tb-secondary" disabled={!!busy} onClick={() => { setQR(null); connect(); }}>刷新二维码</button><button type="button" className="tb-secondary" onClick={() => { setQR(null); pending.current = ""; setConnection({ status: "login_required", message: "已关闭扫码，重新连接时检查登录状态" }); }}>关闭扫码</button></div></section>}
    {error && <p role="alert" className="tb-inline-note">{error}</p>}
    <div className="tb-interpret-grid">
      <form className="tb-interpret-card" onSubmit={interpret}>
        <h2>笔记内容</h2>
        <label className="tb-field">单篇小红书笔记链接或分享文案<textarea rows="3" required value={sourceURL} onChange={(e) => { setSourceURL(e.target.value); setSnapshot(null); setResult(null); setPreviewIndex(null); pending.current = ""; }} placeholder="粘贴小红书完整分享链接或分享文案" /></label>
        <button className="tb-secondary" type="button" disabled={!!busy || !sourceURL.trim()} onClick={() => read()}>{busy === "read" ? <Loader2 size={16} className="spin" /> : null}读取笔记</button>
        <label className="tb-field">素材内容补充<textarea rows="5" maxLength={12000} value={materialNote} onChange={(e) => setMaterialNote(e.target.value)} placeholder="读取受限时，可粘贴正文、图片文字或视频逐字稿作为补充。" /></label>
        <label className="tb-field">评论补充<textarea rows="4" maxLength={8000} value={comments} onChange={(e) => setComments(e.target.value)} placeholder="粘贴代表性评论，解读时标注为用户提供。" /></label>
        <button className="tb-primary" disabled={!!busy || !ready}>{busy === "interpret" ? <Loader2 size={16} className="spin" /> : <Sparkles size={16} />}开始创意解读</button>
        <p className="tb-muted">读取内容不调用解读模型。仅取得分享标题时不能开始解读；可补充正文后继续。</p>
      </form>
      <div className="tb-interpret-card" aria-live="polite"><h2>读取结果与创意解读</h2>
        {snapshot && <div className="tb-interpret-source"><strong>{snapshot.title || snapshot.share_heading || "内容未读取"}</strong><p>{snapshot.warning}</p>{snapshot.description && <details open><summary>笔记正文</summary><p className="xhs-body">{snapshot.description}</p></details>}{snapshot.images?.length > 0 && <><p>发现图片 {snapshot.images.length} 张{result ? ` · 提交视觉模型 ${result.images_submitted || 0} 张` : ""}</p><div className="xhs-images">{snapshot.images.map((url, index) => <NoteImage key={`${index}:${url}`} url={url} index={index} onPreview={() => setPreviewIndex(index)} />)}</div></>}{result?.visual_warning && <p>{result.visual_warning}</p>}</div>}
        {(snapshot?.status === "parsed" || result) && <section className="xhs-import-panel" aria-label="保存到产品资料"><h3>保存到产品资料</h3><div className="xhs-actions"><button type="button" className="tb-secondary" aria-pressed={importMode === "existing"} disabled={!!busy} onClick={() => setImportMode("existing")}>已有产品</button><button type="button" className="tb-secondary" aria-pressed={importMode === "new"} disabled={!!busy} onClick={() => setImportMode("new")}>新建产品</button></div>{importMode === "existing" ? <label className="tb-field">目标产品<CanvasSelect aria-label="目标产品" value={importProductId} disabled={!!busy} onChange={event => setImportProductId(event.target.value)}><option value="">请选择目标产品</option>{productOptions.map(product => <option key={product.id} value={product.id}>{product.title}</option>)}</CanvasSelect></label> : <label className="tb-field">新产品名称<input value={newProductTitle} maxLength={200} disabled={!!busy} onChange={event => setNewProductTitle(event.target.value)} placeholder="填写新产品名称" /></label>}<div className="xhs-save-options"><CanvasToggle checked={saveImages && Boolean(snapshot?.images?.length)} disabled={!!busy || !snapshot?.images?.length} onChange={setSaveImages}>笔记图片（{snapshot?.images?.length || 0} 张）</CanvasToggle><CanvasToggle checked={saveOriginal} disabled={!!busy} onChange={setSaveOriginal}>笔记原文与补充文案</CanvasToggle><CanvasToggle checked={saveAnalysis && Boolean(result)} disabled={!!busy || !result} onChange={setSaveAnalysis}>创意解读结果{!result && "（解读后可选）"}</CanvasToggle></div><p className="tb-muted">文字会追加到产品资料，保留原有内容；笔记原文和 AI 解读会标明来源，作为待核对参考。</p><button type="button" className="tb-primary" disabled={!!busy || (importMode === "existing" ? !importProductId : !newProductTitle.trim())} onClick={saveToProduct}>{busy === "import" ? `正在保存 · 图片 ${importProgress}/${saveImages ? snapshot?.images?.length || 0 : 0}` : "保存选中内容"}</button>{importNotice && <p role="status">{importNotice}</p>}{savedProductId && onOpenProducts && <button type="button" className="tb-text-button" disabled={!!busy} onClick={() => onOpenProducts(savedProductId)}>查看产品资料</button>}</section>}
        {result ? <pre className="tb-interpret-output">{result.markdown}</pre> : <p className="tb-muted">先读取笔记并核对图文，再开始解读。视频和完整评论尚未自动解析。</p>}
      </div>
    </div>
    {previewIndex !== null && snapshot?.images?.length > 0 && <CanvasImagePreview images={snapshot.images.map((url, index) => ({ src: noteImageURL(url), alt: `笔记图片 ${index + 1}`, filename: `笔记图片${index + 1}` }))} initialIndex={previewIndex} onClose={() => setPreviewIndex(null)} />}
  </div>;
}
