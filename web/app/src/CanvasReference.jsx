import CanvasSelect from "./CanvasSelect.jsx";
import CanvasTextField from "./CanvasTextField.jsx";
import { useRef, useState } from "react";
import { ImagePlus, Loader2 } from "lucide-react";
import CanvasToggle from "./CanvasToggle.jsx";
import DownloadableImage from "./DownloadableImage.jsx";
import { uploadCanvasReference } from "./api.js";

export const REFERENCE_ASPECTS = ["配色", "光线", "构图", "留白", "质感", "排版节奏"];

export function referenceFocus(aspects = [], note = "") {
  return [...aspects, note.trim()].filter(Boolean).join("；").slice(0, 220);
}

export default function CanvasReference({ productId, references, value, onChange, onUploaded, onError, heading = "视觉参考图", aspects = REFERENCE_ASPECTS, hint = "参考图只影响所选视觉特征，商品事实仍以商品原图和已确认资料为准。" }) {
  const input = useRef(null);
  const [uploading, setUploading] = useState(false);
  const selected = references.find((item) => item.id === value.assetId);
  async function upload(file) {
    if (!file) return;
    if (!["image/jpeg", "image/png"].includes(file.type) || file.size > 10 * 1024 * 1024) { onError("参考图需为 10MB 以内的 JPG 或 PNG"); return; }
    setUploading(true);
    try {
      const asset = await uploadCanvasReference(productId, file);
      onUploaded(asset);
      onChange({ ...value, assetId: asset.id });
      onError("");
    } catch (error) { onError(error.message); }
    finally { setUploading(false); if (input.current) input.current.value = ""; }
  }
  return <div className="workflow-reference">
    <label className="workflow-field">{heading}<CanvasSelect className="nodrag" value={value.assetId} getOptionImage={id => id ? `/api/assets/${id}/file` : ""} onChange={(event) => onChange({ ...value, assetId: event.target.value })}><option value="">不使用参考图</option>{references.map((item) => <option key={item.id} value={item.id}>{item.original_name}</option>)}</CanvasSelect></label>
    <input ref={input} type="file" accept="image/jpeg,image/png" hidden onChange={(event) => upload(event.target.files?.[0])} />
    <button type="button" className="workflow-text-button nodrag" disabled={uploading} onClick={() => input.current?.click()}>{uploading ? <Loader2 size={14} className="spin" /> : <ImagePlus size={14} />}{uploading ? "上传中…" : "上传参考图片"}</button>
    {selected && <>
      <DownloadableImage className="workflow-preview" src={`/api/assets/${selected.id}/file`} alt={selected.original_name} filename={selected.original_name} />
      <span className="workflow-field">要参考的部分</span>
      <div className="workflow-reference-aspects">{aspects.map((aspect) => <CanvasToggle key={aspect} checked={value.aspects.includes(aspect)} onChange={() => onChange({ ...value, aspects: value.aspects.includes(aspect) ? value.aspects.filter((item) => item !== aspect) : [...value.aspects, aspect] })}>{aspect}</CanvasToggle>)}</div>
      <label className="workflow-field">补充说明（选填）<CanvasTextField multiline rows={2} maxLength={160} value={value.note} onValueChange={(note) => onChange({ ...value, note })} placeholder="例如：只参考右侧留白，不参考图中的商品" /></label>
      <p>{hint}</p>
    </>}
  </div>;
}
