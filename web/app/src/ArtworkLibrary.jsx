import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { X, RefreshCw } from "lucide-react";
import { listArtworks } from "./api.js";
import CanvasImagePreview from "./CanvasImagePreview.jsx";
import DownloadableImage from "./DownloadableImage.jsx";
import "./ArtworkLibrary.css";

const sources = { pose: "姿势图", ai: "模特图", edit: "编辑图", "product-photo": "商品精修", detail: "详情排版" };
export default function ArtworkLibrary({ products, productId, onClose, onUse }) {
  const dialog = useRef(null);
  const [filter, setFilter] = useState(productId || "");
  const [items, setItems] = useState([]);
  const [more, setMore] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [preview, setPreview] = useState(null);
  const [refresh, setRefresh] = useState(0);
  const requestID = useRef(0);
  useEffect(() => { const previous = document.activeElement; dialog.current.showModal(); return () => { requestID.current++; previous?.focus?.(); }; }, []);
  async function load(offset = 0) {
    const id = ++requestID.current;
    setBusy(true); setError("");
    try {
      const result = await listArtworks(filter, offset);
      if (id !== requestID.current) return;
      setItems((current) => offset ? [...current, ...result.items] : result.items);
      setMore(result.has_more);
    } catch (err) { if (id === requestID.current) setError(err.message); }
    finally { if (id === requestID.current) setBusy(false); }
  }
  useEffect(() => { setItems([]); setMore(false); load(); }, [filter, refresh]);
  return createPortal(<dialog ref={dialog} className="artwork-library" aria-label="作品库" onCancel={(event) => { event.preventDefault(); onClose(); }}>
    <header><div><h2>作品库</h2><p>历史生成图片自动保存，重置画布视图不会删除作品。</p></div><button type="button" aria-label="关闭作品库" onClick={onClose}><X size={22} /></button></header>
    <div className="artwork-library-toolbar"><label>按产品筛选<select value={filter} onChange={(event) => setFilter(event.target.value)}><option value="">全部产品</option>{products.map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}<option value="unassigned">未关联产品</option></select></label><button type="button" disabled={busy} onClick={() => setRefresh((value) => value + 1)}><RefreshCw size={16} />刷新</button></div>
    <div className="artwork-library-body">
      {error && <p role="alert">加载失败：{error}<button onClick={() => load(items.length)}>重试</button></p>}
      {!busy && !error && !items.length && <p className="artwork-library-empty">该产品还没有作品。完成生成后，图片会自动收录在这里。</p>}
      <div className="artwork-library-grid">{items.map((item, index) => <article key={item.id}>
        <DownloadableImage src={item.url} alt={item.name} filename={item.name} loading="lazy" localCredit={item.source === "detail"} onPreview={() => setPreview(index)} />
        <strong title={item.name}>{item.name}</strong><span>{item.product_title || "未关联产品"} · {sources[item.source] || "生成图片"}</span><small>{new Date(item.created_at).toLocaleString("zh-CN")}</small>
        {item.asset_id && item.source !== "detail" && <button type="button" onClick={() => onUse(item)}>用于详情页</button>}
      </article>)}</div>
      {busy && <p role="status">正在加载作品…</p>}
      {more && <button className="artwork-library-more" disabled={busy} onClick={() => load(items.length)}>加载更多</button>}
    </div>
    {preview !== null && <CanvasImagePreview images={items.map((item) => ({ src: item.url, alt: item.name, filename: item.name }))} initialIndex={preview} onClose={() => setPreview(null)} />}
  </dialog>, document.body);
}
