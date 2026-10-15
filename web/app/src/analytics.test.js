import test from "node:test";
import assert from "node:assert/strict";
import { setAnalyticsUser, trackPage, trackEvent, analyticsHeaders } from "./analytics.js";

test("analytics stays account-scoped, deduplicates page views and never submits content", async () => {
  const original = { fetch: globalThis.fetch, document: globalThis.document, location: globalThis.location };
  const requests = [];
  globalThis.document = { visibilityState: "visible" };
  globalThis.location = { pathname: "/" };
  globalThis.fetch = async (url, init) => { requests.push({url, event: JSON.parse(init.body)}); return {}; };
  try {
    setAnalyticsUser("user-a"); trackPage("home"); trackPage("home");
    assert.equal(requests.length, 1);
    const first = analyticsHeaders();
    assert.equal(requests[0].event.name, "page_view");
    assert.deepEqual(Object.keys(requests[0].event).sort(), ["entry", "feature", "id", "name", "session_id"]);
    assert.equal(JSON.stringify(requests).includes("user-a"), false);
    setAnalyticsUser("user-b"); trackPage("home");
    assert.notEqual(analyticsHeaders()["X-Creative-Session"], first["X-Creative-Session"]);
    assert.equal(requests.length, 2);
    globalThis.location.pathname = "/admin/users/example/view";
    trackEvent("result_preview", "other"); assert.equal(requests.length, 2);
    assert.deepEqual(analyticsHeaders(), {});
    globalThis.location.pathname = "/";
    globalThis.fetch = () => Promise.reject(new Error("offline"));
    assert.doesNotThrow(() => trackEvent("feature_open", "pose"));
    await new Promise((resolve) => setTimeout(resolve, 0));
  } finally { setAnalyticsUser(""); Object.assign(globalThis, original); }
});
