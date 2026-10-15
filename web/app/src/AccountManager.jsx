import ImageTaskHistory from "./ImageTaskHistory.jsx";
import PurchasePackages from "./PurchasePackages.jsx";
import CreditUsagePanel from "./CreditUsage.jsx";
import { useEffect, useState } from "react";
import {
  adjustManagedCredits, createManagedAccount, getMyCreditLedger, getMyCredits,
  getPlatformProvider, listManagedAccounts, listManagedLedger, resetManagedPassword, savePlatformProvider, updateManagedAccount,
  savePlatformModelProfile, updateCreditPrice,
} from "./api";
import "./AccountManager.css";

const displayTime = (value) => value ? new Date(value).toLocaleString("zh-CN", { hour12: false }) : "未设置";
const priceNames = { text: "文字", multimodal: "多模态", embedding: "向量", image_generation: "图片生成", image_edit: "图片编辑", video_generation: "视频每秒", speech_recognition: "语音识别" };
const capabilityNames = { text: "文字生成", multimodal: "图片与视频理解", embedding: "向量检索", image_generation: "图片生成", image_edit: "图片编辑", video_generation: "章鱼哥视频生成", speech_recognition: "语音识别" };

export default function AccountManager({ section = "all", modelCatalog = [] }) {
  const [data, setData] = useState({ accounts: [], prices: {} });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [credential, setCredential] = useState(null);
  const [selected, setSelected] = useState("");
  const [ledger, setLedger] = useState([]);
  const [provider, setProvider] = useState(null);
  const [platformKey, setPlatformKey] = useState("");

  async function refresh() {
    const [accounts, currentProvider] = await Promise.all([listManagedAccounts(), getPlatformProvider()]);
    setData(accounts);
    setProvider(currentProvider);
  }
  useEffect(() => {
    let active = true;
    refresh().catch((err) => { if (active) setError(err.message); });
    const updateUsage = () => listManagedAccounts().then((accounts) => { if (active) setData(accounts); }).catch(() => {});
    const interval = window.setInterval(updateUsage, 30000);
    window.addEventListener("focus", updateUsage);
    return () => { active = false; window.clearInterval(interval); window.removeEventListener("focus", updateUsage); };
  }, []);

  async function create(event) {
    event.preventDefault();
    const formElement=event.currentTarget;
    const form = new FormData(formElement);
    setBusy(true); setError("");
    try {
      const result = await createManagedAccount({
        email: String(form.get("email") || "").trim(),
        name: String(form.get("name") || "").trim(),
        trial_days: Number(form.get("trial_days")),
        credits: Number(form.get("credits")),
        daily_limit: Number(form.get("daily_limit")),
      });
      setCredential({ email: result.user.email, password: result.temporary_password });
      formElement.reset();
      await refresh();
    } catch (err) { setError(err.message); }
    finally { setBusy(false); }
  }

  async function act(callback) {
    setBusy(true); setError("");
    try { await callback(); await refresh(); if (selected) setLedger(await listManagedLedger(selected)); }
    catch (err) { setError(err.message); }
    finally { setBusy(false); }
  }

  async function showLedger(id) {
    if (selected === id) { setSelected(""); return; }
    setSelected(id);
    try { setLedger(await listManagedLedger(id)); } catch (err) { setError(err.message); }
  }

  async function saveKey(event) {
    event.preventDefault();
    setBusy(true); setError("");
    try {
      setProvider(await savePlatformProvider(platformKey));
      setPlatformKey("");
    } catch (err) { setError(err.message); }
    finally { setBusy(false); }
  }

  async function saveProfile(event, capability) {
    event.preventDefault();
    const formElement=event.currentTarget;
    const form = new FormData(formElement);
    setBusy(true); setError("");
    try {
      setProvider(await savePlatformModelProfile(capability, {
        provider: String(form.get("provider") || "dashscope"),
        model: String(form.get("model") || "").trim(),
        endpoint: String(form.get("endpoint") || "").trim(),
        account_ref:String(form.get("account_ref")||"").trim(),
        api_key: String(form.get("api_key") || "").trim(),
        use_shared_key: form.get("use_shared_key") === "on",
      }));
      formElement.querySelector('[name="api_key"]').value = "";
    } catch (err) { setError(err.message); }
    finally { setBusy(false); }
  }

  return <section className="account-manager" aria-label="账号与积分管理">
    <header className="account-manager-head"><div><h2>{section === "models" ? "平台模型接入" : "账号与积分"}</h2><p>发放试用账号，设置期限与额度。密码只在创建或重置后显示一次。</p></div><button type="button" className="secondary-button" onClick={() => refresh().catch((err) => setError(err.message))}>刷新</button></header>
    {section !== "accounts" && <><form className="platform-key-settings" onSubmit={saveKey}>
      <div><strong>百炼共享 Key</strong><p>{provider?.configured ? "共享百炼 Key 已保存。请检查下方每项模型和接口，试用账号统一使用平台积分。" : "请先配置百炼 Key，再发放可调用模型的试用账号。"}{provider?.mode === "mock" ? " 当前仍是模拟运行，需以真实模型模式启动服务。" : ""}</p></div>
      <label>平台 API Key<input type="password" value={platformKey} onChange={(event) => setPlatformKey(event.target.value)} placeholder={provider?.configured ? "输入新 Key 可替换现有配置" : "输入阿里云百炼 Key"} autoComplete="new-password" required /></label>
      <button className="secondary-button" type="submit" disabled={busy || !platformKey.trim()}>{busy ? "保存中" : provider?.configured ? "更换 Key" : "保存 Key"}</button>
      <small>密钥加密保存在服务端；此处不会显示原文。保存后请用低额度账号发起一次真实调用，确认供应商权限和积分扣减。</small>
    </form>
    <section className="supplier-model-catalog" aria-label="章鱼哥模型目录">
      <h3>章鱼哥 AI 图片模型</h3>
      <p>已接入 GPT Image 2.0 和 Nano Banana 系列。先在下方图片生成、图片编辑中保存章鱼哥 API Key，再到“积分定价”设置真实成本并启用档位。</p>
      <div className="supplier-model-grid">{modelCatalog.map(({model}) => <article key={model.id}>
        <strong>{model.name}</strong><code>{model.id}</code>
        <small>{model.tiers?.length ? model.tiers.join(" / ") : model.tier === "default" ? "标准档位" : model.tier} · 图片生成与编辑</small>
        <span>已接入 · 启用状态见积分定价</span>
      </article>)}</div>
      {!modelCatalog.length && <p>模型目录暂未加载，请点击刷新重试。</p>}
    </section>
    <datalist id="platform-image-model-choices">{modelCatalog.map(item=><option key={item.model.id} value={item.model.id}>{item.model.name}</option>)}</datalist>
<section className="platform-models" aria-label="平台模型配置">
      <div><h3>平台模型配置</h3><p>这里的设置供所有试用账号使用；管理员个人 Key 在自己的模型设置中单独管理。图片可选章鱼哥异步协议；各供应商 Key 分开保存。视频使用章鱼哥 Seedance 2.0 和 MiniMax H3 系列。</p></div>
      <div className="platform-model-list">{(provider?.profiles || []).map((profile) => <details key={profile.capability} className="platform-model-row" open={profile.capability.startsWith("image_")}>
        <summary><strong>{capabilityNames[profile.capability] || profile.capability}</strong><span>{profile.provider === "zexapi" ? "章鱼哥 AI" : profile.provider === "dashscope" ? "阿里云百炼" : "OpenAI 兼容接口"} · {profile.model}</span><small>{profile.configured ? "配置完整" : "待配置"}</small></summary>
        <form onSubmit={(event) => saveProfile(event, profile.capability)}>
          <label>服务商<select name="provider" defaultValue={profile.provider} onChange={e=>{const form=e.currentTarget.form;if(e.target.value==='zexapi'){form.elements.endpoint.value='https://zexapi.com/v1/videos';form.elements.model.value=profile.capability==='video_generation'?'seedance-2.0':'gpt-image-2';}}} disabled={!profile.capability.startsWith("image_") && profile.capability !== "speech_recognition" && profile.capability !== "video_generation"}>{profile.capability!=="video_generation" && <option value="dashscope">阿里云百炼</option>}{(profile.capability.startsWith("image_") || profile.capability==="video_generation") && <option value="zexapi">章鱼哥 AI</option>}{profile.capability === "speech_recognition" && <option value="openai">OpenAI 兼容接口</option>}</select></label>
          <label>模型{profile.capability==="video_generation" ? <select name="model" defaultValue={profile.model}>{["seedance-2.0","seedance-2.0-mini","minimax-h3","minimax-h3-max"].map(id=><option key={id} value={id}>{id}</option>)}</select> : <input name="model" list={profile.capability.startsWith("image_") ? "platform-image-model-choices" : undefined} defaultValue={profile.model} required />}</label><label>供应商账号标识（对账）<input name="account_ref" maxLength={120} defaultValue={profile.account_ref||""} placeholder="例如 zex-main；留空用 Key 指纹"/></label>
          <label className="platform-model-endpoint">接口地址<input name="endpoint" defaultValue={profile.endpoint || (profile.provider === "zexapi" ? "https://zexapi.com/v1/videos" : "")} placeholder={profile.capability === "video_generation" ? "https://zexapi.com/v1/videos" : profile.capability.startsWith("image_") ? "https://工作空间ID.cn-beijing.maas.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation" : "https://..."} required /></label>
          <label>供应商 API Key<input name="api_key" type="password" placeholder={profile.key_source === "capability" ? "仅同供应商可留空保留，换供应商须填新 Key" : profile.provider === "openai" ? "填写语音识别服务 Key" : "该供应商有独立 Key 时留空保留；百炼可使用共享 Key"} autoComplete="new-password" /></label>
          {profile.provider === "dashscope" && profile.key_source === "capability" && <label className="platform-shared-toggle"><input name="use_shared_key" type="checkbox" />改用平台共享 Key</label>}
          <button className="secondary-button" type="submit" disabled={busy}>{busy ? "保存中" : "保存模型"}</button>
          <small>状态表示配置完整，尚需用低额度试用账号验证模型权限与积分结算。</small>
          {profile.capability==="video_generation" && <small>4–15 秒 · 六种画幅 · 参考内容最多 9 项。<a href={profile.model.startsWith("minimax")?"https://6l0ket291i.apifox.cn/521606932e0":"https://6l0ket291i.apifox.cn/521606855e0"} target="_blank" rel="noreferrer">查看供应商 API 文档</a>；实际成本在积分定价中配置。</small>}
        </form>
      </details>)}</div>
    </section>
    </>}
 {section !== "models" && <><form className="account-create" onSubmit={create}>
      <label>登录邮箱<input name="email" type="email" required placeholder="user@example.com" /></label>
      <label>用户名称<input name="name" maxLength={80} placeholder="可选" /></label>
      <label>试用天数<input name="trial_days" type="number" min="1" max="365" defaultValue="7" required /></label>
      <label>初始积分<input name="credits" type="number" min="0" max="1000000" defaultValue="100" required /></label>
      <label>每日上限<input name="daily_limit" type="number" min="1" max="1000000" defaultValue="500" required /></label>
      <button type="submit" className="primary-button" disabled={busy}>{busy ? "创建中" : "创建账号"}</button>
    </form>
    {credential && <div className="account-credential" role="status">
      <div><strong>账号已创建</strong><p>请立即安全地保存并交给用户。关闭后无法再次查看原密码。</p><code>{credential.email}</code><code>{credential.password}</code></div>
      <button type="button" className="secondary-button" onClick={async () => { try { await navigator.clipboard.writeText(`账号：${credential.email}\n密码：${credential.password}`); } catch { setError("复制失败，请手动复制账号和密码"); } }}>复制账号密码</button>
      <button type="button" className="secondary-button" onClick={() => setCredential(null)}>关闭</button>
    </div>}
    {error && <p className="error-banner" role="alert">{error}</p>}
    <div className="account-price-note">当前积分规则：文字 {data.prices.text ?? "—"}/次，多模态 {data.prices.multimodal ?? "—"}/次，图片 {data.prices.image_generation ?? "—"}/张，视频按所选型号、秒数和参考素材报价。请在对外定价前核对真实供应商成本。</div>
    <details className="account-price-editor"><summary>调整积分单价</summary><div>{Object.entries(data.prices).filter(([capability])=>capability!=="video_generation").map(([capability, points]) => <div key={capability}><span>{priceNames[capability] || capability}</span><strong>{points} 积分</strong><button type="button" disabled={busy} onClick={() => { const raw = window.prompt(`设置${priceNames[capability] || capability}的单价`, String(points)); if (raw === null) return; const next = Number(raw); if (!Number.isSafeInteger(next) || next < 1 || next > 100000) { setError("单价需为 1 到 100000 的整数"); return; } act(() => updateCreditPrice(capability, next)); }}>修改</button></div>)}</div></details>
    <div className="account-list">
      {data.accounts.map(({ user, credit, daily_used: dailyUsed = 0 }) => <article key={user.id} className="account-row">
        <div className="account-row-main"><div><strong>{user.name || user.email}</strong><small>{user.email}</small></div><div><strong>{credit?.managed_enabled ? `剩余 ${credit.balance} 积分` : "自带 Key"}</strong><small>{credit?.managed_enabled ? `试用至 ${displayTime(credit.trial_ends_at)}` : "未开通平台额度"}</small></div><span className={user.status === "active" ? "account-active" : "account-suspended"}>{user.status === "active" ? "使用中" : "已停用"}</span></div>
        {credit?.managed_enabled && <div className="account-credit-progress">
          <div><span>今日已用</span><strong>{dailyUsed} / {credit.daily_limit}</strong></div>
          <progress value={Math.min(Math.max(dailyUsed, 0), credit.daily_limit)} max={Math.max(credit.daily_limit, 1)} aria-label={`${user.name || user.email} 今日积分使用进度`} />
        </div>}
        <div className="account-actions"><a className="secondary-button" href={`/admin/users/${encodeURIComponent(user.id)}/view`} target="_blank" rel="noopener noreferrer">只读查看工作台</a></div>
        {user.role === "member" && <div className="account-actions">
          {credit?.managed_enabled && <><button type="button" disabled={busy} onClick={() => { const raw = window.prompt("增加积分请输入正数，扣减请输入负数", "100"); if (raw === null) return; const amount = Number(raw); if (!Number.isSafeInteger(amount) || amount === 0) { setError("请输入非零整数积分"); return; } act(() => adjustManagedCredits(user.id, { amount, note: "管理员调整" })); }}>调整积分</button>
            <button type="button" disabled={busy} onClick={() => { const raw = window.prompt("从现在起延长试用多少天？", "7"); if (raw === null) return; const days = Number(raw); if (!Number.isInteger(days) || days < 1 || days > 365) { setError("请输入 1 到 365 天"); return; } act(() => updateManagedAccount(user.id, { trial_days: days })); }}>设置试用期</button></>}
          <button type="button" disabled={busy} onClick={() => act(async () => { const result = await resetManagedPassword(user.id); setCredential({ email: user.email, password: result.temporary_password }); })}>重置密码</button>
          <button type="button" disabled={busy} onClick={() => { const next = user.status === "active" ? "suspended" : "active"; if (next === "suspended" && !window.confirm(`停用 ${user.email}？用户会立即退出。`)) return; act(() => updateManagedAccount(user.id, { status: next })); }}>{user.status === "active" ? "停用" : "启用"}</button>
          {credit?.managed_enabled && <button type="button" onClick={() => showLedger(user.id)}>{selected === user.id ? "收起流水" : "查看流水"}</button>}
        </div>}
        {selected === user.id && <CreditUsagePanel key={user.id} userID={user.id} balance={credit?.balance} />}
        {selected === user.id && <div className="account-ledger">{ledger.length ? ledger.map((entry) => <div key={entry.id}><span>{entry.kind === "grant" ? "发放" : entry.kind === "reserve" ? "使用" : entry.kind === "refund" ? "退回" : "调整"}{entry.capability ? ` · ${entry.capability}` : ""}</span><strong>{entry.amount > 0 ? "+" : ""}{entry.amount}</strong><small>{displayTime(entry.created_at)}</small></div>) : <p>暂无积分流水</p>}</div>}
      </article>)}
      {!data.accounts.length && <p>暂无账号</p>}
    </div></>}
  </section>;
}

export function ManagedCreditsPanel() {
  const [data, setData] = useState(null);
  const [error, setError] = useState("");
  const [showPurchaseNote, setShowPurchaseNote] = useState(false);
  async function refresh() { setData(await getMyCredits()); }
  useEffect(() => { refresh().catch((err) => setError(err.message)); }, []);
  return <section className="managed-credits-workspace">
    <header><div><span>账户</span><h1>试用与积分</h1><p>你可以直接使用已开通的模型能力。每次调用前会检查试用期和积分。</p></div><button type="button" className="secondary-button" onClick={() => refresh().catch((err) => setError(err.message))}>刷新</button></header>
    {error && <p className="error-banner" role="alert">{error}</p>}
    {data?.account && <><div className="managed-credits-summary"><div><small>剩余积分</small><strong>{data.account.unlimited ? "不限" : data.account.balance}</strong></div><div><small>今日使用</small><strong>{data.account.unlimited ? "不限" : `${data.daily_used || 0} / ${data.account.daily_limit}`}</strong></div><div><small>试用截止</small><strong>{data.account.unlimited ? "本地测试" : displayTime(data.account.trial_ends_at)}</strong></div></div>
      <PurchasePackages />
      <div className="account-price-note">文字 {data.prices.text}/次 · 多模态 {data.prices.multimodal}/次 · 图片按所选模型报价 · 视频按所选型号和参数报价</div>
      <CreditUsagePanel /><ImageTaskHistory /></>}
  </section>;
}
