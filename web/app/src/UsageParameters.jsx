import "./CreditUsage.css";

const labels = { billing_tier: "提交时计费档位", model: "具体型号", size: "图片尺寸 / 档位", image_size: "清晰度", aspect_ratio: "画面比例", quality: "质量", resolution: "分辨率", seed: "随机种子", n: "生成数量", reference_count: "参考图片数", response_format: "返回格式", prompt: "提示词", negative_prompt: "负面提示词", negative: "负面提示词", duration: "时长（秒）", sound_enabled: "生成声音" };
export function usageResolution(item) {
  const p = item.parameters || {};
  return p.image_size || p.resolution || p.size || (p.billing_tier !== "default" ? p.billing_tier : "") || "清晰度未记录";
}
export default function UsageParameters({ item }) {
  const entries = Object.entries(item.parameters || {}).filter(([key]) => key !== "model");
  return <div className="usage-parameters"><dl><dt>具体型号</dt><dd>{item.model || item.parameters?.model || "未记录"}</dd>{item.provider && <><dt>供应商</dt><dd>{item.provider}</dd></>}{entries.map(([key, value]) => <div className="usage-parameter-row" key={key}><dt>{labels[key] || key}</dt><dd>{typeof value === "boolean" ? value ? "开启" : "关闭" : String(value)}</dd></div>)}</dl>{!entries.length && <p>该记录未保存调用参数。</p>}<small>显示提交时记录的参数；清晰度档位与最终图片像素尺寸可能不同。未传入的参数由供应商默认处理。</small></div>;
}
