export const MAX_VERSIONS = 30;
export const MODE_LABELS = { model: "换模特", copy: "换文案", product: "换产品" };
export function createVersion(patch = {}) {
  return { id: crypto.randomUUID(), selected: true, title: "", target: null, notes: "", text: "", segments: [], coverExisting: false, scope: "subtitle", name: "", claims: "", adapt: true, ...patch };
}
export function initialVersions() {
  return Object.fromEntries(Object.keys(MODE_LABELS).map((mode) => [mode, [createVersion()]]));
}
export function versionIssues(mode, version) {
  const issues = [];
  if (mode !== "copy" && !version.target) issues.push(mode === "model" ? "缺模特图" : "缺商品图");
  const timed = mode === "copy" && (version.segments?.length || 0) > 0;
  if (mode === "copy" && !timed && !version.text.trim()) issues.push("缺文案或逐句字幕");
  if (mode === "copy" && !timed && Array.from(version.text.trim()).length > 60) issues.push("字幕超过 60 字");
  if (mode === "copy" && !timed && version.text.trim().split(/\n+/).length > 5) issues.push("字幕超过 5 行");
  if (timed && version.segments.length > 20) issues.push("字幕超过 20 句");
  if (timed && version.segments.some((segment) => segment.needs_review)) issues.push("有待核对的识别句子");
  if (timed && version.segments.some((segment, index) => !segment.text?.trim() || Array.from(segment.text.trim()).length > 60 || !Number.isFinite(Number(segment.start)) || !Number.isFinite(Number(segment.end)) || Number(segment.start) < 0 || Number(segment.end) <= Number(segment.start) || (index > 0 && Number(segment.start) < Number(version.segments[index - 1].end)))) issues.push("逐句时间或文案有误");
  if (mode === "product" && version.adapt && !version.text.trim()) issues.push("缺商品文案");
  return issues;
}
function fingerprint(mode, row) {
  const media = row.target?.id || (row.target?.file ? `${row.target.file.name}:${row.target.file.size}:${row.target.file.lastModified}` : "");
  const clean = (value = "") => value.trim().replace(/\s+/g, " ");
  if (mode === "model") return JSON.stringify([media, clean(row.notes)]);
  if (mode === "copy") return JSON.stringify([row.segments?.length ? row.segments.map((s) => [s.start, s.end, clean(s.text)]) : clean(row.text), row.coverExisting]);
  return JSON.stringify([media, clean(row.name), clean(row.claims), row.adapt, row.adapt ? clean(row.text) : ""]);
}
export function batchSummary(mode, rows) {
  const selected = rows.filter((row) => row.selected);
  const complete = selected.filter((row) => !versionIssues(mode, row).length);
  const seen = new Map();
  const duplicateIds = new Set();
  for (const row of complete) {
    const key = fingerprint(mode, row);
    if (seen.has(key)) { duplicateIds.add(row.id); duplicateIds.add(seen.get(key)); }
    else seen.set(key, row.id);
  }
  return { selected, complete, incomplete: selected.length - complete.length, duplicateIds };
}
export function restoreBatch(draft) {
  if (draft.versionsByMode) return draft.versionsByMode;
  const result = initialVersions();
  if (draft.forms) {
    for (const mode of Object.keys(MODE_LABELS)) result[mode] = [createVersion(draft.forms[mode] || {})];
  }
  return result;
}

export function mergeImportedVersions(rows, imported) {
  const empty = (row) => !row.target && !row.text.trim() && !row.segments?.length && !row.title.trim() && !row.notes.trim() && !row.name.trim() && !row.claims.trim();
  if (rows.filter((row) => !empty(row)).length + imported.length > MAX_VERSIONS) throw new Error(`每批最多 ${MAX_VERSIONS} 个版本，请减少导入数量。`);
  let cursor = 0;
  const next = rows.flatMap((row) => {
    if (!empty(row)) return [row];
    return cursor < imported.length ? [{ ...row, ...imported[cursor++], selected: true }] : [];
  });
  return [...next, ...imported.slice(cursor).map((patch) => createVersion(patch))];
}

export function parseBulkCopies(value) {
  const texts = value.split(/^\s*---\s*$/m).map((text) => text.trim()).filter(Boolean);
  if (!texts.length || texts.some((text) => Array.from(text).length > 60)) throw new Error("每版字幕需要 1–60 个字符。");
  return texts.map((text) => ({ text }));
}

export function splitRecognizedSegments(segments) {
  return segments.flatMap((segment) => {
    const text = (segment.text || "").trim();
    if (!text) return [];
    const sentences = text.match(/[^。！？!?；;]+[。！？!?；;]?/g)?.map((part) => part.trim()).filter(Boolean) || [text];
    const chunks = sentences.flatMap((sentence) => {
      const chars = Array.from(sentence);
      if (chars.length <= 60) return [sentence];
      const parts = [];
      for (let i = 0; i < chars.length; i += 50) parts.push(chars.slice(i, i + 50).join(""));
      return parts;
    });
    const start = Number(segment.start), end = Number(segment.end);
    if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) return [];
    const total = chunks.reduce((n, part) => n + Array.from(part).length, 0);
    let offset = 0;
    return chunks.map((part, index) => {
      const from = start + (end - start) * offset / total;
      offset += Array.from(part).length;
      const to = index === chunks.length - 1 ? end : start + (end - start) * offset / total;
      return { start: Math.round(from * 100) / 100, end: Math.round(to * 100) / 100, text: part, source: segment.source || "speech", needs_review: true, review_reason: segment.source === "visual" ? "画面识别待核对" : segment.needs_review ? "语音识别不确定" : "请核对语音识别" };
    });
  });
}

export function compareVisualCaptions(speechSegments, visualSegments) {
  if (!speechSegments?.length || speechSegments.every((segment) => segment.source === "visual")) return splitRecognizedSegments(visualSegments);
  const normalized = (value) => Array.from((value || "").toLowerCase()).filter((char) => /[\p{L}\p{N}]/u.test(char)).join("");
  return speechSegments.map((segment) => {
    const match = visualSegments.map((visual) => ({ visual, overlap: Math.max(0, Math.min(Number(segment.end), Number(visual.end)) - Math.max(Number(segment.start), Number(visual.start))) })).sort((a, b) => b.overlap - a.overlap)[0];
    if (!match || match.overlap < 0.25) return segment;
    const heard = normalized(segment.text), seen = normalized(match.visual.text);
    const differs = !!heard && !!seen && !heard.includes(seen) && !seen.includes(heard);
    return { ...segment, visual_text: match.visual.text, needs_review: !!segment.needs_review || differs, review_reason: differs ? "画面文字与语音疑似不一致" : segment.review_reason };
  });
}
