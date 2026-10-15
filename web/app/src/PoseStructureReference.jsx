import CanvasSelect from "./CanvasSelect.jsx";
export default function PoseStructureReference({ assets, primaryIds, value, onChange, category }) {
  return <div className="nodrag nopan nowheel">
    <label className="workflow-field">拍摄场景<CanvasSelect value={value.scene} onChange={e => onChange({ ...value, scene: e.target.value })}><option value="studio">浅色影棚</option><option value="courtyard">石材庭院</option><option value="street">城市街景</option><option value="indoor">现代室内</option></CanvasSelect></label>
    {category === "apparel" && <><label className="workflow-field">同款结构参考图（可选）<CanvasSelect value={value.supporting_asset_id} getOptionImage={id => id ? `/api/assets/${id}/file` : ""} onChange={e => onChange({ ...value, supporting_asset_id: e.target.value, same_style_confirmed: false })}><option value="">不使用</option>{assets.filter(a => !primaryIds.includes(a.id)).map(a => <option key={a.id} value={a.id}>{a.original_name}</option>)}</CanvasSelect></label>
      {value.supporting_asset_id && <><label className="workflow-field">目标颜色（建议明确填写）<input value={value.target_color || ""} maxLength={40} onChange={e => onChange({ ...value, target_color: e.target.value })} placeholder="例如：浅米白色，不使用黑色" /></label><img className="workflow-preview" src={`/api/assets/${value.supporting_asset_id}/file`} alt="同款结构参考图" /><label><input type="checkbox" checked={value.same_style_confirmed} onChange={e => onChange({ ...value, same_style_confirmed: e.target.checked })} />确认是同款，仅颜色或视角不同</label><p>主商品图决定颜色，参考图只辅助结构。缺失视角属于 AI 补全，生成后需核对。</p></>}
    </>}
  </div>;
}
export function poseStructurePayload(value, category) {
  return { target_color: value.target_color || "", scene: value.scene, supporting_asset_id: category === "apparel" ? value.supporting_asset_id : "", same_style_confirmed: category === "apparel" && value.same_style_confirmed };
}
export function poseStructureRetry(controls = {}) {
  return { target_color: controls.target_color || "", scene: controls.scene || "studio", supporting_asset_id: controls.supporting_asset_id || "", same_style_confirmed: controls.same_style_confirmed === "true" };
}
