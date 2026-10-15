import { useEffect, useRef, useState } from "react";
import { ChevronDown, Package } from "lucide-react";
import { listProductAssets } from "./api.js";
import "./ProductPicker.css";

export function ProductThumbnail({ productId, className = "" }) {
  const [imageId, setImageId] = useState("");
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    setImageId("");
    setFailed(false);
    if (!productId) return undefined;
    let active = true;
    listProductAssets(productId)
      .then((items) => {
        if (!active) return;
        const image = (Array.isArray(items) ? items : []).find((item) => item.kind === "image");
        setImageId(image?.id || "");
      })
      .catch(() => { if (active) setImageId(""); });
    return () => { active = false; };
  }, [productId]);

  return <span className={`product-picker-thumbnail ${className}`} aria-hidden="true">
    {imageId && !failed ? <img src={`/api/assets/${imageId}/file`} alt="" onError={() => setFailed(true)} /> : <Package size={18} />}
  </span>;
}

export function ProductChoices({ products, selectedId, onSelect, includeNone = true }) {
  return <div className="product-connector-choices product-picker-choices" role="group" aria-label="选择产品">
    {includeNone && <button type="button" aria-pressed={!selectedId} onClick={(event) => { onSelect(""); if (event.currentTarget.closest("details")) event.currentTarget.closest("details").open = false; }}><ProductThumbnail /><span>不连接产品</span></button>}
    {products.map((product) => <button
      key={product.id}
      type="button"
      aria-pressed={selectedId === product.id}
      onClick={(event) => {
        onSelect(product.id);
        const details = event.currentTarget.closest("details");
        if (details) details.open = false;
      }}
    ><ProductThumbnail productId={product.id} /><span>{product.title}</span></button>)}
  </div>;
}

export default function ProductPicker({ products, value, onChange, label = "产品资料" }) {
  const selected = products.find((product) => product.id === value);
  const detailsRef = useRef(null);
  const [open, setOpen] = useState(false);
  useEffect(() => {
    function dismiss(event) {
      if (detailsRef.current?.open && !detailsRef.current.contains(event.target)) detailsRef.current.open = false;
    }
    function dismissOnEscape(event) {
      if (event.key === "Escape" && detailsRef.current?.open) { detailsRef.current.open = false; detailsRef.current.querySelector("summary")?.focus(); }
    }
    document.addEventListener("pointerdown", dismiss, true);
    document.addEventListener("keydown", dismissOnEscape);
    return () => { document.removeEventListener("pointerdown", dismiss, true); document.removeEventListener("keydown", dismissOnEscape); };
  }, []);
  return <div className="product-picker">
    <span className="product-picker-label">{label}</span>
    <details className="product-picker-details" ref={detailsRef} onToggle={(event) => setOpen(event.currentTarget.open)}>
      <summary aria-label={`${label}：${selected?.title || "选择产品"}`}><ProductThumbnail productId={selected?.id} /><span>{selected?.title || "选择产品"}</span><ChevronDown size={17} aria-hidden="true" /></summary>
      {open && <div className="product-picker-popover">{products.length ? <ProductChoices products={products} selectedId={value} onSelect={onChange} includeNone={false} /> : <p>暂无产品资料</p>}</div>}
    </details>
  </div>;
}
