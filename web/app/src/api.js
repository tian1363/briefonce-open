import { analyticsHeaders } from "./analytics.js";
const jsonHeaders = {
  "Content-Type": "application/json; charset=utf-8",
};

export async function getAuthStatus() {
  return request("/api/auth/status");
}
export async function getCurrentUser() {
  return request("/api/auth/me");
}
export async function login(input) {
  return request("/api/auth/login", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function register(input) {
  return request("/api/auth/register", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function logout() {
  return request("/api/auth/logout", { method: "POST", headers: jsonHeaders });
}

export async function listJobs() {
  return request("/api/jobs");
}

export async function getJob(id) {
  return request(`/api/jobs/${id}`);
}

export async function createJob(formData) {
  return request("/api/jobs", {
    method: "POST",
    body: formData,
  });
}

export async function listProducts() {
  return request("/api/products");
}

export async function listSkills() {
  return request("/api/skills");
}

export async function createSkill(input) {
  return request("/api/skills", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}

export async function generateSkillDraft(requirement) {
  return request("/api/skills/draft", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ requirement }),
  });
}

export async function updateSkill(id, input) {
  return request(`/api/skills/${id}`, {
    method: "PUT",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}

export async function createProduct(formData) {
  return request("/api/products", {
    method: "POST",
    body: formData,
  });
}
export async function parseProductDocument(file) {
  const form = new FormData();
  form.append("document", file);
  return request("/api/products/parse", { method: "POST", body: form });
}

export async function getProductMarkdown(id) {
  return request(`/api/products/${id}/markdown`);
}
export async function listProductObservations(id) {
  return request(`/api/products/${id}/observations`);
}
export async function listProductAssets(id) {
  return request(`/api/products/${id}/assets`);
}
export async function listImageTransformJobs(productId) {
  return request(`/api/products/${productId}/image-transforms`);
}
export async function startImageTransformBatch(productId, input) {
  return request(`/api/products/${productId}/image-transforms`, { method: "POST", headers: jsonHeaders, body: JSON.stringify(input) });
}
export async function uploadProductAsset(id, file) {
  const form = new FormData();
  form.append("asset", file);
  return request(`/api/products/${id}/assets`, { method: "POST", body: form });
}
export async function uploadCanvasReference(id, file) {
  const form = new FormData();
  form.append("asset", file);
  form.append("purpose", "visual_reference");
  return request(`/api/products/${id}/assets`, { method: "POST", body: form });
}
export async function deleteProductAsset(id) {
  return request(`/api/assets/${id}`, { method: "DELETE" });
}
export async function updateProduct(id, input) {
  return request(`/api/products/${id}`, {
    method: "PUT",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function listSpaces() {
  return request("/api/spaces");
}
export async function assignChatSpace(id, spaceId) {
  return request(`/api/chats/${encodeURIComponent(id)}/space`, {
    method: "PUT",
    headers: jsonHeaders,
    body: JSON.stringify({ space_id: spaceId }),
  });
}
export async function listSpaceVideos(id) {
  return request(`/api/spaces/${encodeURIComponent(id)}/videos`);
}
export async function listSuggestions() {
  return request("/api/suggestions");
}
export async function updateSuggestionStatus(id, status) {
  return request(`/api/suggestions/${id}/status`, {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ status }),
  });
}
export async function createSpace(input) {
  return request("/api/spaces", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function updateSpace(id, input) {
  return request(`/api/spaces/${id}`, {
    method: "PUT",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function deleteSpace(id) {
  return request(`/api/spaces/${id}`, { method: "DELETE" });
}
export async function getSpaceObservability(id, limit = 100) {
  return request(`/api/spaces/${id}/observability?limit=${limit}`);
}
export async function getIntelligence(spaceId = "") {
  return request(
    `/api/intelligence${spaceId ? `?space_id=${encodeURIComponent(spaceId)}` : ""}`,
  );
}
export async function seedIntelligenceDemo(spaceId = "") {
  return request("/api/intelligence/demo", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ space_id: spaceId }),
  });
}
export async function promoteIntelligenceSignal(id, spaceId) {
  return request(`/api/intelligence/signals/${id}/memory`, {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ space_id: spaceId }),
  });
}
export async function updateCreativeMemory(id, input) {
  return request(`/api/intelligence/memories/${id}`, {
    method: "PUT",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function deleteCreativeMemory(id) {
  return request(`/api/intelligence/memories/${id}`, { method: "DELETE" });
}
export async function createCompetitorMonitor(input) {
  return request("/api/intelligence/competitors", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function scanCompetitorMonitor(id) {
  return request(`/api/intelligence/competitors/${id}/scan`, {
    method: "POST",
    headers: jsonHeaders,
  });
}
export async function getOwnerSession() {
  return request("/api/owner/session");
}
export async function getOwnerOverview() {
  return request("/api/owner/overview");
}
export const listInvites = () => request("/api/owner/invites");
export const createInvite = (input) => request("/api/owner/invites", {method: "POST", headers: jsonHeaders, body: JSON.stringify(input)});
export const revokeInvite = (id) => request(`/api/owner/invites/${encodeURIComponent(id)}/revoke`, {method: "POST", headers: jsonHeaders});
export const listManagedAccounts = () => request("/api/owner/accounts");
export const getPlatformProvider = () => request("/api/owner/provider");
export const savePlatformProvider = (apiKey) => request("/api/owner/provider", {method: "PUT", headers: jsonHeaders, body: JSON.stringify({api_key: apiKey})});
export const savePlatformModelProfile = (capability, input) => request(`/api/owner/provider/profiles/${encodeURIComponent(capability)}`, { method: "PUT", headers: jsonHeaders, body: JSON.stringify(input) });
export const createManagedAccount = (input) => request("/api/owner/accounts", {method: "POST", headers: jsonHeaders, body: JSON.stringify(input)});
export const updateManagedAccount = (id, input) => request(`/api/owner/accounts/${encodeURIComponent(id)}`, {method: "PATCH", headers: jsonHeaders, body: JSON.stringify(input)});
export const adjustManagedCredits = (id, input) => request(`/api/owner/accounts/${encodeURIComponent(id)}/credits`, {method: "POST", headers: jsonHeaders, body: JSON.stringify(input)});
export const resetManagedPassword = (id) => request(`/api/owner/accounts/${encodeURIComponent(id)}/reset-password`, {method: "POST", headers: jsonHeaders});
export const listManagedLedger = (id) => request(`/api/owner/accounts/${encodeURIComponent(id)}/ledger`);
export const updateCreditPrice = (capability, points) => request(`/api/owner/prices/${encodeURIComponent(capability)}`, {method: "PUT", headers: jsonHeaders, body: JSON.stringify({points})});
export const getMyCredits = () => request("/api/credits");
export const getMyCreditLedger = () => request("/api/credits/ledger");
export async function loginOwner(username, password) {
  return request("/api/owner/login", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ username, password }),
  });
}
export async function logoutOwner() {
  return request("/api/owner/logout", { method: "POST", headers: jsonHeaders });
}

export async function interpretCreative(input) {
  return request("/api/creative/interpret", { method: "POST", headers: jsonHeaders, body: JSON.stringify(input) });
}

export async function getModelSettings() {
  return request("/api/settings/model");
}

export async function saveModelSettings(settings) {
  return request("/api/settings/model", {
    method: "PUT",
    headers: jsonHeaders,
    body: JSON.stringify(settings),
  });
}

export async function publishJob(id) {
  return request(`/api/jobs/${id}/publish`, {
    method: "POST",
    headers: jsonHeaders,
  });
}

export async function retryJob(id) {
  return request(`/api/jobs/${id}/retry`, {
    method: "POST",
    headers: jsonHeaders,
  });
}

export async function generateVideoPrompts(id, source = "all") {
  return request(`/api/jobs/${id}/video-prompts`, {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ source }),
  });
}

export async function listVideos() {
  return request("/api/videos");
}
export async function createVideo(input) {
  return request("/api/videos", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function retryVideo(id) {
  return request(`/api/videos/${id}/retry`, {
    method: "POST",
    headers: jsonHeaders,
  });
}

export async function createRenderBatch(form) {
  return request("/api/render-batches", { method: "POST", body: form });
}
export async function transcribeRenderSource(input) {
  return request("/api/render-batches/transcribe", { method: "POST", headers: jsonHeaders, body: JSON.stringify(input) });
}
export async function scanRenderCaptions(input) {
  return request("/api/render-batches/scan-captions", { method: "POST", headers: jsonHeaders, body: JSON.stringify(input) });
}
export async function getRenderBatch(id) {
  return request(`/api/render-batches/${id}`);
}
export async function listRenderBatches() {
  return request("/api/render-batches");
}
export async function retryRenderVariant(batchId, variantId) {
  return request(`/api/render-batches/${batchId}/variants/${variantId}/retry`, { method: "POST" });
}

export async function listChats() {
  return request("/api/chats");
}

export async function getChat(id) {
  return request(`/api/chats/${id}`);
}

export async function getChatProgress(id) {
  return request(`/api/chats/${id}/progress`);
}

export async function sendNewChatMessage(
  content,
  productId = "",
  attachment = null,
  spaceId = "",
) {
  if (attachment) {
    const form = new FormData();
    form.append("content", content);
    form.append("product_id", productId);
    form.append("space_id", spaceId);
    form.append("attachment", attachment);
    return request("/api/chats/messages", { method: "POST", body: form });
  }
  return request("/api/chats/messages", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ content, product_id: productId, space_id: spaceId }),
  });
}

export async function sendChatMessage(
  id,
  content,
  productId = "",
  attachment = null,
) {
  if (attachment) {
    const form = new FormData();
    form.append("content", content);
    form.append("product_id", productId);
    form.append("attachment", attachment);
    return request(`/api/chats/${id}/messages`, { method: "POST", body: form });
  }
  return request(`/api/chats/${id}/messages`, {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ content, product_id: productId }),
  });
}

export async function listModelCalls(params = {}) {
  const query = new URLSearchParams();
  if (params.ref_id) query.set("ref_id", params.ref_id);
  if (params.limit) query.set("limit", String(params.limit));
  const suffix = query.toString() ? `?${query.toString()}` : "";
  return request(`/api/model-calls${suffix}`);
}

async function request(path, init) {
  const res = await fetch(path, { ...init, headers: { ...analyticsHeaders(path, init?.method), ...init?.headers } });
  if (res.status === 204) return null;
  const isJSON = (res.headers.get("content-type") || "").includes(
    "application/json",
  );
  const data = isJSON ? await res.json().catch(() => ({})) : {};
  if (!res.ok) {
    throw new Error(data.error || `Request failed: ${res.status}`);
  }
  if (!isJSON) throw new Error("服务暂时不可用，请稍后再试");
  return data;
}

export const listAssetExtractions = () => request("/api/asset-extractions");
export const getAssetExtraction = (id) => request(`/api/asset-extractions/${encodeURIComponent(id)}`);
export const startAssetExtraction = (input) => request("/api/asset-extractions", { method: "POST", headers: jsonHeaders, body: JSON.stringify(input) });
export const startAssetExtractionBatch = (input) => request("/api/asset-extractions/batch", { method: "POST", headers: jsonHeaders, body: JSON.stringify(input) });
export const startUploadedAssetExtractionBatch = (file, input) => {
 const form = new FormData(); form.append("video", file); form.append("kinds", JSON.stringify(input.kinds)); form.append("label", input.label); form.append("at_seconds", String(input.at_seconds)); form.append("image_options", JSON.stringify(input.image_options||{}));
 return request("/api/asset-extractions/batch/upload", { method: "POST", body: form });
};
export const startUploadedAssetExtraction = (file, input) => {
 const form = new FormData(); form.append("video", file); form.append("kind", input.kind); form.append("label", input.label); form.append("at_seconds", String(input.at_seconds)); form.append("image_options", JSON.stringify(input.image_options||{}));
 return request("/api/asset-extractions/upload", { method: "POST", body: form });
};
export const importAssetExtraction = (id, productId) => request(`/api/asset-extractions/${encodeURIComponent(id)}/import`, { method: "POST", headers: jsonHeaders, body: JSON.stringify({ product_id: productId }) });
export const acceptAssetExtractionReview = (id) => request(`/api/asset-extractions/${encodeURIComponent(id)}/accept-review`, { method: "POST" });
export const listPosePresets = () => request("/api/pose-presets");
export const startPoseGeneration = (input) => request("/api/pose-generations", { method: "POST", headers: jsonHeaders, body: JSON.stringify(input) });

export async function generateToolboxDraft(input, signal) {
  return request("/api/toolbox/draft", { method: "POST", headers: jsonHeaders, body: JSON.stringify(input), signal });
}

export async function createChat(title = "视频创作") {
 return request("/api/chats", {method: "POST", headers: jsonHeaders, body: JSON.stringify({title})});
}

export async function listCharacters(signal) { return request('/api/characters', { signal }); }
export async function uploadCharacter(name, file) {
 const body = new FormData(); body.append('name',name); body.append('image',file);
 return request('/api/characters',{method:'POST',body});
}
export async function generateCharacter(input) { return request('/api/characters/generate',{method:'POST',headers:jsonHeaders,body:JSON.stringify(input)}); }

export const getUsageActivity = (offset = 0) => request(`/api/credits/activity?offset=${offset}`);
export const getImageUsage = (kind, id) => request(`/api/credits/image?${kind}=${encodeURIComponent(id)}`);

export function listArtworks(productId = "", offset = 0) {
  return request(`/api/artworks?${new URLSearchParams({ product_id: productId, offset })}`);
}
export function saveDetailArtwork(productId, file) {
  const form = new FormData();
  form.append("asset", file);
  form.append("purpose", "generated_detail");
  return request(`/api/products/${productId}/assets`, { method: "POST", body: form });
}

export const generateDetailPlan = (id, input) => request(`/api/products/${id}/detail-plan`, { method: "POST", headers: jsonHeaders, body: JSON.stringify(input) });
export const previewDetailPlan = (id, plan) => request(`/api/products/${id}/detail-plan/preview`, { method: "POST", headers: jsonHeaders, body: JSON.stringify(plan) });
export const listDetailImages = (id) => request(`/api/products/${id}/detail-images`);
export const startDetailImages = (id, input) => request(`/api/products/${id}/detail-images`, { method: "POST", headers: jsonHeaders, body: JSON.stringify(input) });

export const getOperations = () => request('/api/owner/operations');
export const saveImagePrice = (price, note) => request('/api/owner/operations/prices', { method:'PUT', headers:jsonHeaders, body:JSON.stringify({price,note}) });
export const saveCreditPackage = (value, note) => request('/api/owner/operations/packages', { method:'PUT', headers:jsonHeaders, body:JSON.stringify({package:value,note}) });
export const createPurchaseOrder = (packageId) => request('/api/credits/orders', { method:'POST', headers:jsonHeaders, body:JSON.stringify({package_id:packageId}) });
export const createAdminPurchase = (userId, packageId) => request('/api/owner/operations/orders', {method:'POST',headers:jsonHeaders,body:JSON.stringify({user_id:userId,package_id:packageId})});
export const confirmPurchaseOrder = (id, receipt) => request(`/api/owner/operations/orders/${encodeURIComponent(id)}/confirm`, {method:'POST',headers:jsonHeaders,body:JSON.stringify({receipt})});
export const refundPurchaseOrder = (id, receipt) => request(`/api/owner/operations/orders/${encodeURIComponent(id)}/refund`, {method:'POST',headers:jsonHeaders,body:JSON.stringify({receipt})});
export const reconcileSupplierCall = (id, value) => request(`/api/owner/operations/calls/${encodeURIComponent(id)}/reconcile`, {method:'POST',headers:jsonHeaders,body:JSON.stringify(value)});
export const recoverSupplierCall = (id, value) => request(`/api/owner/operations/calls/${encodeURIComponent(id)}/recover`, {method:'POST',headers:jsonHeaders,body:JSON.stringify(value)});
export const getSupplierRequests = (id) => request(`/api/owner/operations/calls/${encodeURIComponent(id)}/requests`);
export const importSupplierStatements = (lines) => request('/api/owner/operations/statements', {method:'POST',headers:jsonHeaders,body:JSON.stringify({lines})});
export const matchSupplierStatement = (id, jobId) => request(`/api/owner/operations/statements/${encodeURIComponent(id)}/match`, {method:'POST',headers:jsonHeaders,body:JSON.stringify({job_id:jobId})});
export const addSupplierDeposit = (value) => request('/api/owner/operations/deposits', {method:'POST',headers:jsonHeaders,body:JSON.stringify(value)});
export const listCreditPackages = () => request('/api/credits/packages');
export const listPurchaseOrders = () => request('/api/credits/orders');
export const getImageCatalog = () => request('/api/image-models');
export const getImageQuote = (options) => request('/api/image-models/quote', {method:'POST',headers:jsonHeaders,body:JSON.stringify(options)});
export const listProviderImageTasks = () => request('/api/image-provider-jobs');

export const resolveSupplierCall = (id, value) => request(`/api/owner/operations/calls/${encodeURIComponent(id)}/resolve`, {method:"POST",headers:jsonHeaders,body:JSON.stringify(value)});

export const getAccountUsageActivity = (id, offset = 0) => request(`/api/owner/accounts/${encodeURIComponent(id)}/activity?offset=${offset}`);

export const detachProductAsset = (id) => request(`/api/assets/${encodeURIComponent(id)}/detach`, { method: "POST" });

export async function replaceProductAsset(id,file) { const form=new FormData();form.append("asset",file);return request(`/api/assets/${encodeURIComponent(id)}/replace`,{method:"POST",body:form}); }
export const getVideoModels = (seconds=5,resolution='default',videoReference=false) => request(`/api/video-models?seconds=${seconds}&resolution=${encodeURIComponent(resolution)}&video_reference=${videoReference}`);
export const saveVideoPrice = (price,note) => request('/api/owner/operations/video-prices',{method:'PUT',headers:jsonHeaders,body:JSON.stringify({price,note})});

export function getCreativeConnection() { return request("/api/creative/connection"); }
export function connectCreativeSource() { return request("/api/creative/connection", { method: "POST", headers: jsonHeaders }); }
export function disconnectCreativeSource() { return request("/api/creative/connection", { method: "DELETE", headers: jsonHeaders }); }
export function readCreativeNote(source_url) { return request("/api/creative/read", { method: "POST", headers: jsonHeaders, body: JSON.stringify({ source_url }) }); }

export function appendProductReference(id, content) { return request(`/api/products/${id}/references`, { method: "POST", headers: jsonHeaders, body: JSON.stringify({content}) }); }

export const getOperationsAnalytics = (days = 30) => request(`/api/owner/analytics?days=${days}`);
