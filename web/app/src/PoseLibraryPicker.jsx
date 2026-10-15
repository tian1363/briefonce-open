import CanvasSelect from "./CanvasSelect.jsx";
import { useState } from "react";
import { filterPoses, poseDimensions, poseValues } from "./poseCatalog.js";
import "./PoseLibraryPicker.css";

export default function PoseLibraryPicker({ presets, selected, onToggle, compact = true }) {
  const [query, setQuery] = useState("");
  const [filters, setFilters] = useState({});
  const [page, setPage] = useState(0);
  const [selectedOnly, setSelectedOnly] = useState(false);
  const matches = filterPoses(presets, query, filters).filter((pose) => !selectedOnly || selected.includes(pose.id));
  const size = compact ? 12 : 24;
  const lastPage = Math.max(0, Math.ceil(matches.length / size) - 1);
  const current = Math.min(page, lastPage);
  const visible = matches.slice(current * size, (current + 1) * size);
  return <div className={`pose-library-picker nodrag nopan nowheel ${compact ? "compact" : ""}`}>
    <label>查找姿势<input type="search" value={query} onChange={(event) => { setQuery(event.target.value); setPage(0); }} placeholder="编号、动作、服装或作者" /></label>
    <div className="pose-library-filters">{poseDimensions.map(([key, label]) => <label key={key}>{label}<CanvasSelect value={filters[key] || ""} onChange={(event) => { setFilters({ ...filters, [key]: event.target.value }); setPage(0); }}><option value="">全部{label}</option>{[...new Set(presets.flatMap((pose) => poseValues(pose, key)))].map((value) => <option key={value}>{value}</option>)}</CanvasSelect></label>)}</div>
    <div className="pose-library-summary"><span aria-live="polite">{matches.length}/{presets.length} 个 · 已选 {selected.length}/6</span><button type="button" aria-pressed={selectedOnly} onClick={() => { setSelectedOnly(!selectedOnly); setPage(0); }}>只看已选</button><button type="button" onClick={() => { setQuery(""); setFilters({}); setSelectedOnly(false); setPage(0); }}>清除筛选</button><button type="button" disabled={!selected.length} onClick={() => { selected.forEach(onToggle); setSelectedOnly(false); }}>清空已选</button></div>
    <div className="pose-library-grid" role="group" aria-label="选择生成姿势">{visible.map((pose) => <article key={pose.id} className={selected.includes(pose.id) ? "selected" : ""}>
      <button type="button" className="pose-library-choice" aria-pressed={selected.includes(pose.id)} disabled={!selected.includes(pose.id) && selected.length >= 6} onClick={() => onToggle(pose.id)}>
        {pose.image_url ? <img src={pose.image_url} alt={pose.name} loading="lazy" /> : <div className="pose-library-placeholder">暂无本地预览<br />可按动作说明选择</div>}
        <small>{pose.id} · {pose.preview_status}</small><strong>{pose.name}</strong><span>{pose.shot} · {pose.gender}</span><span>{pose.style} · {pose.use}</span>
      </button>
      <details><summary>动作与来源</summary><p>{pose.instruction}</p><p>{pose.source_author} · {pose.source_url_type}</p>{pose.source_url && <a href={pose.source_url} target="_blank" rel="noreferrer">{pose.source_url_type === "搜索结果页" ? "查看来源搜索页" : "查看来源帖子"}</a>}</details>
    </article>)}</div>
    {!matches.length && <p>没有匹配的姿势，请调整筛选。</p>}
    <div className="pose-library-summary"><button type="button" disabled={!current} onClick={() => setPage(current - 1)}>上一页</button><span>{current + 1}/{lastPage + 1}</span><button type="button" disabled={current >= lastPage} onClick={() => setPage(current + 1)}>下一页</button></div>
    {!!selected.length && <details><summary>已选动作 · 查看或移除</summary>{selected.map((id) => { const pose = presets.find((item) => item.id === id); return <div key={id}><strong>{pose?.name || id}</strong><p>{pose?.instruction}</p><button type="button" onClick={() => onToggle(id)}>移除此动作</button></div>; })}</details>}
    <p>图片为低清参考预览；生成使用动作文字、所选模特图和商品图。</p>
  </div>;
}
