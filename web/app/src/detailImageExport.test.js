import test from "node:test";
import assert from "node:assert/strict";
import { containRect } from "./detailImageExport.js";
test("platform exports preserve full image without distortion", () => {
  for (const [w, h] of [[750,1000],[790,1053],[750,1200],[1000,1000],[640,640],[1416,708],[970,300]]) {
    const r = containRect(1024,1280,w,h);
    assert.ok(r.x >= 0 && r.y >= 0);
    assert.ok(r.width <= w && r.height <= h);
    assert.ok(Math.abs(r.width / r.height - .8) < 1e-10);
  }
});
