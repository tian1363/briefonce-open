import CanvasSelect from "./CanvasSelect.jsx";
import { useEffect, useState } from "react";
import { getImageCatalog } from "./api.js";
import "./ImageModelPicker.css";
export default function ImageModelPicker({
  value,
  onChange,
  defaultRatio = "1:1",
  count = 1,
  disabled = false,
  capability = "image_edit",
}) {
  const [catalog, setCatalog] = useState(null),
    [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    getImageCatalog()
      .then((v) => {
        if (active) setCatalog(v);
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, []);
  const defaultProfile = catalog?.defaults?.[capability];
  const models =
    defaultProfile?.provider === "zexapi"
      ? (catalog?.models || []).filter((x) => x.quotes.length)
      : [];
  useEffect(() => {
    if (!value.model && defaultProfile?.provider === "zexapi") {
      const item = (catalog?.models || []).find(
        (x) => x.model.id === defaultProfile.model && x.quotes.length,
      );
      if (item) {
        const q = item.quotes[0];
        onChange({
          model: item.model.id,
          aspect_ratio: defaultRatio,
          price_version: q.version,
          ...(item.model.tier === "variable" ? { image_size: q.tier } : {}),
        });
      }
    }
  }, [
    catalog,
    capability,
    value.model,
    defaultRatio,
    onChange,
    defaultProfile,
  ]);
  const selected = models.find((x) => x.model.id === value.model);
  const tier =
    selected?.model.tier === "variable"
      ? value.image_size || "1K"
      : selected?.model.tier;
  const quote = selected?.quotes.find((q) => q.tier === tier);
  function choose(id) {
    const next = models.find((x) => x.model.id === id);
    const ratio = value.aspect_ratio || defaultRatio;
    onChange(
      id
        ? {
            model: id,
            aspect_ratio: ratio,
            price_version: next.quotes[0]?.version,
            ...(next?.model.tier === "variable"
              ? { image_size: next.quotes[0]?.tier || "1K" }
              : {}),
          }
        : {},
    );
  }
  return (
    <fieldset className="image-model-picker nodrag" disabled={disabled}>
      <legend>出图模型与参数</legend>
      <label>
        模型
        <CanvasSelect
          value={value.model || ""}
          onChange={(e) => choose(e.target.value)}
        >
          <option value="">使用平台默认模型</option>
          {models.map((x) => (
            <option key={x.model.id} value={x.model.id}>
              {x.model.name}
            </option>
          ))}
        </CanvasSelect>
      </label>
      {selected && (
        <>
          <div className="image-model-controls">
            <label>
              画幅
              <CanvasSelect
                value={value.aspect_ratio || defaultRatio}
                onChange={(e) =>
                  onChange({
                    ...value,
                    aspect_ratio: e.target.value,
                    size: undefined,
                  })
                }
              >
                {selected.model.ratios.map((r) => (
                  <option key={r} value={r}>
                    {r === "auto" ? "自动（按提示词）" : r}
                  </option>
                ))}
              </CanvasSelect>
            </label>
            <label>
              分辨率
              {selected.model.tier === "variable" ? (
                <CanvasSelect
                  value={value.image_size || "1K"}
                  onChange={(e) =>
                    onChange({
                      ...value,
                      image_size: e.target.value,
                      price_version: selected.quotes.find(
                        (q) => q.tier === e.target.value,
                      )?.version,
                    })
                  }
                >
                  {selected.quotes.map((q) => (
                    <option key={q.tier}>{q.tier}</option>
                  ))}
                </CanvasSelect>
              ) : (
                <output>{tier === "default" ? "供应商默认" : tier}</output>
              )}
            </label>
          </div>
          {selected.model.pixel_size && (
            <details>
              <summary>精确像素（高级设置）</summary>
              <label>
                像素尺寸
                <CanvasSelect
                  value={value.size || ""}
                  onChange={(e) =>
                    onChange(
                      e.target.value
                        ? {
                            model: value.model,
                            size: e.target.value,
                            price_version: value.price_version,
                          }
                        : {
                            model: value.model,
                            aspect_ratio: defaultRatio,
                            price_version: value.price_version,
                          },
                    )
                  }
                >
                  <option value="">按画幅生成</option>
                  {Object.entries(catalog.pixel_sizes).map(([ratio, sizes]) => (
                    <option
                      key={ratio}
                      value={sizes[{ "1K": 0, "2K": 1, "4K": 2 }[tier]]}
                    >
                      {ratio} · {sizes[{ "1K": 0, "2K": 1, "4K": 2 }[tier]]}
                    </option>
                  ))}
                </CanvasSelect>
              </label>
            </details>
          )}
          <strong aria-live="polite">
            {quote
              ? `${quote.points} 积分／张 · 本次预计 ${quote.points * count} 积分`
              : "该档位未启用，请选择其他档位"}
          </strong>
        </>
      )}
      {!selected && (
        <p>
          默认型号由管理员设置；更换平台默认模型不影响已提交任务。展开选择可查看各型号限制与报价。
        </p>
      )}
      {!models.length && catalog && (
        <p>新供应商模型尚未启用并定价，继续使用当前已配置的图片能力。</p>
      )}
      {error && <p role="alert">{error}</p>}
    </fieldset>
  );
}
