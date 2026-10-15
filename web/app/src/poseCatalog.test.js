import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, existsSync } from "node:fs";
import { filterPoses, poseValues } from "./poseCatalog.js";
const poses = JSON.parse(readFileSync(new URL("../../../internal/web/pose_catalog.json", import.meta.url)));
test("catalog preserves the audited visible collection and preview availability", () => {
  assert.equal(poses.length, 568);
  assert.equal(new Set(poses.map((pose) => pose.id)).size, 568);
  assert.equal(poses.filter((pose) => pose.image_url).length, 568);
  for (const pose of poses) {
    assert.equal(pose.image_url, `/pose-resource-gallery/${pose.id}.png`);
    assert.ok(existsSync(new URL(`../public${pose.image_url}`, import.meta.url)), `Missing preview: ${pose.id}`);
  }
  assert.equal(poses.filter((pose) => pose.gender === "女装").length, 399);
  assert.equal(poses.filter((pose) => pose.shot === "全身").length, 407);
  assert.equal(poses.filter((pose) => pose.source_url_type === "搜索结果页").length, 185);
});
test("multi-tag filters match individual tags and combine with keyword search", () => {
  assert.equal(filterPoses(poses, "xhs-005", { action_tags: "基础站姿", clothing_tags: "牛仔装", gender: "女装" })[0].id, "xhs-005");
  assert.equal(filterPoses(poses, "xhs-005", { gender: "男装" }).length, 0);
  const result = filterPoses(poses, "", { style_tags: "自然松弛", shot: "全身" });
  assert.ok(result.length > 0);
  assert.ok(result.every((pose) => poseValues(pose, "style_tags").includes("自然松弛") && pose.shot === "全身"));
  assert.deepEqual(filterPoses(poses, "肯定不存在的姿势词", {}), []);
});
