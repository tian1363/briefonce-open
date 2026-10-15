// Only controlled identifiers are sent. No prompt, asset URL or document content.
let identity = "";
let session = "";
let entry = "navigation";
let lastPage = "";
let lastInteraction = 0;
function touchSession() { const now=Date.now(); if (now-lastInteraction>30*60*1000) session=crypto.randomUUID(); lastInteraction=now; }
export function setAnalyticsUser(id) {
  if (identity === id) return;
  identity = id || "";
  session = identity ? crypto.randomUUID() : "";
  lastPage = "";
  lastInteraction = Date.now();
}
export function analyticsContext(next) { entry = next; }
export function analyticsHeaders(path = "", method = "GET") {
  if (!identity || location.pathname.startsWith("/admin")) return {};
  if (method !== "GET" && !path.includes("/analytics")) touchSession();
  return { "X-Creative-Session": session, "X-Creative-Entry": entry };
}
export function trackEvent(name, feature, origin = entry) {
  if (!identity || location.pathname.startsWith("/admin") || document.visibilityState === "hidden") return;
  touchSession();
  // Analytics is best effort and must never block an action.
  void fetch("/api/analytics/events", { method: "POST", credentials: "same-origin", keepalive: true,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id: crypto.randomUUID(), name, feature, entry: origin, session_id: session }),
  }).catch(() => {});
}
export function trackPage(view) {
  const feature = ["home","spaces","products","toolbox","analytics","history","chat"].includes(view) ? view : "other";
  if (lastPage === view) return;
  lastPage = view;
  analyticsContext(view === "chat" || view === "home" ? "composer" : "navigation");
  trackEvent("page_view", feature, "navigation");
}

export function installDownloadTracking() {
  const listener = (event) => {
    if (!identity || location.pathname.startsWith("/admin")) return;
    const anchor = event.target.closest?.("a[download]");
    if (!anchor) return;
    const url = new URL(anchor.href, location.href);
    if (url.origin !== location.origin || !url.pathname.startsWith("/api/")) return;
    url.searchParams.set("download", "1");
    anchor.href = url.href;
  };
  document.addEventListener("click", listener, true);
  return () => document.removeEventListener("click", listener, true);
}
