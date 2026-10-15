import { test } from "node:test";
import assert from "node:assert/strict";
import { DETAIL_PLATFORMS, DETAIL_TEMPLATES, detailExportVariants, detailSections, validateDetailInput } from "./detailPageBuilder.js";

test("detail export requires verified content and a product image", () => {
  const input = { title: "test", assetIds: ["asset"], platformIds: ["taobao"], claims: ["confirmed claim"], specs: "100 ml", usage: "see label" };
  assert.equal(validateDetailInput(input), "");
  assert.match(validateDetailInput({ ...input, assetIds: [] }), /图片/);
  assert.match(validateDetailInput({ ...input, specs: "" }), /规格/);
  assert.match(validateDetailInput({ ...input, claims: [] }), /卖点/);
  assert.match(validateDetailInput({ ...input, claims: ["a", "b", "c", "d", "e"] }), /4 条/);
});

test("platforms export their own sections, aspect ratios and module variants", () => {
  assert.equal(DETAIL_PLATFORMS.length, 8);
  const counts = Object.fromEntries(DETAIL_PLATFORMS.map((item) => [item.id, item.sections.length]));
  assert.deepEqual(counts, { taobao: 5, jd: 4, douyin: 4, miniprogram: 3, tiktok: 5, ozon: 4, amazon: 5, pdd: 4 });
  for (const platform of DETAIL_PLATFORMS) {
    assert.ok(platform.width > 0 && platform.height > 0);
    assert.match(platform.source, /^https:\/\//);
    assert.ok(platform.sections.every((section) => ["hero", "benefits", "details", "specs", "usage"].includes(section)));
    for (const variant of detailExportVariants(platform)) {
      assert.equal(variant.sections.length, counts[platform.id]);
      assert.ok(variant.exportId);
      assert.ok(variant.width > 0 && variant.height > 0);
    }
  }
  const ozon = detailExportVariants(DETAIL_PLATFORMS.find((item) => item.id === "ozon"));
  assert.deepEqual(ozon.map((variant) => [variant.exportId, variant.width, variant.height]), [["ozon-mobile", 640, 640], ["ozon-desktop", 1416, 708]]);
  assert.deepEqual(detailExportVariants(DETAIL_PLATFORMS.find((item) => item.id === "amazon")).map((variant) => [variant.width, variant.height]), [[970, 300]]);
  for (const template of DETAIL_TEMPLATES) assert.equal(detailSections(template.id).length, 5);
  assert.equal(new Set(DETAIL_TEMPLATES.map((template) => detailSections(template.id).join("|"))).size, DETAIL_TEMPLATES.length);
  assert.match(detailSections("fashion")[3], /尺码/);
  assert.match(detailSections("beauty")[3], /使用/);
  assert.match(detailSections("electronics")[3], /技术参数/);
  assert.match(detailSections("home")[3], /尺寸/);
});

test("detail field limits identify all overflowing imported fields without altering their source", () => {
  const input = { title: "商品", assetIds: ["asset"], platformIds: ["taobao"], claims: ["卖点"], specs: "参数", usage: "按标签保养" };
  assert.equal(validateDetailInput({ ...input, detailText: "字".repeat(120) }), "");
  const imported = { ...input, detailText: "字".repeat(121), specs: "字".repeat(130) };
  const error = validateDetailInput(imported);
  assert.match(error, /「材质与细节」121 字，超出 1 字/);
  assert.match(error, /「规格与参数」130 字，超出 10 字/);
  assert.match(error, /非平台字数限制/);
  assert.equal(imported.detailText.length, 121);
  assert.equal(validateDetailInput({ ...input, detailText: `  ${"😀".repeat(120)}\n` }), "");
});
