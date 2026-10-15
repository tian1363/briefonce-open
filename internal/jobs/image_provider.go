package jobs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/tian1363/scriptagent/internal/charactergen"
	"github.com/tian1363/scriptagent/internal/model"
	"strings"
)

type ProviderImageJob struct {
	ID              string `json:"id"`
	UserID          string `json:"user_id"`
	Kind            string `json:"kind"`
	ProductID       string `json:"product_id"`
	ChargeID        string `json:"charge_id"`
	Capability      string `json:"capability"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	Endpoint        string `json:"endpoint"`
	KeyRef          string `json:"key_ref"`
	Credential      string `json:"-"`
	RequestJSON     string `json:"-"`
	PriceJSON       string `json:"price_json"`
	EstimatedMicros int64  `json:"estimated_micros"`
	UpstreamID      string `json:"upstream_id"`
	Status          string `json:"status"`
	Progress        int    `json:"progress"`
	ResultURL       string `json:"-"`
	SavedPath       string `json:"-"`
	AssetID         string `json:"asset_id"`
	Error           string `json:"error"`
}

func (s *Store) QueueProviderImage(j ProviderImageJob, cfg model.RuntimeConfig, body map[string]any, o charactergen.Options) error {
	if j.ID == "" || j.UserID == "" || cfg.Provider != "zexapi" {
		return errors.New("图片任务信息无效")
	}
	p, _, err := s.ImageQuote(o)
	if err != nil {
		return err
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	key, err := s.encryptSecret(cfg.APIKey)
	if err != nil {
		return err
	}
	payload, err := s.encryptSecret(string(b))
	if err != nil {
		return err
	}
	pb, _ := json.Marshal(p)
	hash := sha256.Sum256([]byte(cfg.APIKey))
	keyRef := hex.EncodeToString(hash[:6])
	accountRef := strings.TrimSpace(cfg.SupplierAccount)
	if accountRef == "" {
		accountRef = keyRef
	}
	estimate := p.CostMicros
	if cfg.Source != "managed" {
		estimate = 0
	}
	// Reservation pricing is authoritative if prices changed during task creation.
	if j.ChargeID != "" {
		var snapshot string
		if err = s.db.QueryRow(`SELECT pricing_json FROM credit_charges WHERE id=? AND user_id=?`, j.ChargeID, j.UserID).Scan(&snapshot); err != nil {
			return err
		}
		if snapshot != "" {
			if json.Unmarshal([]byte(snapshot), &p) != nil {
				return errors.New("积分价格快照无效")
			}
			pb = []byte(snapshot)
			estimate = p.CostMicros
		}
	}
	_, err = s.db.Exec(`INSERT INTO image_provider_jobs(id,user_id,kind,product_id,charge_id,capability,provider,model,endpoint,key_ref,credential,request_json,price_json,estimated_micros,source,account_ref,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, j.ID, j.UserID, j.Kind, j.ProductID, j.ChargeID, j.Capability, cfg.Provider, cfg.Model, cfg.Endpoint, keyRef, key, payload, string(pb), estimate, cfg.Source, accountRef, nowUTC(), nowUTC())
	if err != nil {
		return err
	}
	if j.ChargeID != "" {
		_, err = s.db.Exec(`UPDATE credit_charges SET provider_job_id=?,model_name=? WHERE id=? AND user_id=?`, j.ID, cfg.Model, j.ChargeID, j.UserID)
	}
	return err
}
func (s *Store) PendingProviderImages() ([]ProviderImageJob, error) {
	rows, err := s.db.Query(`SELECT id,user_id,kind,product_id,charge_id,capability,provider,model,endpoint,key_ref,credential,request_json,price_json,estimated_micros,upstream_id,status,progress,result_url,saved_path,asset_id,error FROM image_provider_jobs WHERE status NOT IN ('completed','failed','uncertain') ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	out := []ProviderImageJob{}
	for rows.Next() {
		var j ProviderImageJob
		if err = rows.Scan(&j.ID, &j.UserID, &j.Kind, &j.ProductID, &j.ChargeID, &j.Capability, &j.Provider, &j.Model, &j.Endpoint, &j.KeyRef, &j.Credential, &j.RequestJSON, &j.PriceJSON, &j.EstimatedMicros, &j.UpstreamID, &j.Status, &j.Progress, &j.ResultURL, &j.SavedPath, &j.AssetID, &j.Error); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, j)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Credential, err = s.decryptSecret(out[i].Credential)
		if err != nil {
			return nil, err
		}
		out[i].RequestJSON, err = s.decryptSecret(out[i].RequestJSON)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func (s *Store) UpdateProviderImage(id, status, upstream string, progress int, url, message string) error {
	_, err := s.db.Exec(`UPDATE image_provider_jobs SET status=?,upstream_id=CASE WHEN ?<>'' THEN ? ELSE upstream_id END,progress=?,result_url=CASE WHEN ?<>'' THEN ? ELSE result_url END,error=?,updated_at=? WHERE id=?`, status, upstream, upstream, progress, url, url, message, nowUTC(), id)
	return err
}
func (s *Store) SaveProviderImagePath(id, path, asset string) error {
	_, err := s.db.Exec(`UPDATE image_provider_jobs SET saved_path=?,asset_id=?,updated_at=? WHERE id=?`, path, asset, nowUTC(), id)
	return err
}
func (s *Store) RecordProviderRequest(id, kind string, status int, requestID, message string) error {
	_, err := s.db.Exec(`INSERT INTO provider_requests VALUES(?,?,?,?,?,?,?)`, newID(), id, kind, status, requestID, message, nowUTC())
	return err
}
func (s *Store) ProviderImageRecords() ([]map[string]any, error) {
	items, err := s.queryMaps(`SELECT i.request_json AS request_snapshot,i.id,i.user_id,u.email,i.kind,i.product_id,i.charge_id,i.capability,i.provider,i.source,i.account_ref,i.model,i.key_ref,i.price_json,i.estimated_micros,i.actual_micros,i.refund_micros,i.evidence,i.reconcile_status,i.upstream_id,i.status,i.progress,i.error,i.created_at,i.updated_at,COALESCE(c.points,0) AS points,c.status AS billing_status,(SELECT COUNT(*) FROM provider_requests r WHERE r.job_id=i.id) AS requests FROM image_provider_jobs i JOIN users u ON u.id=i.user_id LEFT JOIN credit_charges c ON c.id=i.charge_id ORDER BY i.created_at DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		snapshot, _ := item["request_snapshot"].(string)
		price, _ := item["price_json"].(string)
		item["parameters"] = usagePriceTier(s.usageParameters(snapshot), price)
		delete(item, "request_snapshot")
	}
	return items, nil
}

// Expose only invocation controls. Credentials and embedded reference images never leave storage.
func (s *Store) usageParameters(snapshot string) map[string]any {
	raw, err := s.decryptSecret(snapshot)
	if err != nil {
		return nil
	}
	var body map[string]any
	if json.Unmarshal([]byte(raw), &body) != nil {
		return nil
	}
	out := map[string]any{}
	for _, k := range []string{"model", "prompt", "negative_prompt", "size", "image_size", "aspect_ratio", "quality", "resolution", "seed", "n", "response_format"} {
		if v, ok := body[k]; ok {
			out[k] = v
		}
	}
	if refs, ok := body["images"].([]any); ok {
		out["reference_count"] = len(refs)
	}
	return out
}
func (s *Store) ProviderRequestRecords(id string) ([]map[string]any, error) {
	return s.queryMaps(`SELECT * FROM provider_requests WHERE job_id=? ORDER BY created_at`, id)
}
func (s *Store) ReconcileProviderImage(id, actor, evidence string, cost, refund int64) error {
	if cost < 0 || refund < 0 || refund > cost || cost > 100000000000 || strings.TrimSpace(evidence) == "" {
		return errors.New("实际金额或账单凭证无效")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old string
	if err = tx.QueryRow(`SELECT reconcile_status FROM image_provider_jobs WHERE id=?`, id).Scan(&old); err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE image_provider_jobs SET actual_micros=?,refund_micros=?,evidence=?,reconcile_status='reconciled',updated_at=? WHERE id=?`, cost, refund, evidence, nowUTC(), id)
	if err != nil {
		return err
	}
	if err = auditTx(tx, actor, "supplier_reconcile", id, old, map[string]any{"cost_micros": cost, "refund_micros": refund}, evidence); err != nil {
		return err
	}
	return tx.Commit()
}

type StatementLine struct {
	Account      string `json:"account"`
	ExternalID   string `json:"external_id"`
	TaskID       string `json:"task_id"`
	AmountMicros int64  `json:"amount_micros"`
	RefundMicros int64  `json:"refund_micros"`
	Evidence     string `json:"evidence"`
}

func (s *Store) ImportStatements(lines []StatementLine, actor string) (int, error) {
	if len(lines) == 0 || len(lines) > 500 {
		return 0, errors.New("单批账单需为 1–500 行")
	}
	for _, l := range lines {
		if l.Account == "" || l.ExternalID == "" || l.AmountMicros < 0 || l.AmountMicros > 100000000000 || l.RefundMicros < 0 || l.RefundMicros > l.AmountMicros || l.Evidence == "" {
			return 0, errors.New("账单行缺少账号、唯一编号、人民币金额或凭证")
		}
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	count := 0
	for _, l := range lines {
		var task, evidence string
		var cost, refund int64
		lookup := tx.QueryRow(`SELECT task_id,amount_micros,refund_micros,evidence FROM supplier_statements WHERE account=? AND external_id=?`, l.Account, l.ExternalID).Scan(&task, &cost, &refund, &evidence)
		if lookup == nil && (task != l.TaskID || cost != l.AmountMicros || refund != l.RefundMicros || evidence != l.Evidence) {
			return 0, errors.New("相同账单编号的内容发生变化，请人工核对，不可覆盖")
		}
		res, err := tx.Exec(`INSERT OR IGNORE INTO supplier_statements(id,account,external_id,task_id,amount_micros,refund_micros,evidence,created_at) VALUES(?,?,?,?,?,?,?,?)`, newID(), l.Account, l.ExternalID, l.TaskID, l.AmountMicros, l.RefundMicros, l.Evidence, nowUTC())
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		count += int(n)
	}
	if err = auditTx(tx, actor, "statement_import", "", nil, map[string]int{"inserted": count}, "人民币账单导入"); err != nil {
		return 0, err
	}
	return count, tx.Commit()
}
func (s *Store) Statements() ([]map[string]any, error) {
	return s.queryMaps(`SELECT s.*,(SELECT COUNT(*) FROM image_provider_jobs i WHERE i.upstream_id=s.task_id AND i.account_ref=s.account) AS candidates FROM supplier_statements s ORDER BY created_at DESC LIMIT 500`)
}
func (s *Store) MatchStatement(id, jobID, actor string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var evidence, account, taskID string
	var cost, refund int64
	if err = tx.QueryRow(`SELECT evidence,account,task_id,amount_micros,refund_micros FROM supplier_statements WHERE id=? AND match_id=''`, id).Scan(&evidence, &account, &taskID, &cost, &refund); err != nil {
		return errors.New("账单已匹配或不存在")
	}
	var keyRef, upstream string
	if err = tx.QueryRow(`SELECT account_ref,upstream_id FROM image_provider_jobs WHERE id=?`, jobID).Scan(&keyRef, &upstream); err != nil {
		return err
	}
	if keyRef != account || upstream != taskID {
		return errors.New("供应商账号指纹或上游任务 ID 不匹配")
	}
	var candidateCount int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM image_provider_jobs WHERE account_ref=? AND upstream_id=?`, account, taskID).Scan(&candidateCount); err != nil {
		return err
	}
	if candidateCount != 1 {
		return errors.New("此上游任务对应多个平台调用，请在任务详情人工核对")
	}
	var n int
	_ = tx.QueryRow(`SELECT COUNT(*) FROM supplier_statements WHERE match_id=?`, jobID).Scan(&n)
	if n > 0 {
		return errors.New("任务已有匹配账单，额外费用请人工核对")
	}
	_, err = tx.Exec(`UPDATE supplier_statements SET match_id=? WHERE id=?`, jobID, id)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE image_provider_jobs SET actual_micros=?,refund_micros=?,evidence=?,reconcile_status='reconciled',updated_at=? WHERE id=?`, cost, refund, evidence, nowUTC(), jobID)
	if err != nil {
		return err
	}
	if err = auditTx(tx, actor, "statement_match", id, nil, jobID, evidence); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) AddSupplierDeposit(account, receipt, actor string, amount, bonus int64) error {
	if account == "" || receipt == "" || amount <= 0 || amount > 100000000000 || bonus < 0 || bonus > 100000000000 {
		return errors.New("供应商充值信息无效")
	}
	_, err := s.db.Exec(`INSERT INTO supplier_deposits VALUES(?,?,?,?,?,?,?)`, newID(), account, amount, bonus, receipt, actor, nowUTC())
	return err
}
func (s *Store) SupplierDeposits() ([]map[string]any, error) {
	return s.queryMaps(`SELECT * FROM supplier_deposits ORDER BY created_at DESC LIMIT 200`)
}
func (s *Store) RecoverProviderImage(id, upstream, actor, note string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE image_provider_jobs SET upstream_id=?,status='queued',error='',updated_at=? WHERE id=? AND status='uncertain'`, upstream, nowUTC(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return errors.New("只可找回待核对的提交")
	}
	if err = auditTx(tx, actor, "recover_image", id, "uncertain", upstream, note); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) ProviderImageResult(id, userID string) (string, string, string, error) {
	var status, path, message string
	err := s.db.QueryRow(`SELECT status,saved_path,error FROM image_provider_jobs WHERE id=? AND user_id=?`, id, userID).Scan(&status, &path, &message)
	return status, path, message, err
}

func (s *Store) UserProviderImages(userID string) ([]map[string]any, error) {
	return s.queryMaps(`SELECT i.id,i.status,i.progress,i.model,i.error,i.created_at,COALESCE(c.points,0) AS points,c.status AS billing_status FROM image_provider_jobs i LEFT JOIN credit_charges c ON c.id=i.charge_id WHERE i.user_id=? ORDER BY i.created_at DESC LIMIT 200`, userID)
}
func (s *Store) ExportProviderImages() ([]map[string]any, error) {
	return s.queryMaps(`SELECT i.id,i.created_at,u.email,i.provider,i.account_ref,i.key_ref,i.model,i.upstream_id,i.status,(SELECT COUNT(*) FROM provider_requests r WHERE r.job_id=i.id) AS requests,i.estimated_micros,i.actual_micros,i.refund_micros,i.reconcile_status,COALESCE(c.points,0) AS points,c.status AS billing_status,i.evidence FROM image_provider_jobs i JOIN users u ON u.id=i.user_id LEFT JOIN credit_charges c ON c.id=i.charge_id ORDER BY i.created_at DESC`)
}

func (s *Store) ProviderImageForResolution(id string) (ProviderImageJob, error) {
	var j ProviderImageJob
	err := s.db.QueryRow(`SELECT id,user_id,kind,charge_id,upstream_id,status FROM image_provider_jobs WHERE id=?`, id).Scan(&j.ID, &j.UserID, &j.Kind, &j.ChargeID, &j.UpstreamID, &j.Status)
	return j, err
}
func (s *Store) ResolveUncertainImage(id, actor, evidence string, cost int64) error {
	if strings.TrimSpace(evidence) == "" || cost < 0 || cost > 100000000000 {
		return errors.New("请填写核对凭证与实际支出")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE image_provider_jobs SET status='failed',actual_micros=?,evidence=?,reconcile_status='reconciled',error='管理员已核对未生成',updated_at=? WHERE id=? AND status='uncertain'`, cost, evidence, nowUTC(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return errors.New("只可处理待核对的任务")
	}
	if err = auditTx(tx, actor, "resolve_uncertain", id, "uncertain", "failed", evidence); err != nil {
		return err
	}
	return tx.Commit()
}

// Asset insertion and task linkage commit together, so restart cannot duplicate it.
func (s *Store) CreateProviderImageAsset(jobID string, asset ProductAsset) (*ProductAsset, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var existing, product, path string
	if err = tx.QueryRow(`SELECT asset_id,product_id,saved_path FROM image_provider_jobs WHERE id=?`, jobID).Scan(&existing, &product, &path); err != nil {
		return nil, err
	}
	if existing != "" {
		return &ProductAsset{ID: existing}, nil
	}
	if product == "" || product != asset.ProductID || path == "" || path != asset.Path {
		return nil, errors.New("图片结果与任务不匹配")
	}
	asset.ID = newID()
	_, err = tx.Exec(`INSERT INTO product_assets(id,product_id,kind,path,original_name,mime_type,size_bytes,created_at) VALUES(?,?,?,?,?,?,?,?)`, asset.ID, asset.ProductID, asset.Kind, asset.Path, asset.OriginalName, asset.MimeType, asset.SizeBytes, nowUTC())
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`UPDATE image_provider_jobs SET asset_id=?,updated_at=? WHERE id=?`, asset.ID, nowUTC(), jobID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &asset, nil
}

func usagePriceTier(parameters map[string]any, snapshot string) map[string]any {
	var price ImagePrice
	if json.Unmarshal([]byte(snapshot), &price) == nil && price.Tier != "" {
		if parameters == nil {
			parameters = map[string]any{}
		}
		parameters["billing_tier"] = price.Tier
	}
	return parameters
}
