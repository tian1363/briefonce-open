import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";
import PoseLibraryPicker from "./PoseLibraryPicker.jsx";
import "./CanvasPosePicker.css";

function PoseDialog({ presets, selected, onToggle, onClose }) {
  const dialog = useRef(null);
  useEffect(() => {
    const element = dialog.current;
    const previous = document.activeElement;
    element.showModal();
    return () => { element.close(); previous?.focus(); };
  }, []);
  return createPortal(<dialog ref={dialog} className="canvas-pose-dialog" aria-labelledby="canvas-pose-title" onCancel={event => { event.preventDefault(); onClose(); }} onClose={onClose} onClick={event => { event.stopPropagation(); if (event.target === event.currentTarget) onClose(); }} onPointerDown={event => event.stopPropagation()} onKeyDown={event => event.stopPropagation()} onWheel={event => event.stopPropagation()}>
    <div className="canvas-pose-dialog-body">
      <header><div><h2 id="canvas-pose-title">姿势库</h2><p>选择最多 6 个姿势，选择会自动保留。</p></div><button type="button" onClick={onClose} aria-label="关闭姿势库"><X size={20} /></button></header>
      <div className="canvas-pose-dialog-content"><PoseLibraryPicker presets={presets} selected={selected} onToggle={onToggle} compact={false} /></div>
      <footer><span aria-live="polite">已选 {selected.length}/6 个姿势</span><button type="button" className="workflow-run" onClick={onClose}>完成选择</button></footer>
    </div>
  </dialog>, document.body);
}

export default function CanvasPosePicker({ presets, selected, onToggle }) {
  const [open, setOpen] = useState(false);
  return <section className="canvas-pose-selection nodrag nopan nowheel" aria-label="已选姿势">
    <div className="canvas-pose-selection-header"><button type="button" onClick={() => setOpen(true)}>选择姿势</button><span aria-live="polite">已选 {selected.length}/6</span></div>
    {selected.length ? <div className="canvas-pose-selected-grid">{selected.map(id => { const pose = presets.find(item => item.id === id); return <figure key={id}>
      {pose?.image_url && <img src={pose.image_url} alt={pose.name} loading="lazy" />}
      <figcaption title={pose?.name}>{pose?.name || id}</figcaption>
      <button type="button" onClick={() => onToggle(id)} aria-label={`移除姿势：${pose?.name || id}`}><X size={14} /></button>
    </figure>; })}</div> : <p>打开姿势库，搜索并选择动作。</p>}
    {open && <PoseDialog presets={presets} selected={selected} onToggle={onToggle} onClose={() => setOpen(false)} />}
  </section>;
}
