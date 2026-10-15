import CanvasImagePreview from "./CanvasImagePreview.jsx";
import { DETAIL_TEMPLATES } from "./detailPageBuilder.js";
import { detailBriefError } from "./detailBrief.js";
import ImageModelPicker from "./ImageModelPicker.jsx";
import { useEffect, useRef, useState } from "react";
import DownloadableImage from "./DownloadableImage.jsx";
import { generateDetailPlan, previewDetailPlan, listDetailImages, startDetailImages } from "./api.js";
import { exportDetailImages } from "./detailImageExport.js";
import "./detailContentPlanner.css";

const labels = { whole: "完整详情图", hero: "首屏", benefits: "核心卖点", details: "真实细节", specs: "规格参数", usage: "使用与保养" };
export default function DetailContentPlanner({ userId, productId, draft, assets, onClose }) {
  const key = `briefonce:detail-plan:${userId}:${productId}`;
  const [imageOptions,setImageOptions]=useState({});
  const [referencePreview,setReferencePreview]=useState(false);
  const [saved] = useState(() => { try { return JSON.parse(localStorage.getItem(key) || "{}"); } catch { return {}; } });
  const [plan, setPlan] = useState(saved.plan ? { ...saved.plan, render_mode: saved.plan.render_mode || "whole" } : null);
  const [renderMode, setRenderMode] = useState(saved.plan?.render_mode || "whole");
  const [stamp, setStamp] = useState(saved.stamp || "");
  const [audience, setAudience] = useState(saved.audience || "");
  const [goal, setGoal] = useState(saved.goal || "");
  const [roles, setRoles] = useState(saved.roles || {});
  const [download, setDownload] = useState(null);
  const downloadRef = useRef(null);
  useEffect(() => () => { if (downloadRef.current) URL.revokeObjectURL(downloadRef.current); }, []);
  const [preview, setPreview] = useState(null);
  const [jobs, setJobs] = useState([]);
  const [platform, setPlatform] = useState("");
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const pending = useRef(null);
  const dialog = useRef(null);
  const latestPlan = useRef(plan); latestPlan.current = plan;
  useEffect(() => {
    const previous = document.activeElement;
    const root = dialog.current;
    root?.querySelector("button")?.focus();
    const handle = event => {
      if (event.key === "Escape") { onClose(); return; }
      if (event.key !== "Tab") return;
      const nodes = [...root.querySelectorAll('button:not(:disabled),input:not(:disabled),textarea:not(:disabled),select:not(:disabled),summary,a[href]')].filter(el => el.getClientRects().length);
      const first = nodes[0], last = nodes[nodes.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    };
    root?.addEventListener("keydown", handle);
    return () => { root?.removeEventListener("keydown", handle); previous?.focus(); };
  }, []);
  const input = { facts: { title: draft.title, claims: draft.claimsText, details: draft.detailText, specs: draft.specs, usage: draft.usage }, audience, goal, brand: `模板参考：${draft.referenceTemplateId || "自选参考"} ${draft.referenceTemplateName || ""}。实际参考图用于整体版式层级、图片节奏、构图与配色，替换为当前商品内容，不沿用参考品牌标志、品牌名、模板商品和未证实卖点。商品类型方向：${DETAIL_TEMPLATES.find(item => item.id === draft.templateId)?.name || "按商品资料"}。${(draft.referenceAspects || []).join("、")}；${draft.referenceNote || ""}`, assets: assets.map(a => ({ id: a.id, name: a.original_name, role: roles[a.id] || "待核对用途" })), platform_ids: draft.platformIds, reference_id: draft.referenceId };
  const fingerprint = JSON.stringify(input);
  const stale = plan && stamp !== fingerprint;
  useEffect(() => { try { localStorage.setItem(key, JSON.stringify({ plan, stamp, audience, goal, roles })); } catch {} }, [key, plan, stamp, audience, goal, roles]);
  useEffect(() => {
    let active = true; let timer;
    const refresh = async () => { try { const items = await listDetailImages(productId); if (active) { setJobs(items); setError(current => current.startsWith("任务记录读取失败") ? "" : current); } } catch (e) { if (active) setError(`任务记录读取失败：${e.message}`); } finally { if (active) timer = setTimeout(refresh, 4000); } };
    refresh(); return () => { active = false; clearTimeout(timer); };
  }, [productId]);
  useEffect(() => { setPreview(null); setConfirmed(false); pending.current = null; if (downloadRef.current) URL.revokeObjectURL(downloadRef.current); downloadRef.current = null; setDownload(null); }, [plan, fingerprint]);
  useEffect(() => { if (downloadRef.current) URL.revokeObjectURL(downloadRef.current); downloadRef.current = null; setDownload(null); }, [platform]);
  async function run(name, action) { setBusy(name); setError(""); try { await action(); } catch (e) { setError(e.message); } finally { setBusy(""); } }
  function edit(patch) { setPlan(p => ({ ...p, ...patch })); }
  function editPage(id, patch) { edit({ pages: plan.pages.map(p => p.id === id ? { ...p, ...patch } : p) }); }
  async function inspect() { const inspected = plan; const next = await previewDetailPlan(productId, inspected); if (latestPlan.current !== inspected) return; setPreview(next); setPlatform(current => next.tasks.some(t => t.platform_id === current) ? current : next.tasks[0]?.platform_id || ""); }
  async function start(pageID = "", retryID = "") {
    if (!retryID && !draft.referenceId) throw new Error("请先选择一张实际详情页模板图片，再生成详情页");
    const body = retryID ? { retry_job_id: retryID, confirmed: true } : { plan, platform_id: platform, page_id: pageID, confirmed };
    const signature = JSON.stringify({...body,image_options:imageOptions});
    if (pending.current?.signature !== signature) pending.current = { signature, requestID: crypto.randomUUID() };
    await startDetailImages(productId, { ...body,image_options:imageOptions, request_id: pending.current.requestID });
    pending.current = null;
    setJobs(await listDetailImages(productId));
  }
  const platforms = [...new Map((preview?.tasks || []).map(t => [t.platform_id, t.platform_name])).entries()];
  const tasks = preview?.tasks.filter(t => t.platform_id === platform) || [];
  const currentJobs = tasks.map(t => jobs.find(j => j.task.platform_id === t.platform_id && j.task.page_id === t.page_id && j.task.prompt === t.prompt));
  const ready = currentJobs.filter(j => j?.status === "ready");
  const running = jobs.some(j => ["queued", "generating"].includes(j.status));
  const missing = plan?.pages.some(p => tasks.some(t => t.page_id === p.id || t.section_ids?.includes(p.id)) && p.missing.length);
  return <div className="detail-plan-overlay"><section ref={dialog} className="detail-plan-dialog" role="dialog" aria-modal="true" aria-label="详情内容方案">
    <header><div><h2>AI 商品详情图</h2><p>使用已选商品信息与平台；生成结果会保存到作品库。</p></div><button type="button" onClick={onClose}>关闭</button></header>
    <div className="detail-plan-scroll">
      <label>出图方式<select value={renderMode} onChange={e => { setRenderMode(e.target.value); if (plan) edit({ render_mode: e.target.value }); setImageOptions({}); }}><option value="whole">整页一次生成 · 每个平台一张</option><option value="modules">逐屏生成 · 每屏可单独修改</option></select></label>
      <p>{renderMode === "whole" ? "AI 一次绘制整张详情设计，不拼接多张生成图。移动版默认 9:16，横版默认 16:9；完整文案先策划为适合画幅的内容。整页图不等同于平台模块上传包。" : "每屏分别调用 AI 生成，便于修改和上传平台模块。"}</p>
      {draft.referenceId && <figure className="detail-plan-reference"><button type="button" aria-label="放大预览模板参考图" onClick={()=>setReferencePreview(true)}><img className="detail-plan-source" src={`/api/assets/${draft.referenceId}/file`} alt="本次 AI 出图的模板参考图" /></button><figcaption>模板参考图 · 点击放大查看</figcaption></figure>}
      <div className="detail-plan-brief"><label>目标买家（选填）<input value={audience} onChange={e => setAudience(e.target.value)} maxLength={300} placeholder="例如：通勤女性、首次购买的用户" /></label><label>这套详情要解决什么（选填）<input value={goal} onChange={e => setGoal(e.target.value)} maxLength={300} placeholder="例如：讲清剪裁与真实面料细节" /></label></div>
      <details><summary>指定素材用途 · {assets.length} 张</summary><div className="detail-plan-assets">{assets.map(a => <label key={a.id}><img src={`/api/assets/${a.id}/file`} alt={a.original_name} /><span>{a.original_name}</span><input aria-label={`${a.original_name}用途`} value={roles[a.id] || ""} onChange={e => setRoles(v => ({ ...v, [a.id]: e.target.value }))} maxLength={200} placeholder="如：正面全貌、领口细节、真实上身" /></label>)}</div></details>
      <button type="button" className="workflow-run" disabled={!!busy || !assets.length} onClick={() => run("策划中…", async () => { const problem = detailBriefError(draft, assets.length); if (problem) throw new Error(problem); const next = await generateDetailPlan(productId, input); setPlan({ ...next, render_mode: renderMode }); setStamp(fingerprint); })}>{busy === "策划中…" ? busy : plan ? "根据当前资料重新策划" : "生成内容方案"}</button>
      <p>策划会调用文本模型；仅根据文字与素材用途规划，请核对每屏素材匹配。</p>
      {stale && <p role="status">商品资料或策划条件已变化，请重新策划后再生成。</p>}
      {plan && <>
        <h3>购买动机</h3><p>{({ visual: "视觉与外观", problem: "解决问题", lifestyle: "生活方式" })[plan.driver]} · {plan.rationale}</p>
        <label>整套统一视觉风格<textarea rows={3} value={plan.style_lock} onChange={e => edit({ style_lock: e.target.value })} maxLength={2000} /></label>
        <div className="detail-plan-pages">{plan.pages.map(p => <article key={p.id}><h3>{labels[p.id]}</h3>
          <label>买家问题 / 本屏目的<input value={p.purpose} onChange={e => editPage(p.id, { purpose: e.target.value })} maxLength={300} /></label>
          <label>商品素材<select value={p.source_asset_id} onChange={e => editPage(p.id, { source_asset_id: e.target.value })}>{assets.map(a => <option key={a.id} value={a.id}>{a.original_name}</option>)}</select></label>
          <img className="detail-plan-source" src={`/api/assets/${p.source_asset_id}/file`} alt="本屏商品依据" />
          <label>标题<input value={p.headline} onChange={e => editPage(p.id, { headline: e.target.value })} maxLength={60} /></label>
          <label>正文与标签<textarea rows={2} value={p.copy} onChange={e => editPage(p.id, { copy: e.target.value })} maxLength={600} /></label>
          <label>构图与画面<textarea rows={3} value={p.visual} onChange={e => editPage(p.id, { visual: e.target.value })} maxLength={1500} /></label>
          {p.missing.length > 0 && <div className="detail-plan-missing"><strong>待核对</strong><ul>{p.missing.map((m, i) => <li key={i}>{m}</li>)}</ul><button type="button" onClick={() => editPage(p.id, { missing: [] })}>已补充文案 / 素材并核对</button></div>}
          <details><summary>事实来源与场景</summary><p>{p.evidence.join(" / ")} · {p.scene_id}</p></details>
        </article>)}</div>
        <button type="button" disabled={!!busy || stale} onClick={() => run("整理提示词…", inspect)}>查看平台提示词与费用</button>
      </>}
      {preview && <section><h3>按平台生成</h3><ImageModelPicker value={imageOptions} onChange={v=>{setImageOptions(v);setConfirmed(false)}} defaultRatio={tasks[0]?.width > tasks[0]?.height ? "16:9" : renderMode === "whole" ? "9:16" : "auto"} count={tasks.length} disabled={!!busy}/><label>平台<select value={platform} onChange={e => { setPlatform(e.target.value); setConfirmed(false); }}>{platforms.map(([id, name]) => <option value={id} key={id}>{name}</option>)}</select></label>
        <p>{preview.source === "managed" && !imageOptions.model ? `每张 ${preview.points_per_image} 积分，本平台 ${tasks.length} 张共 ${tasks.length * preview.points_per_image} 积分。` : "积分按上方模型报价；自带 Key 的费用由模型服务商收取。"} 单张重试会再次调用模型。</p>
        <label className="detail-plan-confirm"><input type="checkbox" checked={confirmed} onChange={e => setConfirmed(e.target.checked)} />已核对商品事实、画面方案和费用</label>
        <button type="button" className="workflow-run" disabled={!!busy || running || !confirmed || missing || stale} onClick={() => run("提交图片任务…", () => start())}>生成本平台 {tasks.length} 张</button>
        {missing && <p>本平台还有待核对内容，完成后可生成。</p>}
        <div className="detail-plan-pages">{tasks.map((t, i) => { const job = currentJobs[i]; return <article key={t.page_id}><h3>{labels[t.page_id]} · {t.width}×{t.height}</h3><details><summary>完整提示词</summary><pre>{t.prompt}</pre></details>
          {job && <p role="status">{({ queued: "等待生成", generating: "正在生成", ready: "已保存到作品库", failed: "生成失败" })[job.status]}{job.error && `：${job.error}`}</p>}
          {job?.output_asset_id && <DownloadableImage src={`/api/assets/${job.output_asset_id}/file`} alt={t.headline} />}
          <button type="button" disabled={!!busy || running || !confirmed || stale || plan.pages.some(p => (p.id === t.page_id || t.section_ids?.includes(p.id)) && p.missing.length > 0)} onClick={() => run("提交图片任务…", () => start(t.page_id, job?.status === "failed" ? job.id : ""))}>{job?.status === "failed" ? "按原提示词重试" : job ? "重新生成这张" : "只生成这张"}</button>
        </article>; })}</div>
        <button type="button" disabled={!!busy || ready.length !== tasks.length} onClick={() => run("打包中…", async () => { const result = await exportDetailImages(ready, draft.title, { prepareOnly: true }); if (downloadRef.current) URL.revokeObjectURL(downloadRef.current); downloadRef.current = result.url; setDownload(result); })}>准备本平台尺寸图片包</button>{download && <a className="workflow-text-button" href={download.url} download={download.filename}>图片包已就绪 · 点击下载</a>}<p>按目标尺寸等比留边，不裁切商品；请检查生成文字后上传。</p>
      </section>}
      {!!jobs.length && <details><summary>历史任务 · {jobs.length} 张</summary><div className="detail-plan-pages">{jobs.map(j => <article key={j.id}><strong>{j.task.platform_name} · {labels[j.task.page_id]}</strong><p>{j.status} {j.error}</p>{j.output_asset_id && <DownloadableImage src={`/api/assets/${j.output_asset_id}/file`} alt={j.task.headline} />}<details><summary>当时使用的提示词</summary><pre>{j.task.prompt}</pre></details></article>)}</div></details>}
    </div>
    {referencePreview && <CanvasImagePreview images={[{src:`/api/assets/${draft.referenceId}/file`,alt:"模板参考图",filename:"详情页参考图.png"}]} initialIndex={0} onClose={()=>setReferencePreview(false)}/> }
    <footer aria-live="polite">{error ? <p role="alert" className="workflow-error">{error}</p> : busy || (running ? "图片正在后台生成，可以关闭此窗口后再查看。" : "方案修改会自动保存在当前浏览器。")}</footer>
  </section></div>;
}
