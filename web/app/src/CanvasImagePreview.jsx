import { trackEvent } from "./analytics.js";
import { createContext, useContext, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { ChevronLeft, ChevronRight, Download, Maximize2, X } from "lucide-react";
import "./CanvasImagePreview.css";

export const CanvasPreviewContext = createContext(null);

export function CanvasPreviewButton({ src, label = "图片", className = "canvas-preview-trigger", children }) {
  const open = useContext(CanvasPreviewContext);
  return <button type="button" className={`${className} nodrag nopan`} aria-label={`放大预览 ${label}`} title="放大预览" onPointerDown={(event) => event.stopPropagation()} onClick={(event) => { event.stopPropagation(); open?.(src); }}>{children || <Maximize2 size={14} />}</button>;
}

export default function CanvasImagePreview({ images, initialIndex, onClose }) {
  const dialog = useRef(null);
  const viewport = useRef(null);
  const [index, setIndex] = useState(initialIndex);
  const [actualSize, setActualSize] = useState(false);
  const [failed, setFailed] = useState(false);
  const item = images[index];
  useEffect(() => { trackEvent("result_preview", "other", "library"); }, [index]);
  useEffect(() => {
    const previous = document.activeElement;
    dialog.current.showModal();
    return () => { previous?.focus?.(); };
  }, []);
  useEffect(() => { setFailed(false); setActualSize(false); viewport.current?.scrollTo(0, 0); }, [index]);
  const move = (delta) => setIndex((value) => Math.max(0, Math.min(images.length - 1, value + delta)));
  return createPortal(<dialog ref={dialog} className="canvas-image-dialog" aria-label="画布图片预览" onCancel={(event) => { event.preventDefault(); onClose(); }} onClick={(event) => { if (event.target === dialog.current) onClose(); }} onKeyDown={(event) => {
    if (event.key === "Escape") { event.preventDefault(); onClose(); }
    if (event.key === "ArrowLeft" || event.key === "ArrowRight") { event.preventDefault(); move(event.key === "ArrowLeft" ? -1 : 1); }
  }}>
    <div className="canvas-image-viewer">
      <header><div><strong>{item.alt || "图片预览"}</strong><span aria-live="polite">{index + 1} / {images.length}</span></div><nav aria-label="图片操作"><button type="button" onClick={() => setActualSize((value) => !value)}>{actualSize ? "适应窗口" : "原尺寸"}</button><a href={item.src} download={item.filename || "图片"} aria-label="下载当前图片"><Download size={19} /></a><button type="button" aria-label="关闭图片预览" onClick={onClose} autoFocus><X size={22} /></button></nav></header>
      <div className="canvas-image-content"><button type="button" className="canvas-image-arrow" aria-label="上一张图片" disabled={index === 0} onClick={() => move(-1)}><ChevronLeft size={30} /></button><div ref={viewport} className={`canvas-image-viewport ${actualSize ? "actual-size" : ""}`}>{failed ? <p role="alert">图片加载失败，请关闭后重试。</p> : <img key={item.src} src={item.src} alt={item.alt || "画布图片"} onError={() => setFailed(true)} />}</div><button type="button" className="canvas-image-arrow" aria-label="下一张图片" disabled={index === images.length - 1} onClick={() => move(1)}><ChevronRight size={30} /></button></div>
      <footer>← → 切换图片 · Esc 关闭</footer>
    </div>
  </dialog>, document.body);
}
