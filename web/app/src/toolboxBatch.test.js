import test from "node:test";
import assert from "node:assert/strict";
import { createVersion, initialVersions, batchSummary, versionIssues, mergeImportedVersions, parseBulkCopies, restoreBatch, splitRecognizedSegments, compareVisualCaptions } from "./toolboxBatch.js";

test("batch totals ignore unselected incomplete and duplicated rows", () => {
  const one = createVersion({ text: "新品\n即刻体验" });
  const duplicate = createVersion({ text: "新品 即刻体验" });
  const missing = createVersion({ selected: false });
  const result = batchSummary("copy", [one, duplicate, missing]);
  assert.equal(result.selected.length, 2);
  assert.equal(result.incomplete, 0);
  assert.equal(result.duplicateIds.size, 2);
  duplicate.selected = false;
  assert.equal(batchSummary("copy", [one, duplicate, missing]).duplicateIds.size, 0);
});

test("same reference with different instructions counts as differentiated", () => {
  const target = { id: "image-1" };
  const rows = [createVersion({ target, notes: "自然微笑" }), createVersion({ target, notes: "认真介绍" })];
  assert.equal(batchSummary("model", rows).duplicateIds.size, 0);
  assert.equal(versionIssues("product", createVersion({ target })).length, 1);
  assert.equal(versionIssues("product", createVersion({ target, adapt: false })).length, 0);
});

test("bulk imports replace empty placeholders and preserve existing edited versions", () => {
  const edited = createVersion({ text: "已编辑内容", selected: false });
  const rows = [edited, createVersion(), createVersion()];
  const next = mergeImportedVersions(rows, [{ text: "新文案" }]);
  assert.equal(next.length, 2);
  assert.equal(next[0], edited);
  assert.equal(next[1].text, "新文案");
  assert.equal(rows.length, 3);
  assert.throws(() => mergeImportedVersions([edited], Array.from({ length: 30 }, () => ({ text: "test" }))), /最多/);
});

test("bulk copy retains paragraph breaks and rejects oversized entries", () => {
  assert.deepEqual(parseBulkCopies("第一段\n第二段\n---\n另一版"), [{ text: "第一段\n第二段" }, { text: "另一版" }]);
  assert.throws(() => parseBulkCopies("a".repeat(3001)));
  assert.throws(() => parseBulkCopies("---"));
});

test("legacy saved plans remain restorable, and new batches preserve all versions", () => {
  const legacy = restoreBatch({ forms: { copy: { text: "旧方案" } } });
  assert.equal(legacy.copy[0].text, "旧方案");
  const versionsByMode = initialVersions();
  assert.equal(restoreBatch({ versionsByMode }), versionsByMode);
  assert.equal(new Set(Object.values(versionsByMode).flat().map((row) => row.id)).size, 3);
  for (const rows of Object.values(versionsByMode)) assert.equal(rows.length, 1);
  const existing = { copy: Array.from({ length: 12 }, (_, i) => createVersion({ text: `文案 ${i}` })) };
  assert.equal(restoreBatch({ versionsByMode: existing }).copy.length, 12);
});

test("timed subtitles validate ordering and compare sentence edits", () => {
  const first = createVersion({ segments: [{ start: 0, end: 2, text: "第一句" }, { start: 2, end: 4, text: "第二句" }] });
  const edited = createVersion({ segments: [{ start: 0, end: 2, text: "第一句" }, { start: 2, end: 4, text: "新文案" }] });
  assert.deepEqual(versionIssues("copy", first), []);
  assert.equal(batchSummary("copy", [first, edited]).duplicateIds.size, 0);
  edited.segments[1].start = 1.5;
  assert.match(versionIssues("copy", edited).join(" "), /逐句时间/);
});

test("recognized speech splits into reviewable sentences with ordered times", () => {
  const parts = splitRecognizedSegments([{ start: 0, end: 4, text: "第一句。第二句！" }]);
  assert.equal(parts.length, 2);
  assert.deepEqual(parts.map((item) => item.text), ["第一句。", "第二句！"]);
  assert.equal(parts[0].start, 0);
  assert.equal(parts[1].end, 4);
  assert.equal(parts[0].end, parts[1].start);
  assert.equal(parts[0].needs_review, true);
});

test("visual captions flag a likely mismatch beside the spoken sentence", () => {
  const spoken = [{ start: 0, end: 2, text: "新鲜果汁", source: "speech", needs_review: false }];
  const compared = compareVisualCaptions(spoken, [{ start: 0, end: 2, text: "鲜榨果汁", source: "visual" }]);
  assert.equal(compared[0].visual_text, "鲜榨果汁");
  assert.equal(compared[0].needs_review, true);
  assert.match(compared[0].review_reason, /不一致/);
  assert.equal(spoken[0].needs_review, false);
});
