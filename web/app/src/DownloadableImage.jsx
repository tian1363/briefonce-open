import { trackEvent } from "./analytics.js";
import { ImageCreditBadge } from "./CreditUsage.jsx";
import { useContext, useState } from "react";
import { CanvasPreviewContext } from "./CanvasImagePreview.jsx";
import { Download } from "lucide-react";
import "./DownloadableImage.css";

export default function DownloadableImage({ src, alt, filename = "", className = "", wrapperClassName = "", loading, onPreview, previewClassName = "", localCredit = false }) {
  const canvasPreview = useContext(CanvasPreviewContext);
  const [downloadStarted, setDownloadStarted] = useState(false);
  const preview = onPreview || (canvasPreview ? () => canvasPreview(src) : null);
  const image = <img className={className} src={src} alt={alt} loading={loading} />;
  return <div className={`downloadable-image ${wrapperClassName}`.trim()}>
    {preview ? <button type="button" className={`downloadable-image-preview nodrag nopan ${previewClassName}`.trim()} onPointerDown={(event) => event.stopPropagation()} onClick={() => { if (onPreview && !canvasPreview) trackEvent("result_preview", "other", "library"); preview(); }} aria-label={`预览 ${alt}`}>{image}</button> : image}
    <a
      className="downloadable-image-action nodrag nopan"
      href={src.startsWith("/api/") ? `${src}${src.includes("?") ? "&" : "?"}download=1&filename=${encodeURIComponent(filename || alt)}` : src}
      download={filename}
      aria-label={`下载图片：${alt}`}
      title="下载图片"
      onPointerDown={(event) => event.stopPropagation()}
      onClick={(event) => { event.stopPropagation(); setDownloadStarted(true); }}
    ><Download size={16} aria-hidden="true" /><span>下载</span></a>
    {downloadStarted && <div className="downloadable-image-notice nodrag nopan" role="status" onPointerDown={(event) => event.stopPropagation()} onClick={(event) => event.stopPropagation()}>
      <span>已发起下载，请查看浏览器下载记录。</span>
      <a href={src} target="_blank" rel="noopener noreferrer">打开原图</a>
      <button type="button" onClick={() => setDownloadStarted(false)} aria-label="关闭下载提示">×</button>
    </div>}
    <ImageCreditBadge src={src} local={localCredit} />
  </div>;
}
