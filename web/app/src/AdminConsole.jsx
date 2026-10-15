import VideoPricing from "./VideoPricing.jsx";
import AnalyticsDashboard from "./AnalyticsDashboard.jsx";
import brandLogo from "./assets/scriptagent-agent-v2.png";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  ArrowLeft,
  BarChart3,
  ChartNoAxesCombined,
  Users,
  SlidersHorizontal,
  Plug,
  ReceiptText,
  Wallet,
  ShieldCheck,
  RefreshCw,
  Download,
  Search,
  Plus,
  X,
  ChevronRight,
} from "lucide-react";
import UsageParameters, { usageResolution } from "./UsageParameters.jsx";
import AccountManager from "./AccountManager.jsx";
import InviteManager from "./InviteManager.jsx";
import {
  getOperations,
  saveImagePrice,
  saveCreditPackage,
  confirmPurchaseOrder,
  refundPurchaseOrder,
  reconcileSupplierCall,
  recoverSupplierCall,
  resolveSupplierCall,
  getSupplierRequests,
  importSupplierStatements,
  matchSupplierStatement,
  addSupplierDeposit,
  listManagedAccounts,
  createAdminPurchase,
  getImageCatalog,
} from "./api.js";
import "./AdminConsole.css";
const money = (v) =>
  v == null
    ? "待核对"
    : `¥${(Number(v) / 1e6)
        .toFixed(Number(v) > 0 && Number(v) < 100 ? 6 : 4)
        .replace(/0+$/, "")
        .replace(/\.$/, "")}`;
const when = (v) =>
  v
    ? new Date(v).toLocaleString("zh-CN", {
        timeZone: "Asia/Shanghai",
        hour12: false,
      })
    : "—";
const micros = (v) => Math.round(Number(v) * 1e6);
const names = {
  pending: "待提交",
  submitting: "提交中",
  queued: "排队中",
  in_progress: "生成中",
  saving: "保存中",
  completed: "已完成",
  failed: "失败",
  uncertain: "待核对",
  credited: "已到账",
  refunded: "已退款",
  partially_refunded: "部分退款",
  estimated: "估算",
  reconciled: "已核对",
  reserved: "待结算",
  consumed: "已扣除",
};
const tabs = [
  ["overview", "运营概览", BarChart3],
  ["analytics", "数据看板", ChartNoAxesCombined],
  ["accounts", "账号与邀请", Users],
  ["models", "模型接入", Plug],
  ["supplier", "供应商对账", ReceiptText],
  ["pricing", "积分定价", SlidersHorizontal],
  ["packages", "套餐与订单", Wallet],
  ["audit", "操作记录", ShieldCheck],
];
const descriptions = {
  overview: "查看业务运行、供应商支出与积分收入。",
  analytics: "查看全站用户活跃、功能使用、模型分布与创作路径。",
  accounts: "管理账号、试用期限和可用积分。",
  models: "按能力配置供应商凭证和默认模型。",
  supplier: "核对每次上游调用，保留成本证据与差异。",
  pricing: "以供应商成本为基础，管理模型档位和利润空间。",
  packages: "设置积分售卖价格，确认收款与积分到账。",
  audit: "保留配置与账务操作的修改依据。",
};
function Empty({ children }) {
  return (
    <div className="ops-empty">
      <ReceiptText size={28} />
      <strong>{children}</strong>
      <span>数据会在实际调用或配置完成后出现。</span>
    </div>
  );
}
function Badge({ value }) {
  return (
    <span
      className={`ops-badge ${["failed", "uncertain"].includes(value) ? "warning" : ["completed", "credited", "reconciled"].includes(value) ? "success" : ""}`}
    >
      {names[value] || value || "—"}
    </span>
  );
}
function Field({
  label,
  name,
  value,
  type = "text",
  min,
  max,
  step,
  required = true,
  onChange,
  children,
}) {
  return (
    <label className="ops-field">
      <span>{label}</span>
      {children || (
        <input
          name={name}
          type={type}
          defaultValue={value}
          min={min}
          max={max}
          step={step}
          required={required}
          onChange={onChange}
        />
      )}
    </label>
  );
}
function DataTable({ columns, rows, render, empty = "暂无记录" }) {
  return rows.length ? (
    <div className="ops-table-scroll">
      <table className="ops-table">
        <thead>
          <tr>
            {columns.map((c) => (
              <th key={c}>{c}</th>
            ))}
          </tr>
        </thead>
        <tbody>{rows.map(render)}</tbody>
      </table>
    </div>
  ) : (
    <Empty>{empty}</Empty>
  );
}
export default function AdminConsole({
  overview,
  isLoading,
  error: outerError,
  onRefresh,
  onLogout,
}) {
  const [tab, setTab] = useState(() =>
    (() => {
      const requested = window.location.hash.slice(1);
      if (tabs.some(([id]) => id === requested)) return requested;
      try {
        const saved = localStorage.getItem("briefonce-admin-tab");
        return tabs.some(([id]) => id === saved) ? saved : "overview";
      } catch {
        return "overview";
      }
    })(),
  );
  const [data, setData] = useState(null),
    [analyticsVersion, setAnalyticsVersion] = useState(0),
    [catalog, setCatalog] = useState(null),
    [accounts, setAccounts] = useState([]),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [query, setQuery] = useState(""),
    [status, setStatus] = useState(""),
    [page, setPage] = useState(0),
    [edit, setEdit] = useState(null),
    [requests, setRequests] = useState([]),
    [importText, setImportText] = useState(""),
    [importLines, setImportLines] = useState([]),
    [draft, setDraft] = useState({});
  const dialog = useRef(null);
  async function refresh() {
    const [next, a, c] = await Promise.all([
      getOperations(),
      listManagedAccounts(),
      getImageCatalog(),
    ]);
    setData(next);
    setAccounts(a.accounts);
    setCatalog(c);
  }
  useEffect(() => {
    let active = true;
    Promise.all([getOperations(), listManagedAccounts(), getImageCatalog()])
      .then(([d, a, c]) => {
        if (active) {
          setData(d);
          setAccounts(a.accounts);
          setCatalog(c);
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, []);
  useEffect(() => {
    window.history.replaceState(null, "", `${window.location.pathname}${window.location.search}#${tab}`);
    try {
      localStorage.setItem("briefonce-admin-tab", tab);
    } catch {
      /* Navigation works when local storage is unavailable. */
    }
    setPage(0);
    setQuery("");
    setStatus("");
  }, [tab]);
  useEffect(() => {
    if (edit) dialog.current?.showModal();
  }, [edit]);
  async function action(fn) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await fn();
      await refresh();
      setEdit(null);
      setNotice("已保存，历史记录与已提交任务保持原定价。");
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  const title = tabs.find((t) => t[0] === tab)?.[1] || "运营概览";
  const labelModel = (id) =>
    catalog?.models.find((x) => x.model.id === id)?.model.name || id;
  const calls = useMemo(
    () =>
      (data?.calls || []).filter(
        (c) =>
          (!query ||
            `${c.email} ${c.model} ${c.upstream_id} ${c.account_ref} ${c.key_ref}`
              .toLowerCase()
              .includes(query.toLowerCase())) &&
          (!status || c.status === status || c.reconcile_status === status),
      ),
    [data, query, status],
  );
  const visibleCalls = calls.slice(page * 25, (page + 1) * 25);
  const priceRows = (data?.prices || []).filter(
    (p) =>
      !query || labelModel(p.model).toLowerCase().includes(query.toLowerCase()),
  );
  const summary = data?.summary || {};
  const metrics = [
    [
      "已核对供应商成本",
      money(summary.confirmed_cost_micros),
      "扣除供应商退款",
    ],
    [
      "待核对预计成本",
      money(summary.estimated_cost_micros),
      `${summary.unreconciled_jobs || 0} 个任务未核对`,
    ],
    ["购买实收", money(summary.purchase_cash_micros), "已到账订单的现金流"],
    [
      "用量对应收入测算",
      money(summary.usage_revenue_micros),
      `${summary.unknown_revenue_points || 0} 积分来源未知`,
    ],
  ];
  async function showRequests(row) {
    setEdit({ type: "requests", row });
    setRequests([]);
    try {
      setRequests(await getSupplierRequests(row.id));
    } catch (e) {
      setError(e.message);
    }
  }
  function open(type, row = {}) {
    setDraft({
      cost: Number(row.cost_micros || 0) / 1e6,
      multiplier: Number(row.multiplier_bps || 0) / 10000,
      point: Number(row.point_micros || 0) / 1e6,
      fixed: row.fixed_points || 0,
      amount: Number(row.amount_micros || 0) / 1e6,
      paid: row.paid_points || 1000,
      bonus: row.bonus_points || 0,
      fee: Number(row.fee_bps || 0) / 100,
    });
    setEdit({ type, row });
  }
  async function submit(event) {
    event.preventDefault();
    const f = new FormData(event.currentTarget),
      val = (k) => String(f.get(k) || ""),
      num = (k) => Number(f.get(k));
    const row = edit.row;
    if (edit.type === "price")
      return action(() =>
        saveImagePrice(
          {
            ...row,
            enabled: f.has("enabled"),
            cost_micros: micros(val("cost")),
            multiplier_bps: Math.round(num("multiplier") * 10000),
            point_micros: micros(val("point")),
            fixed_points: num("fixed"),
          },
          val("note"),
        ),
      );
    if (edit.type === "package")
      return action(() =>
        saveCreditPackage(
          {
            id: row.id || "",
            name: val("name"),
            amount_micros: micros(val("amount")),
            paid_points: num("paid"),
            bonus_points: num("bonus"),
            fee_bps: Math.round(num("fee") * 100),
            active: f.has("active"),
          },
          val("note"),
        ),
      );
    if (edit.type === "confirm")
      return action(() => confirmPurchaseOrder(row.id, val("receipt")));
    if (edit.type === "refund")
      return action(() => refundPurchaseOrder(row.id, val("receipt")));
    if (edit.type === "reconcile")
      return action(() =>
        reconcileSupplierCall(row.id, {
          cost_micros: micros(val("cost")),
          refund_micros: micros(val("refund")),
          evidence: val("evidence"),
        }),
      );
    if (edit.type === "resolve")
      return action(() =>
        resolveSupplierCall(row.id, {
          cost_micros: micros(val("cost")),
          evidence: val("evidence"),
        }),
      );
    if (edit.type === "recover")
      return action(() =>
        recoverSupplierCall(row.id, {
          upstream_id: val("upstream_id"),
          note: val("note"),
        }),
      );
    if (edit.type === "order")
      return action(() => createAdminPurchase(val("user"), val("package")));
    if (edit.type === "deposit")
      return action(() =>
        addSupplierDeposit({
          account: val("account"),
          receipt: val("receipt"),
          amount_micros: micros(val("amount")),
          bonus_micros: micros(val("bonus")),
        }),
      );
    if (edit.type === "import")
      return action(() => importSupplierStatements(importLines));
  }
  const predicted =
    draft.fixed > 0
      ? Number(draft.fixed)
      : Math.max(
          1,
          Math.ceil(
            (micros(draft.cost) *
              Math.round(Number(draft.multiplier) * 10000)) /
              (10000 * micros(draft.point || 0)),
          ),
        );
  const packageRate =
    (Number(draft.amount) * (1 - Number(draft.fee) / 100)) /
    (Number(draft.paid) + Number(draft.bonus));
  function toolbar(children) {
    return (
      <div className="ops-toolbar">
        <label className="ops-search">
          <Search size={16} />
          <input
            aria-label="搜索记录"
            placeholder="搜索模型、用户或任务"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setPage(0);
            }}
          />
        </label>
        {children}
      </div>
    );
  }
  return (
    <div className="ops-shell">
      <aside className="ops-sidebar">
        <a className="ops-brand" href="/">
          <img
            src={brandLogo}
            alt=""
            onError={(e) => {
              e.currentTarget.style.display = "none";
            }}
          />
          <span>
            BriefOnce<small>运营后台</small>
          </span>
        </a>
        <nav aria-label="运营功能">
          {tabs.map(([id, name, Icon]) => (
            <button
              type="button"
              key={id}
              aria-current={tab === id ? "page" : undefined}
              className={tab === id ? "active" : ""}
              onClick={() => setTab(id)}
            >
              <Icon size={18} />
              {name}
              {tab === id && <ChevronRight size={14} />}
            </button>
          ))}
        </nav>
        <div className="ops-sidebar-foot">
          <ShieldCheck size={16} />
          <span>仅管理员可访问</span>
        </div>
        <a className="ops-back" href="/">
          <ArrowLeft size={16} />
          返回创作工作台
        </a>
      </aside>
      <main className="ops-main">
        <header className="ops-header">
          <div>
            <small>运营后台 / {title}</small>
            <h1>{title}</h1>
            <p>{descriptions[tab]}</p>
          </div>
          <div className="ops-header-actions">
            <button
              type="button"
              onClick={() =>
                action(async () => {
                  if (tab === "analytics") {
                    setAnalyticsVersion((v) => v + 1);
                    return;
                  }
                  await refresh();
                  await onRefresh?.();
                })
              }
              disabled={busy || isLoading}
            >
              <RefreshCw size={16} />
              刷新
            </button>
            <button type="button" onClick={onLogout}>
              退出登录
            </button>
          </div>
        </header>
        {(error || outerError) && (
          <p role="alert" className="error-banner">
            {error || outerError}
          </p>
        )}
        {notice && (
          <p className="ops-notice" role="status">
            {notice}
          </p>
        )}
        {tab === "analytics" ? (
          <AnalyticsDashboard refreshVersion={analyticsVersion} />
        ) : !data ? (
          <div className="ops-empty" role="status">
            正在读取运营数据…
          </div>
        ) : (
          <>
            {tab === "overview" && (
              <>
                <div className="ops-metrics">
                  {metrics.map(([name, value, note]) => (
                    <article key={name}>
                      <span>{name}</span>
                      <strong>{value}</strong>
                      <small>{note}</small>
                    </article>
                  ))}
                </div>
                <section className="ops-section">
                  <div className="ops-section-head">
                    <div>
                      <h2>业务运行</h2>
                      <p>真实任务与调用统计</p>
                    </div>
                    <span className="ops-caption">北京时间</span>
                  </div>
                  <div className="ops-business-metrics">
                    {[
                      ["用户", overview?.totals?.users],
                      ["创意空间", overview?.totals?.spaces],
                      ["创作任务", overview?.totals?.jobs],
                      ["模型调用", overview?.totals?.model_calls],
                    ].map(([name, value]) => (
                      <div key={name}>
                        <span>{name}</span>
                        <strong>{value ?? "—"}</strong>
                      </div>
                    ))}
                  </div>
                </section>
                <div className="ops-overview-grid">
                  <section className="ops-section">
                    <h2>近期供应商任务</h2>
                    <DataTable
                      columns={["模型", "状态", "预计成本"]}
                      rows={data.calls.slice(0, 6)}
                      render={(r) => (
                        <tr key={r.id}>
                          <td>
                            {labelModel(r.model)}
                            <small>{when(r.created_at)}</small>
                          </td>
                          <td>
                            <Badge value={r.status} />
                          </td>
                          <td>{money(r.estimated_micros)}</td>
                        </tr>
                      )}
                    />
                    <button
                      className="ops-link"
                      onClick={() => setTab("supplier")}
                    >
                      查看全部调用 <ChevronRight size={14} />
                    </button>
                  </section>
                  <section className="ops-section">
                    <h2>上线配置</h2>
                    <p className="ops-muted">
                      配置完整表示凭证齐备；供应商权限、价格和真实成本需要核对。
                    </p>
                    <div className="ops-check-list">
                      <button onClick={() => setTab("models")}>
                        <Plug size={18} />
                        <span>
                          连接供应商与默认模型<small>各能力独立配置</small>
                        </span>
                        <ChevronRight size={16} />
                      </button>
                      <button onClick={() => setTab("pricing")}>
                        <SlidersHorizontal size={18} />
                        <span>
                          {data.prices.filter((p) => p.enabled).length}{" "}
                          个模型档位已启用
                          <small>默认 2 倍成本，价格版本留存</small>
                        </span>
                        <ChevronRight size={16} />
                      </button>
                      <button onClick={() => setTab("packages")}>
                        <Wallet size={18} />
                        <span>
                          {data.packages.filter((p) => p.active).length}{" "}
                          个积分套餐已上架<small>线下收款确认后到账</small>
                        </span>
                        <ChevronRight size={16} />
                      </button>
                    </div>
                  </section>
                </div>
                <p className="ops-footnote">
                  充值现金流、用量收入测算与供应商成本分别统计。估算及未知来源不计作已确认利润。
                </p>
              </>
            )}
            {tab === "accounts" && (
              <>
                <AccountManager section="accounts" />
                <section className="ops-section">
                  <InviteManager />
                </section>
              </>
            )}
            {tab === "models" && (
              <AccountManager
                section="models"
                modelCatalog={catalog?.models || []}
              />
            )}
            {tab === "pricing" && (
              <section className="ops-section">
                <VideoPricing prices={data.video_prices || []} onSaved={refresh} />
                <div className="ops-section-head">
                  <div>
                    <h2>模型成本与积分报价</h2>
                    <p>
                      成本 × 倍数 ÷
                      每积分基准价值，向上取整；未配置真实成本的档位保持关闭。
                    </p>
                  </div>
                  <span className="ops-badge">默认 2 倍成本</span>
                </div>
                {toolbar()}
                <DataTable
                  columns={[
                    "模型 / 档位",
                    "供应商成本",
                    "倍数",
                    "积分基准",
                    "每张积分",
                    "状态",
                    "",
                  ]}
                  rows={priceRows}
                  render={(p) => (
                    <tr key={`${p.model}:${p.tier}`}>
                      <td>
                        <strong>{labelModel(p.model)}</strong>
                        <small>
                          {p.tier === "default" ? "供应商默认" : p.tier} · 版本{" "}
                          {p.version}
                        </small>
                      </td>
                      <td>{p.cost_micros ? money(p.cost_micros) : "未设置"}</td>
                      <td>{p.multiplier_bps / 10000}×</td>
                      <td>{money(p.point_micros)}</td>
                      <td>
                        <strong>
                          {p.cost_micros
                            ? p.fixed_points ||
                              Math.ceil(
                                (p.cost_micros * p.multiplier_bps) /
                                  (10000 * p.point_micros),
                              )
                            : "—"}
                        </strong>
                      </td>
                      <td>
                        <Badge value={p.enabled ? "已启用" : "待配置"} />
                      </td>
                      <td>
                        <button onClick={() => open("price", p)}>配置</button>
                      </td>
                    </tr>
                  )}
                />
              </section>
            )}
            {tab === "supplier" && (
              <>
                <div className="ops-metrics">
                  {metrics.slice(0, 2).map(([name, value, note]) => (
                    <article key={name}>
                      <span>{name}</span>
                      <strong>{value}</strong>
                      <small>{note}</small>
                    </article>
                  ))}
                  <article>
                    <span>供应商充值</span>
                    <strong>
                      {money(
                        data.deposits.reduce((a, d) => a + d.amount_micros, 0),
                      )}
                    </strong>
                    <small>预付余额，与生成支出分开记录</small>
                  </article>
                </div>
                <section className="ops-section">
                  <div className="ops-section-head">
                    <div>
                      <h2>实际调用与成本核对</h2>
                      <p>
                        最近 500
                        个任务可展开请求明细；导出包含全量记录。缺失实际成本时保持待核对。
                      </p>
                    </div>
                    <div className="ops-actions">
                      <a
                        className="ops-button"
                        href="/api/owner/operations/export"
                      >
                        <Download size={15} />
                        导出明细
                      </a>
                      <button
                        onClick={() => {
                          setImportText("");
                          setImportLines([]);
                          open("import");
                        }}
                      >
                        导入账单
                      </button>
                      <button onClick={() => open("deposit")}>记录充值</button>
                    </div>
                  </div>
                  {toolbar(
                    <select
                      aria-label="任务或对账状态"
                      value={status}
                      onChange={(e) => {
                        setStatus(e.target.value);
                        setPage(0);
                      }}
                    >
                      <option value="">全部状态</option>
                      {[
                        "completed",
                        "failed",
                        "uncertain",
                        "saving",
                        "estimated",
                        "reconciled",
                      ].map((s) => (
                        <option key={s} value={s}>
                          {names[s]}
                        </option>
                      ))}
                    </select>,
                  )}
                  <DataTable
                    columns={[
                      "任务 / 用户",
                      "模型",
                      "状态",
                      "预计 / 实际净成本",
                      "积分",
                      "对账",
                      "",
                    ]}
                    rows={visibleCalls}
                    render={(r) => (
                      <tr key={r.id}>
                        <td>
                          <strong>{r.email}</strong>
                          <small>{r.upstream_id || r.id}</small>
                          <small>
                            {when(r.created_at)} · 账号{" "}
                            {r.account_ref || r.key_ref}
                          </small>
                        </td>
                        <td>{labelModel(r.model)}<small>{r.model}</small><small>{usageResolution(r)}</small></td>
                        <td>
                          <Badge value={r.status} />
                          {r.error && (
                            <small className="ops-warning">{r.error}</small>
                          )}
                        </td>
                        <td>
                          {money(r.estimated_micros)}
                          <small>
                            {r.actual_micros == null
                              ? "实际：待核对"
                              : `实际净额 ${money(r.actual_micros - r.refund_micros)}`}
                          </small>
                        </td>
                        <td>
                          {r.points}
                          <small>
                            <Badge value={r.billing_status} />
                          </small>
                        </td>
                        <td>
                          <Badge value={r.reconcile_status} />
                        </td>
                        <td>
                          <div className="ops-row-actions">
                            <button onClick={() => showRequests(r)}>
                              明细
                            </button>
                            <button onClick={() => open("reconcile", r)}>
                              核对成本
                            </button>
                            {r.status === "uncertain" && (
                              <>
                                <button onClick={() => open("recover", r)}>
                                  找回任务
                                </button>
                                <button onClick={() => open("resolve", r)}>
                                  确认未生成
                                </button>
                              </>
                            )}
                          </div>
                        </td>
                      </tr>
                    )}
                  />
                  <div className="ops-pagination">
                    <span>{calls.length} 条 · 每页 25 条</span>
                    <button
                      disabled={page === 0}
                      onClick={() => setPage((p) => p - 1)}
                    >
                      上一页
                    </button>
                    <span>{page + 1}</span>
                    <button
                      disabled={(page + 1) * 25 >= calls.length}
                      onClick={() => setPage((p) => p + 1)}
                    >
                      下一页
                    </button>
                  </div>
                </section>
                <section className="ops-section">
                  <h2>导入账单与匹配</h2>
                  <DataTable
                    columns={[
                      "账单编号",
                      "供应商账号",
                      "上游任务",
                      "成本 / 退款",
                      "匹配状态",
                      "",
                    ]}
                    rows={data.statements}
                    render={(s) => (
                      <tr key={s.id}>
                        <td>{s.external_id}</td>
                        <td>{s.account}</td>
                        <td>{s.task_id}</td>
                        <td>
                          {money(s.amount_micros)} / {money(s.refund_micros)}
                        </td>
                        <td>
                          {s.match_id
                            ? "已匹配"
                            : s.candidates
                              ? `${s.candidates} 个候选`
                              : "未找到平台任务"}
                        </td>
                        <td>
                          {!s.match_id && (
                            <button
                              onClick={() => {
                                const candidate = data.calls.find(
                                  (c) =>
                                    (c.account_ref || c.key_ref) ===
                                      s.account && c.upstream_id === s.task_id,
                                );
                                if (!candidate) {
                                  setError(
                                    "未找到账号和任务 ID 一致的平台记录",
                                  );
                                  return;
                                }
                                action(() =>
                                  matchSupplierStatement(s.id, candidate.id),
                                );
                              }}
                            >
                              确认匹配
                            </button>
                          )}
                        </td>
                      </tr>
                    )}
                  />
                </section>
                <section className="ops-section">
                  <h2>供应商充值记录</h2>
                  <DataTable
                    columns={["账号", "充值", "赠送额度", "凭证", "时间"]}
                    rows={data.deposits}
                    render={(d) => (
                      <tr key={d.id}>
                        <td>{d.account}</td>
                        <td>{money(d.amount_micros)}</td>
                        <td>{money(d.bonus_micros)}</td>
                        <td>{d.receipt}</td>
                        <td>{when(d.created_at)}</td>
                      </tr>
                    )}
                  />
                </section>
              </>
            )}
            {tab === "packages" && (
              <>
                <section className="ops-section">
                  <div className="ops-section-head">
                    <div>
                      <h2>积分套餐</h2>
                      <p>赠分会降低单积分实际收入，发布前核对模型利润。</p>
                    </div>
                    <button
                      className="ops-primary"
                      onClick={() => open("package")}
                    >
                      <Plus size={16} />
                      新建套餐
                    </button>
                  </div>
                  <div className="ops-package-grid">
                    {data.packages.map((p) => (
                      <article key={p.id}>
                        <div>
                          <h3>{p.name}</h3>
                          <Badge value={p.active ? "已上架" : "未上架"} />
                        </div>
                        <strong>{money(p.amount_micros)}</strong>
                        <p>
                          {p.paid_points + p.bonus_points} 积分
                          <small>
                            付费 {p.paid_points} · 赠送 {p.bonus_points}
                          </small>
                        </p>
                        <span>
                          每积分实付{" "}
                          {money(
                            p.amount_micros / (p.paid_points + p.bonus_points),
                          )}
                        </span>
                        <button onClick={() => open("package", p)}>
                          修改与利润预览
                        </button>
                      </article>
                    ))}
                  </div>
                  {!data.packages.length && <Empty>还没有积分套餐</Empty>}
                </section>
                <section className="ops-section">
                  <div className="ops-section-head">
                    <div>
                      <h2>购买订单</h2>
                      <p>
                        当前采用线下收款；核对凭证后确认到账，不会自动发起支付。
                      </p>
                    </div>
                    <button onClick={() => open("order")}>
                      代用户创建订单
                    </button>
                  </div>
                  <DataTable
                    columns={[
                      "用户 / 订单",
                      "金额",
                      "到账积分",
                      "状态",
                      "时间",
                      "",
                    ]}
                    rows={data.orders}
                    render={(o) => (
                      <tr key={o.id}>
                        <td>
                          {o.email}
                          <small>{o.id}</small>
                        </td>
                        <td>{money(o.amount_micros)}</td>
                        <td>
                          {o.paid_points + o.bonus_points}
                          <small>赠送 {o.bonus_points}</small>
                        </td>
                        <td>
                          <Badge value={o.status} />
                        </td>
                        <td>{when(o.created_at)}</td>
                        <td>
                          {o.status === "pending" && (
                            <button onClick={() => open("confirm", o)}>
                              确认收款
                            </button>
                          )}
                          {o.status === "credited" && (
                            <button onClick={() => open("refund", o)}>
                              退未使用积分
                            </button>
                          )}
                        </td>
                      </tr>
                    )}
                  />
                </section>
              </>
            )}
            {tab === "audit" && (
              <section className="ops-section">
                <h2>配置与账务审计</h2>
                <DataTable
                  columns={["操作", "对象", "原因 / 凭证", "操作者", "时间"]}
                  rows={data.audit}
                  render={(a) => (
                    <tr key={a.id}>
                      <td>
                        {{
                          image_price: "模型调价",
 video_price: "视频模型调价",
 user_read_only_view: "只读查看用户工作台",
                          package: "套餐修改",
                          purchase_confirm: "订单到账",
                          purchase_refund: "订单退款",
                          supplier_reconcile: "成本核对",
                          statement_import: "账单导入",
                          statement_match: "账单匹配",
                          recover_image: "找回任务",
                        }[a.kind] || a.kind}
                      </td>
                      <td>
                        {a.ref}
                        <details>
                          <summary>修改详情</summary>
                          <pre>
                            {a.before_json} → {a.after_json}
                          </pre>
                        </details>
                      </td>
                      <td>{a.note}</td>
                      <td>{a.actor}</td>
                      <td>{when(a.created_at)}</td>
                    </tr>
                  )}
                />
              </section>
            )}
          </>
        )}
      </main>
      {edit && (
        <dialog
          ref={dialog}
          className="ops-dialog"
          aria-labelledby="ops-dialog-title"
          onCancel={() => setEdit(null)}
        >
          <header>
            <h2 id="ops-dialog-title">
              {
                {
                  price: "配置模型定价",
                  package: "积分套餐与利润预览",
                  confirm: "确认线下收款",
                  refund: "退回未使用积分",
                  reconcile: "核对供应商实际成本",
                  recover: "找回已提交任务",
                  resolve: "确认未生成并退回积分",
                  requests: "供应商请求明细",
                  import: "导入供应商账单",
                  order: "创建购买订单",
                  deposit: "记录供应商充值",
                }[edit.type]
              }
            </h2>
            <button aria-label="关闭" onClick={() => setEdit(null)}>
              <X size={20} />
            </button>
          </header>
          <form onSubmit={submit}>
            <div className="ops-dialog-body">
              {edit.type === "price" && (
                <>
                  <p>
                    {labelModel(edit.row.model)} · {edit.row.tier}
                  </p>
                  <div className="ops-form-grid">
                    <Field
                      label="供应商成本（人民币／张）"
                      name="cost"
                      type="number"
                      min="0.000001"
                      max="1000"
                      step="0.000001"
                      value={draft.cost}
                      onChange={(e) =>
                        setDraft((d) => ({ ...d, cost: e.target.value }))
                      }
                    />
                    <Field
                      label="成本倍数"
                      name="multiplier"
                      type="number"
                      min="0.1"
                      max="100"
                      step="0.01"
                      value={draft.multiplier}
                      onChange={(e) =>
                        setDraft((d) => ({ ...d, multiplier: e.target.value }))
                      }
                    />
                    <Field
                      label="每积分基准价值（元）"
                      name="point"
                      type="number"
                      min="0.000001"
                      max="1"
                      step="0.000001"
                      value={draft.point}
                      onChange={(e) =>
                        setDraft((d) => ({ ...d, point: e.target.value }))
                      }
                    />
                    <Field
                      label="固定积分覆盖（0 为自动）"
                      name="fixed"
                      type="number"
                      min="0"
                      max="1000000"
                      step="1"
                      value={draft.fixed}
                      onChange={(e) =>
                        setDraft((d) => ({ ...d, fixed: e.target.value }))
                      }
                    />
                  </div>
                  <div className="ops-price-preview">
                    <span>提交前报价</span>
                    <strong>
                      {Number.isFinite(predicted) ? predicted : "—"} 积分／张
                    </strong>
                    <small>
                      按成本倍数向上取整；2 倍成本对应基准毛利率
                      50%，其他费用另计。
                    </small>
                  </div>
                  <label className="ops-checkbox">
                    <input
                      name="enabled"
                      type="checkbox"
                      defaultChecked={edit.row.enabled}
                    />
                    已验证可用，启用此档位
                  </label>
                  <Field label="修改原因 / 成本依据" name="note" />
                  <p className="ops-muted">
                    报价只是成本估算，实际支出在供应商账单核对后确认。
                  </p>
                </>
              )}
              {edit.type === "package" && (
                <>
                  <Field
                    label="套餐名称"
                    name="name"
                    value={edit.row.name || ""}
                  />
                  <div className="ops-form-grid">
                    <Field
                      label="售价（人民币）"
                      name="amount"
                      type="number"
                      min="0.01"
                      step="0.01"
                      value={draft.amount}
                      onChange={(e) =>
                        setDraft((d) => ({ ...d, amount: e.target.value }))
                      }
                    />
                    <Field
                      label="付费积分"
                      name="paid"
                      type="number"
                      min="1"
                      step="1"
                      value={draft.paid}
                      onChange={(e) =>
                        setDraft((d) => ({ ...d, paid: e.target.value }))
                      }
                    />
                    <Field
                      label="赠送积分"
                      name="bonus"
                      type="number"
                      min="0"
                      step="1"
                      value={draft.bonus}
                      onChange={(e) =>
                        setDraft((d) => ({ ...d, bonus: e.target.value }))
                      }
                    />
                    <Field
                      label="支付手续费估算（%）"
                      name="fee"
                      type="number"
                      min="0"
                      max="99.99"
                      step="0.01"
                      value={draft.fee}
                      onChange={(e) =>
                        setDraft((d) => ({ ...d, fee: e.target.value }))
                      }
                    />
                  </div>
                  <p>每积分净收入约 {money(packageRate * 1e6)}，赠分已计入。</p>
                  <DataTable
                    columns={[
                      "已启用模型",
                      "每张净收入",
                      "成本覆盖",
                      "预计毛利率",
                    ]}
                    rows={data.prices.filter((p) => p.enabled)}
                    render={(p) => {
                      const n =
                        p.fixed_points ||
                        Math.ceil(
                          (p.cost_micros * p.multiplier_bps) /
                            (10000 * p.point_micros),
                        );
                      const income = n * packageRate * 1e6;
                      return (
                        <tr key={`${p.model}:${p.tier}`}>
                          <td>
                            {labelModel(p.model)} · {p.tier}
                          </td>
                          <td>{money(income)}</td>
                          <td
                            className={
                              income / p.cost_micros < 2 ? "ops-warning" : ""
                            }
                          >
                            {(income / p.cost_micros).toFixed(2)}×
                          </td>
                          <td>
                            {income > 0
                              ? `${(((income - p.cost_micros) / income) * 100).toFixed(1)}%`
                              : "—"}
                          </td>
                        </tr>
                      );
                    }}
                  />
                  <p className="ops-muted">
                    低于 2
                    倍成本的套餐请明确填写折扣原因；预览未包含存储等其他费用。
                  </p>
                  <label className="ops-checkbox">
                    <input
                      name="active"
                      type="checkbox"
                      defaultChecked={edit.row.active}
                    />
                    上架套餐
                  </label>
                  <Field label="发布或修改原因" name="note" />
                </>
              )}
              {["confirm", "refund"].includes(edit.type) && (
                <>
                  {edit.type === "refund" && (
                    <p>
                      可退金额：
                      {money(
                        (edit.row.amount_micros * edit.row.paid_remaining) /
                          edit.row.paid_points,
                      )}{" "}
                      · 收回付费 {edit.row.paid_remaining} 和赠送{" "}
                      {edit.row.bonus_remaining} 积分
                    </p>
                  )}
                  <p>
                    {edit.row.email} · {money(edit.row.amount_micros)} ·{" "}
                    {edit.row.paid_points + edit.row.bonus_points} 积分
                  </p>
                  <p>
                    {edit.type === "confirm"
                      ? "请核实款项已到账；保存后发放积分，重复确认不会再次发分。"
                      : "请先完成线下退款并记录凭证。按剩余付费积分比例退回订单金额，未使用赠分会一并收回；有任务待结算时不可退款。"}
                  </p>
                  <Field label="唯一收款交易号 / 退款凭证" name="receipt" />
                </>
              )}
              {edit.type === "reconcile" && (
                <>
                  <p>
                    任务 {edit.row.upstream_id || edit.row.id} · 预计{" "}
                    {money(edit.row.estimated_micros)}
                  </p>
                  <Field
                    label="供应商实际支出（人民币）"
                    name="cost"
                    type="number"
                    min="0"
                    step="0.000001"
                    value={Number(edit.row.actual_micros || 0) / 1e6}
                  />
                  <Field
                    label="供应商退回金额（人民币）"
                    name="refund"
                    type="number"
                    min="0"
                    step="0.000001"
                    value={Number(edit.row.refund_micros || 0) / 1e6}
                  />
                  <Field
                    label="账单编号与核对依据"
                    name="evidence"
                    value={edit.row.evidence || ""}
                  />
                  <p className="ops-muted">
                    仅更新成本与审计记录，不追加扣除用户积分。
                  </p>
                </>
              )}
              {edit.type === "resolve" && (
                <>
                  <p>
                    仅在供应商已确认没有生成结果后使用。记录实际支出，退回用户积分；不重新发起生成。
                  </p>
                  <Field
                    label="供应商实际支出（人民币，未收费为 0）"
                    name="cost"
                    type="number"
                    min="0"
                    step="0.000001"
                    value="0"
                  />
                  <Field label="供应商核对凭证" name="evidence" />
                </>
              )}
              {edit.type === "recover" && (
                <>
                  <Field label="供应商上游任务 ID" name="upstream_id" />
                  <Field label="核对依据" name="note" />
                  <p>只继续查询已有任务，不重新提交生成。</p>
                </>
              )}
              {edit.type === "requests" && (
                <><UsageParameters item={edit.row} /><DataTable
                  columns={["请求", "状态", "时间", "结果"]}
                  rows={requests}
                  render={(r) => (
                    <tr key={r.id}>
                      <td>
                        {{ submit: "提交", query: "查询", download: "下载" }[
                          r.kind
                        ] || r.kind}
                      </td>
                      <td>{r.status_code || "—"}</td>
                      <td>{when(r.created_at)}</td>
                      <td>{r.error || "请求成功"}</td>
                    </tr>
                  )}
                />
                </>
              )}
              {edit.type === "import" && (
                <>
                  <p>
                    上传 CSV
                    账单，先检查预览，再确认导入。相同账号与账单编号不会重复入库。
                  </p>
                  <button type="button" onClick={downloadStatementTemplate}>
                    下载账单模板
                  </button>
                  <label className="ops-field">
                    <span>选择供应商账单</span>
                    <input
                      type="file"
                      accept=".csv,text/csv"
                      required
                      onChange={async (e) => {
                        try {
                          const file = e.target.files?.[0];
                          if (!file) return;
                          if (file.size > 2 * 1024 * 1024)
                            throw new Error("账单需小于 2MB");
                          const lines = parseStatementCSV(await file.text());
                          setImportLines(lines);
                          setImportText(file.name);
                          setError("");
                        } catch (err) {
                          setImportLines([]);
                          setError(err.message);
                        }
                      }}
                    />
                  </label>
                  <p className="ops-muted">
                    按模板填写人民币金额与供应商账号标识。金额使用元，支持最多 6
                    位小数；账号需与调用记录一致。
                  </p>
                  {importLines.length > 0 && (
                    <>
                      <div className="ops-price-preview">
                        <strong>
                          {importText} · {importLines.length} 行
                        </strong>
                        <span>
                          供应商支出合计{" "}
                          {money(
                            importLines.reduce(
                              (n, r) => n + r.amount_micros,
                              0,
                            ),
                          )}
                        </span>
                      </div>
                      <DataTable
                        columns={[
                          "账单编号",
                          "账号",
                          "上游任务",
                          "支出",
                          "退款",
                        ]}
                        rows={importLines.slice(0, 10)}
                        render={(r, i) => (
                          <tr key={i}>
                            <td>{r.external_id}</td>
                            <td>{r.account}</td>
                            <td>{r.task_id}</td>
                            <td>{money(r.amount_micros)}</td>
                            <td>{money(r.refund_micros)}</td>
                          </tr>
                        )}
                      />
                      <p>预览前 10 行，确认后导入全部记录。</p>
                    </>
                  )}
                </>
              )}
              {edit.type === "order" && (
                <>
                  <Field label="用户">
                    <select name="user" required>
                      <option value="">选择托管用户</option>
                      {accounts
                        .filter((a) => a.credit.managed_enabled)
                        .map((a) => (
                          <option key={a.user.id} value={a.user.id}>
                            {a.user.email}
                          </option>
                        ))}
                    </select>
                  </Field>
                  <Field label="套餐">
                    <select name="package" required>
                      <option value="">选择上架套餐</option>
                      {data.packages
                        .filter((p) => p.active)
                        .map((p) => (
                          <option key={p.id} value={p.id}>
                            {p.name} · {money(p.amount_micros)}
                          </option>
                        ))}
                    </select>
                  </Field>
                </>
              )}
              {edit.type === "deposit" && (
                <>
                  <Field label="供应商账号标识" name="account" />
                  <Field
                    label="充值金额（人民币）"
                    name="amount"
                    type="number"
                    min="0.01"
                    step="0.01"
                  />
                  <Field
                    label="赠送额度（人民币）"
                    name="bonus"
                    type="number"
                    min="0"
                    step="0.01"
                    value="0"
                  />
                  <Field label="充值凭证" name="receipt" />
                </>
              )}
              {error && (
                <p className="error-banner" role="alert">
                  {error}
                </p>
              )}
            </div>
            <footer>
              <button type="button" onClick={() => setEdit(null)}>
                关闭
              </button>
              {edit.type !== "requests" && (
                <button
                  className="ops-primary"
                  type="submit"
                  disabled={
                    busy || (edit.type === "import" && !importLines.length)
                  }
                >
                  {busy
                    ? "保存中…"
                    : edit.type === "import"
                      ? "确认导入"
                      : edit.type === "confirm"
                        ? "确认已收款并发分"
                        : "保存"}
                </button>
              )}
            </footer>
          </form>
        </dialog>
      )}
    </div>
  );
}
function csvRows(text) {
  const rows = [];
  let row = [],
    cell = "",
    quoted = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (c === '"') {
      if (quoted && text[i + 1] === '"') {
        cell += '"';
        i++;
      } else quoted = !quoted;
    } else if (c === "," && !quoted) {
      row.push(cell);
      cell = "";
    } else if ((c === "\n" || c === "\r") && !quoted) {
      if (c === "\r" && text[i + 1] === "\n") i++;
      row.push(cell);
      if (row.some((x) => x.trim())) rows.push(row);
      row = [];
      cell = "";
    } else cell += c;
  }
  if (quoted) throw new Error("账单中存在未闭合引号");
  row.push(cell);
  if (row.some((x) => x.trim())) rows.push(row);
  return rows;
}
function parseStatementCSV(text) {
  const rows = csvRows(text.replace(/^\uFEFF/, ""));
  const headers = [
    "供应商账号",
    "账单编号",
    "上游任务ID",
    "实际支出元",
    "退款元",
    "凭证",
  ];
  if (!rows.length || headers.some((h, i) => rows[0][i]?.trim() !== h))
    throw new Error("请使用账单模板中的六列标题");
  if (rows.length < 2 || rows.length > 501)
    throw new Error("单批账单需为 1–500 行");
  return rows.slice(1).map((row, i) => {
    if (row.length !== 6 || !row[0].trim() || !row[1].trim() || !row[5].trim())
      throw new Error(`第 ${i + 2} 行缺少账号、编号或凭证`);
    for (const index of [3, 4]) {
      if (!/^\d+(\.\d{1,6})?$/.test(row[index].trim() || "0"))
        throw new Error(`第 ${i + 2} 行金额格式无效`);
    }
    const amount = micros(row[3] || 0),
      refund = micros(row[4] || 0);
    if (refund > amount) throw new Error(`第 ${i + 2} 行退款超过支出`);
    return {
      account: row[0].trim(),
      external_id: row[1].trim(),
      task_id: row[2].trim(),
      amount_micros: amount,
      refund_micros: refund,
      evidence: row[5].trim(),
    };
  });
}
function downloadStatementTemplate() {
  const text =
    "\uFEFF供应商账号,账单编号,上游任务ID,实际支出元,退款元,凭证\r\n";
  const url = URL.createObjectURL(
    new Blob([text], { type: "text/csv;charset=utf-8" }),
  );
  const a = document.createElement("a");
  a.href = url;
  a.download = "供应商账单模板.csv";
  a.click();
  URL.revokeObjectURL(url);
}
