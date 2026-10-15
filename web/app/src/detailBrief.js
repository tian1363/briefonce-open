export const detailBriefFields = [
  ["title", "商品名称"], ["claimsText", "已确认卖点"],
  ["detailText", "材质与细节"], ["specs", "真实规格与参数"], ["usage", "使用、保养或售后"],
];
export function detailBriefError(draft, assetCount) {
  if (!draft.title?.trim()) return "请填写商品名称";
  if (!assetCount) return "请至少选择一张商品图片";
  if (!draft.referenceId) return "请先选择或补充一张实际详情页模板参考图";
  if (!draft.platformIds?.length) return "请至少选择一个平台";
  for (const [field, label] of detailBriefFields) {
    if (Array.from(draft[field] || "").length > 3000) return `「${label}」超过 3000 字，请减少内容后策划`;
  }
  return "";
}
