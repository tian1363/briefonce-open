package web

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/tian1363/scriptagent/internal/jobs"
	"io"
	"net/http"
	"strings"
)

func decodeOps(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		writeError(w, 400, errors.New("参数格式无效，请检查输入字段"))
		return false
	}
	if d.Decode(&struct{}{}) != io.EOF {
		writeError(w, 400, errors.New("只能提交一份请求参数"))
		return false
	}
	return true
}
func (h *Handler) opsOverview(w http.ResponseWriter, r *http.Request) {
	summary, err := h.store.RevenueSummary()
	if err != nil {
		writeError(w, 500, err)
		return
	}
	prices, err := h.store.ImagePrices()
	if err != nil {
		writeError(w, 500, err)
		return
	}
	videoPrices, err := h.store.VideoPrices()
	if err != nil {
		writeError(w, 500, err)
		return
	}
	packages, err := h.store.Packages(false)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	orders, err := h.store.Orders("")
	if err != nil {
		writeError(w, 500, err)
		return
	}
	calls, err := h.store.ProviderImageRecords()
	if err != nil {
		writeError(w, 500, err)
		return
	}
	statements, err := h.store.Statements()
	if err != nil {
		writeError(w, 500, err)
		return
	}
	audit, err := h.store.OperationsAudit()
	if err != nil {
		writeError(w, 500, err)
		return
	}
	deposits, err := h.store.SupplierDeposits()
	if err != nil {
		writeError(w, 500, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"summary": summary, "prices": prices, "video_prices": videoPrices, "packages": packages, "orders": orders, "calls": calls, "statements": statements, "audit": audit, "deposits": deposits})
}
func (h *Handler) opsPrice(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Price jobs.ImagePrice `json:"price"`
		Note  string          `json:"note"`
	}
	if !decodeOps(w, r, &in) {
		return
	}
	if err := h.store.SaveImagePrice(in.Price, userIDFromRequest(r), in.Note); err != nil {
		writeError(w, 400, err)
		return
	}
	h.opsOverview(w, r)
}
func (h *Handler) opsPackage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Package jobs.Package `json:"package"`
		Note    string       `json:"note"`
	}
	if !decodeOps(w, r, &in) {
		return
	}
	if err := h.store.SavePackage(in.Package, userIDFromRequest(r), in.Note); err != nil {
		writeError(w, 400, err)
		return
	}
	h.opsOverview(w, r)
}
func (h *Handler) opsConfirm(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Receipt string `json:"receipt"`
	}
	if !decodeOps(w, r, &in) {
		return
	}
	if err := h.store.ConfirmPurchase(chi.URLParam(r, "id"), in.Receipt, userIDFromRequest(r)); err != nil {
		writeError(w, 400, err)
		return
	}
	h.opsOverview(w, r)
}
func (h *Handler) opsRefund(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Receipt string `json:"receipt"`
	}
	if !decodeOps(w, r, &in) {
		return
	}
	if err := h.store.RefundPurchase(chi.URLParam(r, "id"), userIDFromRequest(r), in.Receipt); err != nil {
		writeError(w, 400, err)
		return
	}
	h.opsOverview(w, r)
}
func (h *Handler) opsReconcile(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Cost     int64  `json:"cost_micros"`
		Refund   int64  `json:"refund_micros"`
		Evidence string `json:"evidence"`
	}
	if !decodeOps(w, r, &in) {
		return
	}
	if err := h.store.ReconcileProviderImage(chi.URLParam(r, "id"), userIDFromRequest(r), in.Evidence, in.Cost, in.Refund); err != nil {
		writeError(w, 400, err)
		return
	}
	h.opsOverview(w, r)
}
func (h *Handler) opsRequests(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ProviderRequestRecords(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 500, err)
		return
	}
	writeJSON(w, 200, items)
}
func (h *Handler) opsImport(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Lines []jobs.StatementLine `json:"lines"`
	}
	if !decodeOps(w, r, &in) {
		return
	}
	n, err := h.store.ImportStatements(in.Lines, userIDFromRequest(r))
	if err != nil {
		writeError(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]int{"inserted": n})
}
func (h *Handler) opsMatch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		JobID string `json:"job_id"`
	}
	if !decodeOps(w, r, &in) {
		return
	}
	if err := h.store.MatchStatement(chi.URLParam(r, "id"), in.JobID, userIDFromRequest(r)); err != nil {
		writeError(w, 400, err)
		return
	}
	h.opsOverview(w, r)
}
func (h *Handler) opsDeposit(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Account string `json:"account"`
		Receipt string `json:"receipt"`
		Amount  int64  `json:"amount_micros"`
		Bonus   int64  `json:"bonus_micros"`
	}
	if !decodeOps(w, r, &in) {
		return
	}
	if err := h.store.AddSupplierDeposit(in.Account, in.Receipt, userIDFromRequest(r), in.Amount, in.Bonus); err != nil {
		writeError(w, 400, err)
		return
	}
	h.opsOverview(w, r)
}
func (h *Handler) opsCreateOrder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID    string `json:"user_id"`
		PackageID string `json:"package_id"`
	}
	if !decodeOps(w, r, &in) {
		return
	}
	id, err := h.store.CreatePurchase(in.UserID, in.PackageID)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}
func (h *Handler) myPackages(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.Packages(true)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	writeJSON(w, 200, items)
}
func (h *Handler) myOrders(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.Orders(userIDFromRequest(r))
	if err != nil {
		writeError(w, 500, err)
		return
	}
	for _, o := range items {
		delete(o, "receipt")
		delete(o, "fee_micros")
	}
	writeJSON(w, 200, items)
}
func (h *Handler) createMyOrder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PackageID string `json:"package_id"`
	}
	if !decodeOps(w, r, &in) {
		return
	}
	id, err := h.store.CreatePurchase(userIDFromRequest(r), in.PackageID)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	writeJSON(w, 201, map[string]string{"id": id, "status": "pending", "message": "请联系管理员线下付款，确认收款后积分到账"})
}
func csvSafe(v any) string {
	s := fmt.Sprint(v)
	if v == nil {
		return ""
	}
	if strings.ContainsAny(strings.TrimLeft(s, " \t\r\n")[:min(1, len(strings.TrimLeft(s, " \t\r\n")))], "=+-@") {
		s = "'" + s
	}
	return s
}
func (h *Handler) opsExport(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ExportProviderImages()
	if err != nil {
		writeError(w, 500, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=supplier-usage.csv")
	_, _ = w.Write([]byte("\xef\xbb\xbf"))
	c := csv.NewWriter(w)
	cols := []string{"id", "created_at", "email", "provider", "account_ref", "key_ref", "model", "upstream_id", "status", "requests", "estimated_micros", "actual_micros", "refund_micros", "reconcile_status", "points", "billing_status", "evidence"}
	_ = c.Write([]string{"任务编号", "创建时间", "用户邮箱", "供应商", "供应商账号", "Key指纹", "模型", "上游任务ID", "任务状态", "请求次数", "预计成本（元）", "实际支出（元）", "供应商退款（元）", "对账状态", "消耗积分", "积分状态", "账单凭证"})
	for _, item := range items {
		row := []string{}
		for _, col := range cols {
			value := item[col]
			if (col == "estimated_micros" || col == "actual_micros" || col == "refund_micros") && value != nil {
				if n, ok := value.(int64); ok {
					value = fmt.Sprintf("%.6f", float64(n)/1000000)
				}
			}
			row = append(row, csvSafe(value))
		}
		_ = c.Write(row)
	}
	c.Flush()
}
func (h *Handler) opsRecover(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UpstreamID string `json:"upstream_id"`
		Note       string `json:"note"`
	}
	if !decodeOps(w, r, &in) {
		return
	}
	if in.UpstreamID == "" || len(in.UpstreamID) > 200 || in.Note == "" {
		writeError(w, 400, errors.New("请填写供应商任务 ID 和核对依据"))
		return
	}
	if err := h.store.RecoverProviderImage(chi.URLParam(r, "id"), in.UpstreamID, userIDFromRequest(r), in.Note); err != nil {
		writeError(w, 400, err)
		return
	}
	h.opsOverview(w, r)
}

func (h *Handler) opsResolve(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Evidence string `json:"evidence"`
		Cost     int64  `json:"cost_micros"`
	}
	if !decodeOps(w, r, &in) {
		return
	}
	id := chi.URLParam(r, "id")
	j, err := h.store.ProviderImageForResolution(id)
	if err != nil {
		writeError(w, 404, errors.New("任务不存在"))
		return
	}
	if err = h.store.ResolveUncertainImage(id, userIDFromRequest(r), in.Evidence, in.Cost); err != nil {
		writeError(w, 400, err)
		return
	}
	h.failProviderImage(j, "管理员已核对未生成，积分已退回")
	h.opsOverview(w, r)
}
