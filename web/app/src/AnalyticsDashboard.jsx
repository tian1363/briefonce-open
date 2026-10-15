import { useEffect, useState } from "react";
import { Activity, ArrowUpRight, CheckCheck, CircleHelp, Coins, Layers, TrendingUp, Users } from "lucide-react";
import { getOperationsAnalytics } from "./api.js";
import "./AnalyticsDashboard.css";

const names = { pose: "模特姿势", refine: "挂拍图精修", detail: "商品详情长图", extract: "视频素材提取", interpret: "创意解读", character: "角色生成", video: "视频生成", chat: "创意对话", other: "其他", canvas: "工作流", toolbox: "工具箱", composer: "输入框", navigation: "导航", library: "作品库" };
const number = (n) => new Intl.NumberFormat("zh-CN").format(n || 0);
const percent = (n, total) => total ? `${Math.round(n / total * 100)}%` : "—";
const journey = [["page_view", "进入工作台"], ["material_saved", "准备产品资料"], ["task_accepted", "提交创作"], ["result_preview", "放大预览"], ["download_served", "获取下载文件"], ["asset_imported", "导入资产"]];

function Empty({ children = "还没有记录，用户开始使用后会自动更新。" }) { return <div className="analytics-empty"><Layers size={25} aria-hidden="true" /><p>{children}</p></div>; }
function Ranking({ rows, unit = "个输出" }) {
  const max = Math.max(1, ...rows.map((r) => r.count));
  return rows.length ? <div className="analytics-rank">{rows.map((r) => <div key={r.key}><div className="analytics-rank-label"><span>{names[r.key] || r.key}</span><strong>{number(r.count)} <small>{unit}</small></strong></div><div className="analytics-track"><span style={{ width: `${r.count / max * 100}%` }} /></div></div>)}</div> : <Empty />;
}
export default function AnalyticsDashboard({ refreshVersion = 0 }) {
  const [featureMode,setFeatureMode] = useState("requests");
  const [days, setDays] = useState(30);
  const [version, setVersion] = useState(0);
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true; setLoading(true); setError(""); setData(null);
    getOperationsAnalytics(days).then((d) => { if (active) setData(d); }).catch((e) => { if (active) setError(e.message); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [days, version, refreshVersion]);
  const totalModels = (data?.models || []).reduce((sum, r) => sum + r.count, 0);
  const modelRows = data?.models || [];
  const modelTop = modelRows.slice(0, 4);
  const otherModels = modelRows.slice(4).reduce((sum, r) => sum + r.count, 0);
  const slices = [...modelTop.map((r) => ({ key: r.model, count: r.count })), ...(otherModels ? [{ key: "其他型号", count: otherModels }] : [])];
  let accumulated = 0;
  const gradient = slices.map((r, i) => { const start = accumulated; accumulated += r.count / totalModels * 100; return `var(--analytics-series-${i}) ${start}% ${accumulated}%`; }).join(",");
  const completedRate = percent(data?.completed, (data?.completed || 0) + (data?.failed || 0));
  const chartMax = Math.max(1, ...(data?.timeline || []).map((d) => d.tasks));
  const stats = data ? [
    ["有效活跃用户", number(data.active_users), "人", Users, `近 ${days} 天发生成功保存、提交或获取结果的用户`],
    ["完成产出", number(data.completed), "个输出", CheckCheck, `结束任务完成率 ${completedRate} · 失败 ${number(data.failed)}`],
    ["创作任务", number(data.tasks), "个输出", Layers, `处理中或待复核 ${number(data.pending)} · 提交请求 ${number(data.requests)}`],
    ["已使用积分", number(data.credits), "积分", Coins, "仅已结算消耗，不含预留及已退回积分"],
  ] : [];
  const activity = data?.activity || {};
  return <section className="analytics-dashboard analytics-dashboard-operations" aria-label="全站数据看板">
    <header className="analytics-heading"><p>全站客户数据 · 北京时间</p><div className="analytics-controls"><div className="analytics-period" role="group" aria-label="统计时间范围">{[7, 30, 90].map((n) => <button type="button" key={n} aria-pressed={days === n} onClick={() => setDays(n)}>近 {n} 天</button>)}</div></div></header>
    {error ? <div className="analytics-error" role="alert"><p>{error}</p><button type="button" onClick={() => setVersion((v) => v + 1)}>重新加载</button></div> : null}
    {loading ? <div className="analytics-loading" role="status"><Activity size={24} />正在读取全站使用数据…</div> : null}
    {data && <>
      <div className="analytics-stats analytics-activity">{[["有效日活 DAU", number(activity.dau), "今日"], ["有效周活 WAU", number(activity.wau), "含今日近 7 天"], ["有效月活 MAU", number(activity.mau), "含今日近 30 天"], ["用户黏性", activity.mau ? `${(activity.stickiness_bps / 100).toFixed(1)}%` : "—", "DAU / MAU"]].map(([label, value, hint]) => <article key={label}><div><span>{label}</span><Activity size={19} aria-hidden="true" /></div><p><strong>{value}</strong></p><small>{hint} · 不随下方时间范围变化</small></article>)}</div>
      <div className="analytics-stats">{stats.map(([label, value, suffix, Icon, hint]) => <article key={label}><div><span>{label}</span><Icon size={19} aria-hidden="true" /></div><p><strong>{value}</strong><span>{suffix}</span></p><small>{hint}</small></article>)}</div>
      <div className="analytics-grid">
        <article className="analytics-panel analytics-trend"><header><div><h2><TrendingUp size={18} />创作趋势</h2><p>按提交日期查看输出数量与当前完成状态</p></div><span className="analytics-period-label">{days} DAYS</span></header>
          <div className="analytics-legend"><span><i />提交任务</span><span><i />已完成</span></div>
          {data.tasks ? <><div className="analytics-bars" role="img" aria-label={`近 ${days} 天共提交 ${data.tasks} 个输出，完成 ${data.completed} 个`}>
            {data.timeline.map((d) => <div className="analytics-bar-group" key={d.date} title={`${d.date}：提交 ${d.tasks}，完成 ${d.completed}`}><div className="analytics-bar-total" style={{ height: `${d.tasks / chartMax * 100}%` }}><div style={{ height: `${d.tasks ? d.completed / d.tasks * 100 : 0}%` }} /></div></div>)}
          </div><div className="analytics-axis"><span>{data.timeline[0]?.date.slice(5)}</span><span>{data.timeline[Math.floor(data.timeline.length / 2)]?.date.slice(5)}</span><span>{data.timeline.at(-1)?.date.slice(5)}</span></div></> : <Empty />}
          <details className="analytics-details"><summary>查看每日明细</summary><div className="analytics-table-scroll"><table><thead><tr><th>日期</th><th>页面访问</th><th>有效用户</th><th>有效操作</th><th>提交输出</th><th>已完成</th></tr></thead><tbody>{data.timeline.map((d) => <tr key={d.date}><td>{d.date}</td><td>{d.visits}</td><td>{number(d.active_users)}</td><td>{d.actions}</td><td>{d.tasks}</td><td>{d.completed}</td></tr>)}</tbody></table></div></details>
        </article>
        <article className="analytics-panel"><header><div><h2>模型使用分布</h2><p>文本调用与图片、视频提交记录</p></div><ArrowUpRight size={18} aria-hidden="true" /></header>
          {totalModels ? <><div className="analytics-donut-row"><div className="analytics-donut" style={{ background: `conic-gradient(${gradient})` }} role="img" aria-label={`使用 ${modelRows.length} 个型号，共 ${totalModels} 条记录`}><div><strong>{number(totalModels)}</strong><span>模型记录</span></div></div><div className="analytics-model-legend">{slices.map((r, i) => <div key={r.key}><i style={{ background: `var(--analytics-series-${i})` }} /><span title={r.key}>{r.key}</span><strong>{percent(r.count, totalModels)}</strong></div>)}</div></div><details className="analytics-details"><summary>查看全部型号与记录</summary><div className="analytics-table-scroll"><table><thead><tr><th>型号</th><th>记录数</th><th>已成功</th></tr></thead><tbody>{modelRows.map((r) => <tr key={r.model}><td>{r.model}</td><td>{r.count}</td><td>{r.success}</td></tr>)}</tbody></table></div></details></> : <Empty>还没有模型记录，完成一次创作后在这里查看。</Empty>}
        </article>
        <article className="analytics-panel"><header><div><h2>功能使用</h2><p>区分打开功能、成功提交与实际输出</p></div><Layers size={18} aria-hidden="true" /></header><div className="analytics-feature-tabs" role="group" aria-label="功能使用口径">{[["requests","提交次数"],["outputs","输出数量"],["opens","打开次数"]].map(([id,label])=><button type="button" key={id} aria-pressed={featureMode===id} onClick={()=>setFeatureMode(id)}>{label}</button>)}</div><Ranking rows={(featureMode==="outputs"?data.features:featureMode==="opens"?data.feature_opens:data.feature_requests)||[]} unit={featureMode==="outputs"?"个输出":featureMode==="opens"?"次打开":"次提交"} /></article>
        <article className="analytics-panel"><header><div><h2>创作入口</h2><p>每次成功提交计一次，不乘以输出数量</p></div><ArrowUpRight size={18} aria-hidden="true" /></header><Ranking rows={data.entries} unit="次提交" />{data.errors > 0 && <p className="analytics-footnote">另有 {data.errors} 次请求未能提交，可检查资料、参数与可用额度。</p>}</article>
      </div>
      <article className="analytics-panel analytics-journey"><header><div><h2>使用路径</h2><p>各环节的参与会话数；允许跳过步骤，不作为连续转化漏斗。</p></div><span>{number(data.visits)} 次页面访问</span></header><div className="analytics-journey-steps">{journey.map(([key, label], i) => <div key={key}><span>0{i + 1}</span><strong>{number(data.journey.find((r) => r.key === key)?.count)}</strong><p>{label}</p></div>)}</div></article>
      <article className="analytics-panel analytics-retention"><header><div><h2>用户留存</h2><p>从首次有效行为起，查看指定日期是否再次有效使用；按全部已成熟用户统计。</p></div></header><div className="analytics-retention-grid">{[1, 7, 30].map((day) => { const eligible = activity[`retention_d${day}_eligible`] || 0; return <div key={day}><span>{day === 1 ? "次日" : `${day} 日`}留存</span><strong>{eligible ? `${(activity[`retention_d${day}_bps`] / 100).toFixed(1)}%` : "—"}</strong><small>{eligible ? `${number(activity[`retention_d${day}_returned`])} / ${number(eligible)} 位成熟用户` : "暂无成熟样本"}</small></div>; })}</div></article>
      {data.parameters.length > 0 && <article className="analytics-panel analytics-parameters"><header><div><h2>视频参数偏好</h2><p>比例、清晰度与时长的常用组合</p></div></header><Ranking rows={data.parameters} unit="个视频" /></article>}
      {data.latency_samples > 0 && <article className="analytics-panel analytics-latency"><div><h2>模型响应耗时</h2><p>仅包含有实测耗时的模型调用，不等同于整项任务完成时间</p></div><p><span>中位数 <strong>{(data.latency_p50_ms/1000).toFixed(1)}s</strong></span><span>P95 <strong>{(data.latency_p95_ms/1000).toFixed(1)}s</strong></span><small>{number(data.latency_samples)} 个样本</small></p></article>}
      <details className="analytics-method"><summary><CircleHelp size={15} />统计口径与数据范围</summary><p>汇总全站客户账号，排除管理员及 .test 测试账号；行为统计同时排除开发模拟环境。按北京时间自然日计算。任务、模型和积分来自已保存的业务记录，历史业务记录未标注环境时不推断环境；页面访问、有效活跃和使用路径从 {new Date(data.started_at).toLocaleDateString("zh-CN", { timeZone: "Asia/Shanghai" })} 开始采集，历史行为不会补写。</p><p>已结束任务完成率 = 完成 ÷（完成 + 失败或取消）。处理中和待核对任务不计入分母。模型分布展示已记录的文本调用及图片、视频提交；同一任务已存在模型调用记录时不重复补记，供应商轮询不计入分布。</p><p>下载统计表示服务端成功提供文件响应，不代表文件已保存到本地。图片预览按主动放大计数，缩略图加载不计入。输入正文、图片内容、密钥和内部判断过程均不作为行为埋点采集。</p></details>
    </>}
  </section>;
}
