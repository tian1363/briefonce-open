// Platform-specific composition presets. A preset is an upload guarantee only where the
// linked platform explicitly defines that module size; category rules still need review.
export const DETAIL_PLATFORMS = [
  { id: "taobao", name: "淘宝 / 天猫", width: 750, height: 1000, layout: "editorial", sections: ["hero", "benefits", "details", "specs", "usage"], source: "https://developer.alibaba.com/support/announcementDetail.htm?id=25721&source=search", rule: "移动商详五段叙事；750 × 1000 为本工具预设。官方主图比例规则不等于商详尺寸。" },
  { id: "jd", name: "京东", width: 790, height: 1053, layout: "jd", sections: ["hero", "details", "specs", "usage"], source: "https://media.shop.jd.com/", rule: "商品实拍 → 细节 → 参数 → 服务，突出可核对信息；790 × 1053 为本工具预设。" },
  { id: "douyin", name: "抖店", width: 750, height: 1000, layout: "douyin", sections: ["hero", "benefits", "details", "specs"], source: "https://school.jinritemai.com/doudian/web/articlev0/aHb9JViisd3T", rule: "移动端大图与短句；商品详情须展示细节、卖点和必要信息，750 × 1000 为本工具预设。" },
  { id: "miniprogram", name: "微信小程序", width: 750, height: 1200, layout: "mini", sections: ["hero", "benefits", "specs"], source: "https://developers.weixin.qq.com/miniprogram/dev/component/image.html", rule: "小程序图片随容器自适应；750 × 1200 为本工具移动布局预设，并非微信统一上传尺寸。" },
  { id: "tiktok", name: "TikTok Shop", width: 1000, height: 1000, layout: "tiktok", sections: ["hero", "benefits", "details", "specs", "usage"], source: "https://seller-us.tiktok.com/university/essay?knowledge_id=481891871868714", rule: "方形商品图卡；官方美区商品图建议高于 600 × 600，详情描述图仍按当地站点规则核对。" },
  { id: "ozon", name: "Ozon", width: 640, height: 640, layout: "ozon-mobile", sections: ["hero", "benefits", "specs", "usage"], variants: [{ id: "mobile", name: "Ozon 移动", width: 640, height: 640, layout: "ozon-mobile" }, { id: "desktop", name: "Ozon 桌面", width: 1416, height: 708, layout: "ozon-desktop" }], source: "https://rich-content.ozon.ru/docs", rule: "富内容按模块分别导出移动端方图与桌面横幅；以 Ozon 编辑器当前模块限制为准。" },
  { id: "amazon", name: "Amazon 基础 A+", width: 970, height: 300, layout: "amazon", sections: ["hero", "benefits", "details", "specs", "usage"], source: "https://sellercentral.amazon.com/help/hub/reference/external/GLG4RQK2Y2RJADU4", rule: "按基础 A+ 横向模块导出，970 × 300、每图小于 2 MB；需有 A+ 资格并按所选模块审核。" },
  { id: "pdd", name: "拼多多", width: 750, height: 1000, layout: "pdd", sections: ["hero", "benefits", "specs", "usage"], source: "https://mms.pinduoduo.com/", rule: "紧凑的商品、卖点、规格与服务分图；750 × 1000 为本工具预设，以上传页当前校验为准。" },
];

export function detailExportVariants(platform) {
  if (!platform.variants) return [{ ...platform, platformId: platform.id, exportId: platform.id }];
  return platform.variants.map((variant) => ({ ...platform, ...variant, platformId: platform.id, exportId: `${platform.id}-${variant.id}`, sections: platform.sections }));
}

export const DETAIL_TEMPLATES = [
  { id: "fashion", name: "服饰穿搭", description: "上身展示 → 穿搭卖点 → 面料细节 → 尺码信息 → 穿着养护", accent: "#a24f3b", sampleImage: "/detail-template-gallery/fashion.jpg", sampleTitle: "亚麻西装外套" },
  { id: "beauty", name: "美妆护肤", description: "产品陈列 → 核心特点 → 质地信息 → 使用步骤 → 规格说明", accent: "#9e5d68", sampleImage: "/detail-template-gallery/beauty.jpg", sampleTitle: "精华液" },
  { id: "electronics", name: "数码家电", description: "产品与场景 → 功能解释 → 结构展示 → 技术参数 → 使用说明", accent: "#547a85", sampleImage: "/detail-template-gallery/electronics.jpg", sampleTitle: "头戴式耳机" },
  { id: "home", name: "家居生活", description: "空间展示 → 使用价值 → 材质工艺 → 尺寸信息 → 养护说明", accent: "#806849", sampleImage: "/detail-template-gallery/home.jpg", sampleTitle: "陶瓷台灯" },
];

export function detailSections(templateId) {
  const labels = {
    fashion: ["上身与整体造型", "穿搭亮点", "面料与做工", "尺码与参数", "穿着与养护"],
    beauty: ["产品陈列", "核心特点", "质地与细节", "使用步骤", "规格与说明"],
    electronics: ["产品与使用场景", "功能解释", "结构与细节", "技术参数", "使用与售后"],
    home: ["空间中的商品", "空间与使用价值", "材质与工艺", "尺寸与参数", "养护与售后"],
  };
  return labels[templateId] || labels.fashion;
}

export const DETAIL_TEXT_LIMIT = 120;
export const DETAIL_TEXT_FIELDS = { detailText: "材质与细节", specs: "规格与参数", usage: "使用、保养或售后" };
// Count Unicode characters consistently for imported and typed text, excluding outer whitespace.
export const detailTextLength = (text) => Array.from(String(text || "").trim()).length;
export function detailTextLimitHint(text) {
  const count = detailTextLength(text);
  return `${count} / ${DETAIL_TEXT_LIMIT} 字${count > DETAIL_TEXT_LIMIT ? `，超出 ${count - DETAIL_TEXT_LIMIT} 字` : ""}`;
}

export function validateDetailInput(input) {
  if (!input.title?.trim()) return "请填写商品名称";
  if (Array.from(input.title.trim()).length > 22) return "商品名称请控制在 22 字以内，避免图片截断";
  if (!input.assetIds?.length) return "请至少选择一张商品图片";
  if (!input.platformIds?.length) return "请至少选择一个平台";
  if (!input.claims?.some((line) => line.trim())) return "请填写至少一条已确认的商品卖点";
  if (input.claims.length > 4) return "最多填写 4 条商品卖点";
  if (input.claims.some((line) => Array.from(line.trim()).length > 24)) return "每条卖点请控制在 24 字以内";
  if (!input.specs?.trim()) return "请填写已核实的规格或参数";
  if (!input.usage?.trim()) return "请填写已核实的使用或售后说明";
  const overflow = Object.entries(DETAIL_TEXT_FIELDS).filter(([field]) => detailTextLength(input[field]) > DETAIL_TEXT_LIMIT);
  if (overflow.length) return `${overflow.map(([field, label]) => `「${label}」${detailTextLength(input[field])} 字，超出 ${detailTextLength(input[field]) - DETAIL_TEXT_LIMIT} 字`).join("；")}。当前排版模板每个字段最多 ${DETAIL_TEXT_LIMIT} 字，请精简后生成（非平台字数限制）。`;
  return "";
}

function loadImage(url) {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error("商品图片加载失败，请重新选择素材"));
    image.src = url;
  });
}

const DETAIL_PALETTES = {
  fashion: { paper: "#f5f0e8", ink: "#342c29", soft: "#e8ddd1" },
  beauty: { paper: "#f8eeef", ink: "#4b303a", soft: "#f0dfe4" },
  electronics: { paper: "#eaf0f1", ink: "#20333b", soft: "#d5e0e3" },
  home: { paper: "#eee8dc", ink: "#3e352b", soft: "#ded2be" },
};

function writeLines(ctx, text, x, y, width, lineHeight, maxLines = 8) {
  let line = "", count = 0;
  for (const char of String(text || "")) {
    if (char === "\n" || (line && ctx.measureText(line + char).width > width)) {
      if (line) { ctx.fillText(line, x, y + count * lineHeight); count++; }
      if (count >= maxLines) return y + count * lineHeight;
      line = char === "\n" ? "" : char;
    } else line += char;
  }
  if (line && count < maxLines) { ctx.fillText(line, x, y + count * lineHeight); count++; }
  return y + count * lineHeight;
}

function imageIn(ctx, image, x, y, width, height, fit = "contain") {
  ctx.save();
  ctx.beginPath(); ctx.rect(x, y, width, height); ctx.clip();
  const scale = fit === "cover" ? Math.max(width / image.width, height / image.height) : Math.min(width / image.width, height / image.height);
  const iw = image.width * scale, ih = image.height * scale;
  ctx.drawImage(image, x + (width - iw) / 2, y + (height - ih) / 2, iw, ih);
  ctx.restore();
}

function fill(ctx, color, x, y, width, height) { ctx.fillStyle = color; ctx.fillRect(x, y, width, height); }
function line(ctx, color, x1, y, x2) { fill(ctx, color, x1, y, x2 - x1, 2); }
function copy(ctx, value, x, y, width, size = 27, color = "#222", lines = 5, weight = 500) {
  ctx.fillStyle = color; ctx.font = `${weight} ${size}px Inter, Noto Sans SC, sans-serif`;
  return writeLines(ctx, value, x, y, width, size * 1.5, lines);
}
function sectionHead(ctx, eyebrow, title, x, y, width, palette, accent, inverted = false) {
  copy(ctx, eyebrow, x, y, width, 19, inverted ? "#bacbd0" : accent, 1, 700);
  return copy(ctx, title, x, y + 55, width, 45, inverted ? "#ffffff" : palette.ink, 2, 700);
}
function specsRows(input) {
  return input.specs.split(/\n|；|;/).map((row) => row.trim()).filter(Boolean);
}
function drawSpecs(ctx, input, x, y, width, palette, accent, dark = false) {
  const rows = specsRows(input);
  const ink = dark ? "#f3f6f6" : palette.ink;
  rows.forEach((row, index) => {
    line(ctx, dark ? "#52666d" : palette.soft, x, y - 33, x + width);
    copy(ctx, String(index + 1).padStart(2, "0"), x, y, 52, 21, accent, 1, 700);
    y = copy(ctx, row, x + 70, y, width - 70, 24, ink, 3) + 42;
  });
  return y;
}
function claimList(ctx, input, x, y, width, palette, accent, dark = false) {
  const ink = dark ? "#ffffff" : palette.ink;
  input.claims.forEach((claim, index) => {
    copy(ctx, `0${index + 1}`, x, y, 44, 21, accent, 1, 700);
    y = copy(ctx, claim, x + 57, y, width - 57, 27, ink, 3, 600) + 34;
    line(ctx, dark ? "#52666d" : palette.soft, x, y - 22, x + width);
  });
  return y;
}
function drawPanel(canvas, image, input, platform, template, index, referenceStyle) {
  const { width: w, height: h } = platform;
  const palette = referenceStyle ? { ...DETAIL_PALETTES[template.id], paper: referenceStyle.paper, soft: referenceStyle.soft } : DETAIL_PALETTES[template.id];
  const accent = referenceStyle?.accent || template.accent;
  const section = detailSections(template.id)[index];
  canvas.width = w; canvas.height = h;
  const ctx = canvas.getContext("2d");
  fill(ctx, palette.paper, 0, 0, w, h);
  const title = input.title.trim();
  const summary = input.claims.slice(0, 2).join("  ·  ");
  const details = input.detailText || "请结合实物图片核对细节";

  // Fashion follows an editorial sequence: worn silhouette, styling, fabric, fit, care.
  if (template.id === "fashion") {
    if (index === 0) {
      imageIn(ctx, image, 0, 0, w, 735, "cover");
      fill(ctx, palette.paper, 0, 735, w, 265);
      copy(ctx, "01  /  LOOKBOOK", 55, 783, w - 110, 19, accent, 1, 700);
      copy(ctx, title, 55, 858, w - 110, 53, palette.ink, 2, 700);
      copy(ctx, summary, 55, 949, w - 110, 23, palette.ink, 1);
    } else if (index === 1) {
      sectionHead(ctx, "02  /  STYLE", section, 55, 100, w - 110, palette, accent);
      fill(ctx, palette.soft, 55, 255, 365, 605); imageIn(ctx, image, 55, 255, 365, 605, "cover");
      claimList(ctx, input, 455, 310, w - 505, palette, accent);
    } else if (index === 2) {
      sectionHead(ctx, "03  /  FABRIC & FINISH", section, 55, 100, w - 110, palette, accent);
      imageIn(ctx, image, 55, 260, w - 110, 420, "cover");
      line(ctx, accent, 55, 730, 125); copy(ctx, details, 55, 785, w - 110, 28, palette.ink, 5);
    } else if (index === 3) {
      sectionHead(ctx, "04  /  FIT GUIDE", section, 55, 100, w - 110, palette, accent);
      copy(ctx, "尺码以商品实测资料为准", 55, 250, w - 110, 23, accent, 2);
      const end = drawSpecs(ctx, input, 55, 340, w - 110, palette, accent);
      if (end < 750) imageIn(ctx, image, 55, Math.max(650, end + 20), w - 110, 800 - Math.max(650, end + 20), "cover");
      if (end < 810) { fill(ctx, palette.soft, 55, 820, w - 110, 120); copy(ctx, "选购前核对肩宽、胸围、衣长及版型说明", 75, 866, w - 150, 23, palette.ink, 2); }
    } else {
      imageIn(ctx, image, 0, 0, w, 510, "cover");
      sectionHead(ctx, "05  /  CARE", section, 55, 570, w - 110, palette, accent);
      copy(ctx, input.usage, 55, 755, w - 110, 28, palette.ink, 5);
    }
    return canvas;
  }

  // Beauty prioritizes product texture, application and factual pack information.
  if (template.id === "beauty") {
    if (index === 0) {
      fill(ctx, palette.soft, 45, 70, w - 90, 710);
      imageIn(ctx, image, 65, 90, w - 130, 660, "contain");
      copy(ctx, "PRODUCT / 01", 55, 824, w - 110, 19, accent, 1, 700);
      copy(ctx, title, 55, 888, w - 110, 52, palette.ink, 2, 700);
    } else if (index === 1) {
      sectionHead(ctx, "BENEFITS / 02", section, 55, 105, w - 110, palette, accent);
      const end = claimList(ctx, input, 55, 315, w - 110, palette, accent);
      if (end < 785) imageIn(ctx, image, 55, Math.max(690, end + 18), w - 110, 925 - Math.max(690, end + 18), "cover");
    } else if (index === 2) {
      sectionHead(ctx, "TEXTURE / 03", section, 55, 105, w - 110, palette, accent);
      imageIn(ctx, image, 55, 260, w - 110, 465, "cover");
      copy(ctx, details, 55, 795, w - 110, 28, palette.ink, 4);
    } else if (index === 3) {
      sectionHead(ctx, "HOW TO USE / 04", section, 55, 105, w - 110, palette, accent);
      fill(ctx, palette.soft, 55, 265, w - 110, 460);
      imageIn(ctx, image, 145, 280, w - 290, 420, "contain");
      copy(ctx, input.usage, 55, 795, w - 110, 28, palette.ink, 4);
    } else {
      sectionHead(ctx, "PRODUCT INFO / 05", section, 55, 105, w - 110, palette, accent);
      drawSpecs(ctx, input, 55, 300, w - 110, palette, accent);
    }
    return canvas;
  }

  // Electronics resembles a technical A+ page: dark product stage, features, build, data, support.
  if (template.id === "electronics") {
    fill(ctx, palette.ink, 0, 0, w, h);
    if (index === 0) {
      copy(ctx, "01 / PRODUCT", 55, 92, w - 110, 20, accent, 1, 700);
      copy(ctx, title, 55, 165, w - 110, 52, "#ffffff", 2, 700);
      imageIn(ctx, image, 50, 250, w - 100, 570, "contain");
      copy(ctx, summary, 55, 905, w - 110, 23, "#ffffff", 2);
    } else if (index === 1) {
      sectionHead(ctx, "02 / FEATURES", section, 55, 100, w - 110, palette, accent, true);
      const end = claimList(ctx, input, 55, 295, w - 110, palette, accent, true);
      if (end < 790) imageIn(ctx, image, 55, Math.max(685, end + 18), w - 110, 935 - Math.max(685, end + 18), "cover");
    } else if (index === 2) {
      sectionHead(ctx, "03 / BUILD", section, 55, 100, w - 110, palette, accent, true);
      fill(ctx, "#30464e", 55, 245, w - 110, 480);
      imageIn(ctx, image, 65, 255, w - 130, 460, "contain");
      copy(ctx, details, 55, 795, w - 110, 27, "#ffffff", 4);
    } else if (index === 3) {
      sectionHead(ctx, "04 / SPECIFICATIONS", section, 55, 100, w - 110, palette, accent, true);
      drawSpecs(ctx, input, 55, 300, w - 110, palette, accent, true);
    } else {
      sectionHead(ctx, "05 / SETUP & CARE", section, 55, 100, w - 110, palette, accent, true);
      copy(ctx, input.usage, 55, 320, w - 110, 31, "#ffffff", 7);
      line(ctx, accent, 55, 780, 130);
      copy(ctx, "使用前请核对兼容条件与产品说明", 55, 850, w - 110, 22, "#bacbd0", 3);
    }
    return canvas;
  }

  // Home uses spatial photography, then material and measured dimensions.
  if (index === 0) {
    imageIn(ctx, image, 0, 0, w, 780, "cover");
    fill(ctx, palette.paper, 0, 780, w, 220);
    copy(ctx, "01 / IN THE HOME", 55, 825, w - 110, 20, accent, 1, 700);
    copy(ctx, title, 55, 905, w - 110, 49, palette.ink, 2, 700);
  } else if (index === 1) {
    sectionHead(ctx, "02 / LIVING", section, 55, 95, w - 110, palette, accent);
    imageIn(ctx, image, 55, 245, w - 110, 445, "cover");
    claimList(ctx, input, 55, 755, w - 110, palette, accent);
  } else if (index === 2) {
    sectionHead(ctx, "03 / MATERIAL", section, 55, 95, w - 110, palette, accent);
    imageIn(ctx, image, 55, 245, 395, 580, "cover");
    fill(ctx, palette.soft, 465, 245, w - 520, 580);
    copy(ctx, details, 490, 335, w - 570, 26, palette.ink, 12);
  } else if (index === 3) {
    sectionHead(ctx, "04 / DIMENSIONS", section, 55, 95, w - 110, palette, accent);
    const end = drawSpecs(ctx, input, 55, 300, w - 110, palette, accent);
    if (end < 820) imageIn(ctx, image, 55, Math.max(675, end + 18), w - 110, 945 - Math.max(675, end + 18), "cover");
  } else {
    sectionHead(ctx, "05 / CARE", section, 55, 95, w - 110, palette, accent);
    copy(ctx, input.usage, 55, 290, w - 110, 30, palette.ink, 8);
    imageIn(ctx, image, 55, 610, w - 110, 330, "cover");
  }
  return canvas;
}

const SECTION_TITLES = {
  hero: ["商品总览", "PRODUCT"], benefits: ["核心卖点", "FEATURES"], details: ["真实细节", "DETAILS"], specs: ["规格参数", "SPECIFICATIONS"], usage: ["使用与服务", "CARE & SUPPORT"],
};

function sectionCopy(input, section) {
  if (section === "hero") return [input.title, input.claims.slice(0, 2).join("  ·  ")];
  if (section === "benefits") return [input.claims[0] || "核心特点", input.claims.join("  /  ")];
  if (section === "details") return ["可见的商品细节", input.detailText || input.claims.slice(0, 2).join("  /  ")];
  if (section === "specs") return ["已核实的规格", input.specs];
  return ["使用、保养与服务", input.usage];
}

function sampleReferenceStyle(image, aspects, note, template) {
  const style = { accent: template.accent, paper: DETAIL_PALETTES[template.id].paper, soft: DETAIL_PALETTES[template.id].soft, imageRight: false, airy: false };
  if (!image) return style;
  const canvas = document.createElement("canvas"); canvas.width = 24; canvas.height = 24;
  const ctx = canvas.getContext("2d", { willReadFrequently: true });
  ctx.drawImage(image, 0, 0, 24, 24);
  const pixels = ctx.getImageData(0, 0, 24, 24).data;
  let r = 0, g = 0, b = 0, count = 0, left = 0, right = 0, light = 0;
  for (let y = 0; y < 24; y++) for (let x = 0; x < 24; x++) {
    const p = (y * 24 + x) * 4; const red = pixels[p], green = pixels[p + 1], blue = pixels[p + 2];
    const contrast = 255 - (red + green + blue) / 3;
    if (x < 12) left += contrast; else right += contrast;
    if (contrast < 28) light++;
    if (Math.max(red, green, blue) - Math.min(red, green, blue) > 18 && contrast > 35 && contrast < 200) { r += red; g += green; b += blue; count++; }
  }
  if (aspects.includes("配色") && count) {
    const c = [r, g, b].map((value) => Math.round(value / count));
    const hex = (values) => `#${values.map((value) => Math.max(0, Math.min(255, value)).toString(16).padStart(2, "0")).join("")}`;
    style.accent = hex(c.map((value) => Math.round(value * .72)));
    style.paper = hex(c.map((value) => Math.round(value * .12 + 255 * .88)));
    style.soft = hex(c.map((value) => Math.round(value * .25 + 255 * .75)));
  }
  if (aspects.includes("构图")) style.imageRight = right > left;
  if (aspects.includes("留白")) style.airy = light / 576 > .2;
  if (/右侧|靠右|右边/.test(note)) style.imageRight = true;
  if (/左侧|靠左|左边/.test(note)) style.imageRight = false;
  if (/留白|宽松|疏朗/.test(note)) style.airy = true;
  if (/紧凑|密集/.test(note)) style.airy = false;
  return style;
}

function drawPlatformPanel(canvas, image, input, platform, template, section, index, referenceStyle) {
  const { width: w, height: h, layout } = platform;
  canvas.width = w; canvas.height = h;
  const ctx = canvas.getContext("2d");
  const accent = referenceStyle.accent;
  const paper = referenceStyle.paper;
  const soft = referenceStyle.soft;
  const ink = "#292624";
  const margin = Math.round(w * (referenceStyle.airy ? .09 : .06));
  const [headline, body] = sectionCopy(input, section);
  const international = ["amazon", "tiktok", "ozon-mobile", "ozon-desktop"].includes(layout);
  const sectionTitle = SECTION_TITLES[section][international ? 1 : 0];
  const number = String(index + 1).padStart(2, "0");
  fill(ctx, paper, 0, 0, w, h);

  if (layout === "amazon" || layout === "ozon-desktop") {
    const imageWidth = layout === "amazon" ? Math.round(w * .48) : Math.round(w * .52);
    const imageX = referenceStyle.imageRight !== (index % 2 === 1) ? w - imageWidth : 0;
    const textX = imageX === 0 ? imageWidth + margin : margin;
    const textWidth = w - imageWidth - 2 * margin;
    fill(ctx, soft, imageX, 0, imageWidth, h);
    imageIn(ctx, image, imageX, 0, imageWidth, h, section === "hero" ? "contain" : "cover");
    fill(ctx, accent, textX, 45, Math.min(75, textWidth), 5);
    copy(ctx, `${number} / ${sectionTitle}`, textX, 80, textWidth, layout === "amazon" ? 15 : 20, accent, 1, 700);
    copy(ctx, headline, textX, layout === "amazon" ? 132 : 173, textWidth, layout === "amazon" ? 28 : 42, ink, 2, 700);
    copy(ctx, body, textX, layout === "amazon" ? 196 : 292, textWidth, layout === "amazon" ? 17 : 27, ink, layout === "amazon" ? 3 : 6);
    return canvas;
  }

  if (layout === "tiktok" || layout === "ozon-mobile") {
    const imageHeight = layout === "tiktok" ? Math.round(h * .67) : Math.round(h * .56);
    fill(ctx, soft, 0, 0, w, imageHeight);
    imageIn(ctx, image, 0, 0, w, imageHeight, section === "hero" ? "contain" : "cover");
    fill(ctx, accent, 0, imageHeight, w, layout === "tiktok" ? 9 : 5);
    copy(ctx, `${number}  ${sectionTitle}`, margin, imageHeight + (layout === "tiktok" ? 55 : 37), w - 2 * margin, layout === "tiktok" ? 24 : 17, accent, 1, 700);
    copy(ctx, headline, margin, imageHeight + (layout === "tiktok" ? 116 : 77), w - 2 * margin, layout === "tiktok" ? 43 : 29, ink, 2, 700);
    copy(ctx, body, margin, imageHeight + (layout === "tiktok" ? 190 : 126), w - 2 * margin, layout === "tiktok" ? 24 : 17, ink, 3);
    return canvas;
  }

  if (layout === "mini") {
    fill(ctx, soft, margin, 65, w - 2 * margin, 650);
    imageIn(ctx, image, margin + 15, 80, w - 2 * margin - 30, 620, section === "hero" ? "contain" : "cover");
    fill(ctx, "#ffffff", margin, 730, w - 2 * margin, 400);
    copy(ctx, `${number} / ${sectionTitle}`, margin + 34, 795, w - 2 * margin - 68, 20, accent, 1, 700);
    copy(ctx, headline, margin + 34, 875, w - 2 * margin - 68, 47, ink, 2, 700);
    copy(ctx, body, margin + 34, 1010, w - 2 * margin - 68, 24, ink, 3);
    return canvas;
  }

  if (layout === "douyin") {
    fill(ctx, accent, 0, 0, 12, h);
    copy(ctx, number, margin, 105, w - 2 * margin, 70, accent, 1, 700);
    copy(ctx, sectionTitle, margin + 120, 94, w - 2 * margin - 120, 31, ink, 1, 700);
    fill(ctx, soft, margin, 140, w - 2 * margin, 590);
    imageIn(ctx, image, margin, 140, w - 2 * margin, 590, section === "hero" ? "contain" : "cover");
    copy(ctx, headline, margin, 805, w - 2 * margin, 43, ink, 2, 700);
    copy(ctx, body, margin, 918, w - 2 * margin, 22, ink, 2);
    return canvas;
  }

  if (layout === "jd") {
    fill(ctx, accent, 0, 0, w, 12);
    copy(ctx, `${number} / ${sectionTitle}`, margin, 95, w - 2 * margin, 22, accent, 1, 700);
    copy(ctx, headline, margin, 175, w - 2 * margin, 45, ink, 2, 700);
    const imageX = referenceStyle.imageRight ? w - margin - 335 : margin;
    const textX = referenceStyle.imageRight ? margin : margin + 370;
    fill(ctx, soft, imageX, 250, 335, 660);
    imageIn(ctx, image, imageX, 250, 335, 660, section === "hero" ? "contain" : "cover");
    copy(ctx, body, textX, 330, 320, 28, ink, 11);
    line(ctx, accent, textX, 880, textX + 100);
    return canvas;
  }

  // Pinduoduo: compact proof points and measured facts instead of a full-bleed hero.
  fill(ctx, soft, margin, 55, w - 2 * margin, 520);
  imageIn(ctx, image, margin, 55, w - 2 * margin, 520, section === "hero" ? "contain" : "cover");
  fill(ctx, accent, margin, 605, 75, 6);
  copy(ctx, `${number}  ${sectionTitle}`, margin, 665, w - 2 * margin, 21, accent, 1, 700);
  copy(ctx, headline, margin, 730, w - 2 * margin, 42, ink, 2, 700);
  copy(ctx, body, margin, 845, w - 2 * margin, 25, ink, 4);
  return canvas;
}

function stitchPanels(panels, platform) {
  const canvas = document.createElement("canvas");
  canvas.width = platform.width; canvas.height = platform.height * panels.length;
  const ctx = canvas.getContext("2d");
  panels.forEach((panel, index) => ctx.drawImage(panel, 0, index * platform.height));
  return canvas;
}

function toJPEG(canvas, quality = 0.88) {
  return new Promise((resolve, reject) => canvas.toBlob((blob) => blob ? resolve(blob) : reject(new Error("图片导出失败，请重试")), "image/jpeg", quality));
}

async function toJPEGWithin(canvas, maxBytes = 0) {
  for (const quality of [.88, .78, .68]) {
    const blob = await toJPEG(canvas, quality);
    if (!maxBytes || blob.size <= maxBytes) return blob;
  }
  throw new Error("导出图片超过当前平台单图预设体积，请减少图片复杂度");
}

export async function buildDetailPackage(input, onProgress = () => {}) {
  const problem = validateDetailInput(input);
  if (problem) throw new Error(problem);
  const template = DETAIL_TEMPLATES.find((item) => item.id === input.templateId) || DETAIL_TEMPLATES[0];
  const platforms = DETAIL_PLATFORMS.filter((item) => input.platformIds.includes(item.id));
  const images = await Promise.all(input.assetIds.map((id) => loadImage(`/api/assets/${encodeURIComponent(id)}/file`)));
  const referenceImage = input.referenceId ? await loadImage(`/api/assets/${encodeURIComponent(input.referenceId)}/file`) : null;
  const referenceStyle = sampleReferenceStyle(referenceImage, input.referenceAspects || [], input.referenceNote || "", template);
  const { default: JSZip } = await import("jszip");
  const zip = new JSZip();
  const previews = [];
  const longPages = [];
  const total = platforms.reduce((count, item) => count + item.sections.length * detailExportVariants(item).length, 0);
  for (const platform of platforms) {
    for (const variant of detailExportVariants(platform)) {
      const panels = [];
      for (const [index, section] of variant.sections.entries()) {
        const canvas = variant.layout === "editorial"
          ? drawPanel(document.createElement("canvas"), images[index % images.length], input, variant, template, index, referenceStyle)
          : drawPlatformPanel(document.createElement("canvas"), images[index % images.length], input, variant, template, section, index, referenceStyle);
        panels.push(canvas);
        const blob = await toJPEGWithin(canvas, variant.layout === "amazon" ? 2 * 1024 * 1024 : variant.layout === "tiktok" ? 5 * 1024 * 1024 : 0);
        zip.file(`${variant.exportId}/${String(index + 1).padStart(2, "0")}-${section}.jpg`, blob);
        const sectionName = variant.layout === "editorial" ? detailSections(template.id)[index] : SECTION_TITLES[section][["amazon", "tiktok", "ozon-mobile", "ozon-desktop"].includes(variant.layout) ? 1 : 0];
        previews.push({ platform: variant.name, section: sectionName, width: variant.width, height: variant.height, url: URL.createObjectURL(blob) });
        onProgress(previews.length, total);
      }
      const longBlob = await toJPEG(stitchPanels(panels, variant), 0.86);
      zip.file(`${variant.exportId}/overview-long.jpg`, longBlob);
      longPages.push({ platform: variant.name, width: variant.width, height: variant.height * variant.sections.length, url: URL.createObjectURL(longBlob) });
    }
  }
  zip.file("README.txt", "由 BriefOnce 导出。每个平台目录包含独立版式的分段图；overview-long.jpg 仅供整体预览，Amazon A+ 与 Ozon 请逐模块上传。尺寸预设不保证审核通过；请在对应商家后台核对当前类目、站点、语言、体积和模块要求，并逐图核对商品事实、文字、图片授权。\n" + platforms.map((item) => `${item.name}: ${item.rule} ${item.source}`).join("\n") + (input.referenceId ? `\n视觉参考：${(input.referenceAspects || []).join("、")}；${input.referenceNote || ""}。仅用于配色、构图和留白排版，不作为商品事实。` : ""));
  return { blob: await zip.generateAsync({ type: "blob" }), previews, longPages };
}

export async function buildDetailTemplateSample(template) {
  const image = await loadImage(template.sampleImage);
  const platform = DETAIL_PLATFORMS[0];
  const examples = {
    fashion: { claims: ["利落直线肩型", "自然宽松廓形", "通勤与日常均可搭配"], detailText: "面料纹理、纽扣、走线与内里细节应使用商品实拍展示。", specs: "版型：宽松\n颜色：米色\n尺码：请填写实测尺码表", usage: "请按衣物洗护标签清洁。试穿时核对肩宽、胸围和衣长。" },
    beauty: { claims: ["清透质地", "便于日常护肤搭配", "滴管取用"], detailText: "展示真实质地、瓶身与包装标识；成分与功效以实际产品资料为准。", specs: "品类：精华液\n容量：请填写包装标注\n成分：请填写备案信息", usage: "洁面后按产品说明取适量使用。首次使用前请阅读包装提示。" },
    electronics: { claims: ["包耳式佩戴", "头梁可调节", "耳罩折叠收纳"], detailText: "展示耳罩、按键、接口与头梁结构；功能描述以实测资料为准。", specs: "连接方式：请填写\n续航时间：请填写实测值\n接口类型：请填写", usage: "使用前核对设备兼容性。充电及清洁方式以产品说明书为准。" },
    home: { claims: ["适合床头或边几陈列", "柔和空间氛围", "陶瓷灯体质感"], detailText: "展示灯罩、灯体表面与底座连接处的实拍细节。", specs: "整体高度：请填写实测值\n灯罩尺寸：请填写\n材质：请核对实物", usage: "摆放在平稳台面。清洁与更换光源前，请阅读产品说明。" },
  };
  const input = { title: template.sampleTitle, ...examples[template.id] };
  const panels = detailSections(template.id).map((_, index) => drawPanel(document.createElement("canvas"), image, input, platform, template, index));
  return URL.createObjectURL(await toJPEG(stitchPanels(panels, platform), 0.76));
}
