import test from "node:test";
import assert from "node:assert/strict";
import { detailBriefError } from "./detailBrief.js";
const draft = { title: "户外外套", referenceId: "template-image", platformIds: ["taobao"], claimsText: "", detailText: "材".repeat(256), specs: "", usage: "" };
test("AI planning accepts long product facts without the legacy layout constraints", () => {
  assert.equal(detailBriefError(draft, 1), "");
  assert.equal(draft.detailText.length, 256);
  assert.equal(detailBriefError({ ...draft, detailText: "😀".repeat(3000), claimsText: "卖点\n".repeat(50) }, 1), "");
});
test("AI brief validation matches input bounds and requires product, image and platform", () => {
  assert.match(detailBriefError({ ...draft, title: "" }, 1), /商品名称/);
  assert.match(detailBriefError(draft, 0), /图片/);
  assert.match(detailBriefError({ ...draft, platformIds: [] }, 1), /平台/);
  assert.match(detailBriefError({ ...draft, detailText: "字".repeat(3001) }, 1), /材质与细节.*3000/);
});

test("template image is required rather than a template name", () => { assert.match(detailBriefError({...draft, referenceId:"", referenceTemplateName:"户外机能"},1), /模板参考图/); });
