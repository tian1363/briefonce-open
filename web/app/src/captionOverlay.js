export async function captionOverlay(text, coverExisting = false) {
  const canvas = document.createElement("canvas");
  canvas.width = 720;
  canvas.height = 1280;
  const ctx = canvas.getContext("2d");
  ctx.font = "bold 44px -apple-system, BlinkMacSystemFont, 'Noto Sans CJK SC', sans-serif";
  const lines = [];
  for (const paragraph of text.trim().split(/\n+/)) {
    let line = "";
    for (const char of Array.from(paragraph)) {
      if (line && ctx.measureText(line + char).width > 610) { lines.push(line); line = char; }
      else line += char;
    }
    if (line) lines.push(line);
  }
  if (!lines.length || lines.length > 5) throw new Error("每版字幕最多显示 5 行，请精简文案。");
  const height = 150 + lines.length * 62;
  const top = canvas.height - height;
  if (coverExisting) {
    ctx.fillStyle = "#000";
    ctx.fillRect(0, 990, 720, 290);
  } else {
    const gradient = ctx.createLinearGradient(0, top - 90, 0, canvas.height);
    gradient.addColorStop(0, "rgba(0,0,0,0)");
    gradient.addColorStop(0.38, "rgba(0,0,0,0.7)");
    gradient.addColorStop(1, "rgba(0,0,0,0.84)");
    ctx.fillStyle = gradient;
    ctx.fillRect(0, top - 90, 720, height + 90);
  }
  ctx.fillStyle = "#fff";
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.shadowColor = "rgba(0,0,0,0.75)";
  ctx.shadowBlur = 7;
  lines.forEach((line, index) => ctx.fillText(line, 360, top + 40 + index * 62));
  return new Promise((resolve, reject) => canvas.toBlob((blob) => blob ? resolve(blob) : reject(new Error("无法制作字幕图层")), "image/png"));
}
