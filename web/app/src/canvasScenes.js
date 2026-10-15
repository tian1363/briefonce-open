export const CANVAS_SCENES = [
  { id: "pose", title: "模特姿势", description: "商品图 × 模特 × 姿势，批量生成展示图。" },
  { id: "product-photo", title: "挂拍图精修", description: "从真实挂拍图制作白底图与可见细节特写。" },
  { id: "detail-page", title: "商品详情长图", description: "选择商品素材与平台，由 AI 策划并整页出图。" },
];

export function canvasScene(id) { return CANVAS_SCENES.find((scene) => scene.id === id) || CANVAS_SCENES[0]; }
