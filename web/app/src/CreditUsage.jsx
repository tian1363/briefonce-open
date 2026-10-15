import { useEffect, useState } from "react";
import { getMyCredits, getMyCreditLedger, getUsageActivity, getImageUsage, getAccountUsageActivity } from "./api.js";
import "./CreditUsage.css";
import UsageParameters, { usageResolution } from "./UsageParameters.jsx";

const names = { text: "文字生成", multimodal: "图片与视频理解", embedding: "向量检索", image_generation: "图片生成", image_edit: "图片编辑", video_generation: "视频生成", speech_recognition: "语音识别" };
const states = { reserved: "待结算", consumed: "已消耗", refunded: "已退回", unbilled: "未扣积分", unknown: "历史未记录" };
const taskStates = { ready: "已完成", succeeded: "已完成", completed: "已完成", failed: "失败", queued: "排队中", generating: "生成中", running: "处理中", pending: "等待中", submitted: "已提交" };
const time = (value) => value ? new Date(value).toLocaleString("zh-CN", { hour12: false }) : "—";
export const CreditSymbol = () => <span className="credit-symbol" role="img" aria-label="积分">◈</span>;
const sourceName = (source) => ({ managed: "平台积分", byok: "自带 API", waived: "本地测试免扣分", local: "本地排版", historical: "历史记录" }[source] || source);
const pointsLabel = (item) => item.billing_status === "unknown" ? "未记录" : item.billing_status === "refunded" ? `${item.points} 已退回` : item.billing_status === "unbilled" ? `0 · ${sourceName(item.source)}` : `${item.points}${item.billing_status === "reserved" ? " 待结算" : ""}`;

function TaskDetails({ item }) {
  return <div className="credit-task-details"><dl><dt>任务</dt><dd>{item.title || names[item.capability] || "模型调用"}</dd><dt>任务编号</dt><dd>{item.task_id || "旧记录未关联任务"}</dd>{item.product_title && <><dt>产品</dt><dd>{item.product_title}</dd></>}<dt>模型</dt><dd>{item.model || "该记录未保存模型信息"}</dd><dt>计费来源</dt><dd>{sourceName(item.source)}</dd><dt>任务状态</dt><dd>{taskStates[item.task_status] || item.task_status || "—"}</dd><dt>积分状态</dt><dd>{states[item.billing_status]}</dd><dt>创建时间</dt><dd>{time(item.created_at)}</dd>{item.completed_at && <><dt>结算时间</dt><dd>{time(item.completed_at)}</dd></>}</dl><UsageParameters item={item} />{item.error && <p role="alert">{item.error}</p>}{item.task_status === "ready" && (item.asset_id || item.character_id) && <a href={item.asset_id ? `/api/assets/${item.asset_id}/file` : `/api/characters/${item.character_id}/image`} target="_blank" rel="noreferrer">查看生成图片</a>}</div>;
}

export function ImageCreditBadge({ src, local = false }) {
  const [result, setResult] = useState(null);
  useEffect(() => {
    setResult(null);
    if (local) return;
    const match = src?.match(/^\/api\/(assets|characters)\/([^/?]+)\/(?:file|image)(?:\?|$)/);
    if (!match) return;
    let active = true, timer, attempts = 0;
    const load = () => getImageUsage(match[1] === "assets" ? "asset_id" : "character_id", match[2]).then((item) => {
      if (!active) return;
      setResult(item);
      if (item?.billing_status === "reserved" && attempts++ < 5) timer = setTimeout(load, 2000);
    }).catch(() => {});
    load();
    return () => { active = false; clearTimeout(timer); };
  }, [src, local]);
  if (local) return <span className="image-credit-badge"><CreditSymbol /> 0 · 本地排版</span>;
  if (!result) return null;
  return <details className="image-credit-details nodrag nopan" onPointerDown={(event) => event.stopPropagation()} onClick={(event) => event.stopPropagation()}><summary title="查看积分与任务详情"><CreditSymbol /> {pointsLabel(result)}</summary><TaskDetails item={result} /></details>;
}

export default function CreditUsagePanel({ userID, balance }) {
  const [account, setAccount] = useState(null), [data, setData] = useState(null), [ledger, setLedger] = useState([]);
  const [offset, setOffset] = useState(0), [error, setError] = useState(""), [busy, setBusy] = useState(false), [revision, setRevision] = useState(0);
  useEffect(() => {
    let active = true; setBusy(true); setError("");
    Promise.all(userID ? [Promise.resolve({ account: { balance } }), getAccountUsageActivity(userID, offset), Promise.resolve([])] : [getMyCredits(), getUsageActivity(offset), getMyCreditLedger()]).then(([credits, activity, entries]) => { if (active) { setAccount(credits); setData(activity); setLedger(entries); } }).catch((err) => { if (active) setError(err.message); }).finally(() => { if (active) setBusy(false); });
    return () => { active = false; };
  }, [offset, revision, userID, balance]);
  return <section className="credit-usage-panel" aria-label="积分消耗"><header><div><h2><CreditSymbol /> 积分消耗</h2><p>查看每次消耗、退回和对应任务。◈ 表示积分。</p></div><button type="button" disabled={busy} onClick={() => setRevision((value) => value + 1)}>刷新</button></header>{error && <p role="alert">{error}</p>}
    <div className="credit-usage-totals"><div><small>剩余积分</small><strong><CreditSymbol /> {account?.account?.unlimited ? "不限" : account?.account?.balance ?? "—"}</strong></div>{[["累计消耗", "consumed"], ["待结算", "reserved"], ["累计退回", "refunded"]].map(([label,key]) => <div key={key}><small>{label}</small><strong><CreditSymbol /> {data?.totals?.[key] ?? "—"}</strong></div>)}</div>
    <p className="credit-usage-note">任务提交时预扣，成功后结算；退回记录单独显示。自带 API 和本地测试免扣分任务显示 0 积分。</p>
    <div className="credit-activity-list" aria-busy={busy}>{data?.items?.map((item) => <details key={item.id}><summary><span><strong>{item.title === "模型调用" ? names[item.capability] || item.title : item.title}</strong><small>{time(item.created_at)} · {states[item.billing_status]}</small><small>{item.model || "型号未记录"}{item.capability?.startsWith("image_") ? ` · ${usageResolution(item)}` : ""}</small></span><b><CreditSymbol /> {pointsLabel(item)}</b></summary><TaskDetails item={item} /></details>)}{data && !data.items.length && <p>暂无任务消耗记录</p>}{busy && <p role="status">正在读取积分记录…</p>}</div>
    <div className="credit-pagination"><button type="button" disabled={busy || offset === 0} onClick={() => setOffset(Math.max(0, offset - 50))}>上一页</button><span>第 {offset / 50 + 1} 页</span><button type="button" disabled={busy || !data?.has_more} onClick={() => setOffset(offset + 50)}>下一页</button></div>
    {!userID && <details className="credit-ledger"><summary>积分流水 · 最近 50 条</summary>{ledger.map((entry) => <div key={entry.id}><span>{{ grant: "发放", reserve: "预扣", refund: "退回", adjustment: "调整" }[entry.kind] || "调整"} {names[entry.capability] || ""}<small>{time(entry.created_at)}{entry.note ? ` · ${entry.note}` : ""}</small></span><b><CreditSymbol /> {entry.amount > 0 ? "+" : ""}{entry.amount}</b><small>余额 {entry.balance_after}</small></div>)}{!ledger.length && <p>暂无积分流水</p>}</details>}
  </section>;
}
