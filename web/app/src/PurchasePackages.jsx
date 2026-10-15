import { useEffect, useState } from "react";
import {
  listCreditPackages,
  listPurchaseOrders,
  createPurchaseOrder,
} from "./api.js";
export default function PurchasePackages() {
  const [packages, setPackages] = useState([]),
    [orders, setOrders] = useState([]),
    [busy, setBusy] = useState(""),
    [error, setError] = useState("");
  async function refresh() {
    const [p, o] = await Promise.all([
      listCreditPackages(),
      listPurchaseOrders(),
    ]);
    setPackages(p);
    setOrders(o);
  }
  useEffect(() => {
    let active = true;
    Promise.all([listCreditPackages(), listPurchaseOrders()])
      .then(([p, o]) => {
        if (active) {
          setPackages(p);
          setOrders(o);
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, []);
  async function purchase(id) {
    setBusy(id);
    setError("");
    try {
      await createPurchaseOrder(id);
      await refresh();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy("");
    }
  }
  return (
    <section className="purchase-packages">
      <h2>购买积分</h2>
      <p>
        创建订单后联系管理员线下付款，收款确认后到账。目前不会自动发起支付。
      </p>
      {error && (
        <p role="alert" className="error-banner">
          {error}
        </p>
      )}
      <div className="purchase-package-grid">
        {packages.map((p) => (
          <article key={p.id}>
            <strong>{p.name}</strong>
            <h3>¥{(p.amount_micros / 1e6).toFixed(2)}</h3>
            <p>
              {p.paid_points + p.bonus_points} 积分
              {p.bonus_points ? `（含赠送 ${p.bonus_points}）` : ""}
            </p>
            <button
              className="secondary-button"
              disabled={!!busy}
              onClick={() => purchase(p.id)}
            >
              {busy === p.id ? "创建中…" : "创建购买订单"}
            </button>
          </article>
        ))}
      </div>
      {!packages.length && <p>暂无上架套餐，请联系管理员。</p>}
      <details>
        <summary>我的购买订单（{orders.length}）</summary>
        {orders.map((o) => (
          <div className="purchase-order" key={o.id}>
            <span>
              {o.id}
              <small>{new Date(o.created_at).toLocaleString("zh-CN")}</small>
            </span>
            <strong>¥{(o.amount_micros / 1e6).toFixed(2)}</strong>
            <span>
              {{
                pending: "待线下付款",
                credited: "已到账",
                refunded: "已退款",
                partially_refunded: "部分退款",
              }[o.status] || o.status}
            </span>
          </div>
        ))}
      </details>
    </section>
  );
}
