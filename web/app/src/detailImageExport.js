export function containRect(sourceWidth, sourceHeight, width, height) {
  const scale = Math.min(width / sourceWidth, height / sourceHeight);
  const w = sourceWidth * scale, h = sourceHeight * scale;
  return { x: (width - w) / 2, y: (height - h) / 2, width: w, height: h };
}
export async function exportDetailImages(jobs, title, { prepareOnly = false } = {}) {
  const { default: JSZip } = await import("jszip");
  const zip = new JSZip();
  for (const [i, job] of jobs.entries()) {
    const response = await fetch(`/api/assets/${job.output_asset_id}/file`);
    if (!response.ok) throw new Error("图片读取失败，请重试下载");
    const image = await createImageBitmap(await response.blob());
    const { width, height, platform_id, page_id } = job.task;
    const canvas = document.createElement("canvas"); canvas.width = width; canvas.height = height;
    const ctx = canvas.getContext("2d"); ctx.fillStyle = "#ffffff"; ctx.fillRect(0, 0, width, height);
    const r = containRect(image.width, image.height, width, height); ctx.drawImage(image, r.x, r.y, r.width, r.height); image.close();
    let blob; const limit = platform_id === "amazon" ? 2 * 1024 * 1024 : 5 * 1024 * 1024;
    for (const quality of [.94, .86, .76, .65]) { blob = await new Promise(resolve => canvas.toBlob(resolve, "image/jpeg", quality)); if (blob && blob.size <= limit) break; }
    if (!blob || blob.size > limit) throw new Error("图片无法满足文件大小限制，请单独处理后上传");
    zip.file(`${platform_id}/${String(i + 1).padStart(2, "0")}-${page_id}.jpg`, blob);
  }
  const blob = await zip.generateAsync({ type: "blob" }); const url = URL.createObjectURL(blob);
  const filename = `${title || "商品"}-详情图片.zip`;
  if (prepareOnly) return { url, filename };
  const link = document.createElement("a"); link.href = url; link.download = filename; document.body.appendChild(link); link.click(); link.remove(); setTimeout(() => URL.revokeObjectURL(url), 30000);
}
