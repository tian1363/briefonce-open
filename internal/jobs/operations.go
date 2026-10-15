package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tian1363/scriptagent/internal/charactergen"
	"github.com/tian1363/scriptagent/internal/userctx"
	"strings"
	"time"
)

// Currency values are integer millionths of CNY; multipliers are basis points.
type ImagePrice struct {
	Model         string `json:"model"`
	Tier          string `json:"tier"`
	Enabled       bool   `json:"enabled"`
	CostMicros    int64  `json:"cost_micros"`
	MultiplierBPS int64  `json:"multiplier_bps"`
	PointMicros   int64  `json:"point_micros"`
	FixedPoints   int64  `json:"fixed_points"`
	Version       int64  `json:"version"`
	Actor         string `json:"actor"`
	UpdatedAt     string `json:"updated_at"`
}

func (p ImagePrice) Points() (int64, error) {
	if !p.Enabled || p.CostMicros <= 0 || p.CostMicros > 1000000000 || p.MultiplierBPS < 1000 || p.MultiplierBPS > 1000000 || p.PointMicros < 1 || p.PointMicros > 1000000 || p.FixedPoints < 0 || p.FixedPoints > 1000000 {
		return 0, errors.New("模型未启用或成本及定价规则不完整")
	}
	if p.FixedPoints > 0 {
		return p.FixedPoints, nil
	}
	d := 10000 * p.PointMicros
	n := (p.CostMicros*p.MultiplierBPS + d - 1) / d
	if n < 1 {
		n = 1
	}
	if n > 1000000 {
		return 0, errors.New("单次积分报价过高")
	}
	return n, nil
}
func nowUTC() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func (s *Store) migrateOperations() error {
	if err := s.migrateVideoPrices(); err != nil {
		return err
	}
	_, err := s.db.Exec(`
 CREATE TABLE IF NOT EXISTS image_prices(model TEXT NOT NULL,tier TEXT NOT NULL,enabled INTEGER NOT NULL DEFAULT 0,cost_micros INTEGER NOT NULL DEFAULT 0,multiplier_bps INTEGER NOT NULL DEFAULT 0,point_micros INTEGER NOT NULL DEFAULT 0,fixed_points INTEGER NOT NULL DEFAULT 0,version INTEGER NOT NULL DEFAULT 1,actor TEXT NOT NULL DEFAULT '',updated_at TEXT NOT NULL,PRIMARY KEY(model,tier));
 CREATE TABLE IF NOT EXISTS operations_audit(id TEXT PRIMARY KEY,actor TEXT NOT NULL,kind TEXT NOT NULL,ref TEXT NOT NULL,before_json TEXT NOT NULL,after_json TEXT NOT NULL,note TEXT NOT NULL,created_at TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS credit_packages(id TEXT PRIMARY KEY,name TEXT NOT NULL,amount_micros INTEGER NOT NULL,paid_points INTEGER NOT NULL,bonus_points INTEGER NOT NULL,fee_bps INTEGER NOT NULL DEFAULT 0,active INTEGER NOT NULL DEFAULT 0,version INTEGER NOT NULL DEFAULT 1,updated_at TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS purchase_orders(id TEXT PRIMARY KEY,user_id TEXT NOT NULL REFERENCES users(id),package_id TEXT NOT NULL,snapshot_json TEXT NOT NULL,amount_micros INTEGER NOT NULL,paid_points INTEGER NOT NULL,bonus_points INTEGER NOT NULL,fee_micros INTEGER NOT NULL DEFAULT 0,status TEXT NOT NULL,receipt TEXT NOT NULL DEFAULT '',actor TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL,completed_at TEXT NOT NULL DEFAULT '', UNIQUE(receipt));
 CREATE TABLE IF NOT EXISTS credit_lots(id TEXT PRIMARY KEY,user_id TEXT NOT NULL,source TEXT NOT NULL,order_id TEXT NOT NULL DEFAULT '',total INTEGER NOT NULL,remaining INTEGER NOT NULL,revenue_micros INTEGER,created_at TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS credit_allocations(charge_id TEXT NOT NULL,lot_id TEXT NOT NULL,points INTEGER NOT NULL,PRIMARY KEY(charge_id,lot_id));
 CREATE TABLE IF NOT EXISTS image_provider_jobs(id TEXT PRIMARY KEY,user_id TEXT NOT NULL,kind TEXT NOT NULL,product_id TEXT NOT NULL DEFAULT '',charge_id TEXT NOT NULL DEFAULT '',capability TEXT NOT NULL,provider TEXT NOT NULL,model TEXT NOT NULL,endpoint TEXT NOT NULL,key_ref TEXT NOT NULL,credential TEXT NOT NULL,request_json TEXT NOT NULL,price_json TEXT NOT NULL,estimated_micros INTEGER NOT NULL,actual_micros INTEGER,refund_micros INTEGER NOT NULL DEFAULT 0,evidence TEXT NOT NULL DEFAULT '',reconcile_status TEXT NOT NULL DEFAULT 'estimated',upstream_id TEXT NOT NULL DEFAULT '',status TEXT NOT NULL DEFAULT 'pending',progress INTEGER NOT NULL DEFAULT 0,result_url TEXT NOT NULL DEFAULT '',saved_path TEXT NOT NULL DEFAULT '',asset_id TEXT NOT NULL DEFAULT '',error TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS idx_image_provider_pending ON image_provider_jobs(status);
 CREATE TABLE IF NOT EXISTS provider_requests(id TEXT PRIMARY KEY,job_id TEXT NOT NULL,kind TEXT NOT NULL,status_code INTEGER NOT NULL,upstream_request_id TEXT NOT NULL DEFAULT '',error TEXT NOT NULL,created_at TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS supplier_statements(id TEXT PRIMARY KEY,account TEXT NOT NULL,external_id TEXT NOT NULL,task_id TEXT NOT NULL,amount_micros INTEGER NOT NULL,refund_micros INTEGER NOT NULL DEFAULT 0,evidence TEXT NOT NULL,match_id TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL, UNIQUE(account,external_id));
 CREATE TABLE IF NOT EXISTS supplier_deposits(id TEXT PRIMARY KEY,account TEXT NOT NULL,amount_micros INTEGER NOT NULL,bonus_micros INTEGER NOT NULL,receipt TEXT NOT NULL,actor TEXT NOT NULL,created_at TEXT NOT NULL);
 `)
	if err != nil {
		return err
	}
	for _, m := range charactergen.ImageModels {
		tiers := []string{m.Tier}
		if m.Tier == "variable" {
			tiers = m.Tiers
		}
		for _, tier := range tiers {
			if _, err = s.db.Exec(`INSERT OR IGNORE INTO image_prices(model,tier,updated_at) VALUES(?,?,?)`, m.ID, tier, nowUTC()); err != nil {
				return err
			}
		}
	}
	if err = s.ensureColumn("platform_model_profiles", "account_ref", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err = s.ensureColumn("image_provider_jobs", "account_ref", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err = s.ensureColumn("purchase_orders", "refund_micros", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err = s.ensureColumn("image_provider_jobs", "source", "TEXT NOT NULL DEFAULT 'managed'"); err != nil {
		return err
	}
	for _, col := range []string{"pricing_json", "provider_job_id"} {
		if err = s.ensureColumn("credit_charges", col, "TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
	}
	_, err = s.db.Exec(`INSERT OR IGNORE INTO credit_lots(id,user_id,source,total,remaining,revenue_micros,created_at) SELECT 'legacy-'||user_id,user_id,'legacy',balance,balance,NULL,? FROM account_credits c WHERE balance>0 AND NOT EXISTS (SELECT 1 FROM credit_lots l WHERE l.user_id=c.user_id)`, nowUTC())
	if err != nil {
		return err
	}
	return s.migrateVideoProfiles()
}
func auditTx(tx *sql.Tx, actor, kind, ref string, before, after any, note string) error {
	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)
	_, err := tx.Exec(`INSERT INTO operations_audit VALUES(?,?,?,?,?,?,?,?)`, newID(), actor, kind, ref, string(b), string(a), note, nowUTC())
	return err
}
func (s *Store) ImagePrices() ([]ImagePrice, error) {
	rows, err := s.db.Query(`SELECT model,tier,enabled,cost_micros,multiplier_bps,point_micros,fixed_points,version,actor,updated_at FROM image_prices ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ImagePrice{}
	for rows.Next() {
		var p ImagePrice
		if err = rows.Scan(&p.Model, &p.Tier, &p.Enabled, &p.CostMicros, &p.MultiplierBPS, &p.PointMicros, &p.FixedPoints, &p.Version, &p.Actor, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) ImageQuote(o charactergen.Options) (ImagePrice, int64, error) {
	if err := o.Validate(); err != nil {
		return ImagePrice{}, 0, err
	}
	var p ImagePrice
	err := s.db.QueryRow(`SELECT model,tier,enabled,cost_micros,multiplier_bps,point_micros,fixed_points,version,actor,updated_at FROM image_prices WHERE model=? AND tier=?`, o.Model, o.Tier()).Scan(&p.Model, &p.Tier, &p.Enabled, &p.CostMicros, &p.MultiplierBPS, &p.PointMicros, &p.FixedPoints, &p.Version, &p.Actor, &p.UpdatedAt)
	if err != nil {
		return p, 0, errors.New("此模型尚未定价")
	}
	if o.PriceVersion != 0 && o.PriceVersion != p.Version {
		return p, 0, errors.New("模型报价已更新，请刷新模型参数后重新提交")
	}
	points, err := p.Points()
	return p, points, err
}
func (s *Store) SaveImagePrice(p ImagePrice, actor, note string) error {
	m, ok := charactergen.FindModel(p.Model)
	if !ok {
		return errors.New("未知图片模型")
	}
	ok = m.Tier == p.Tier
	for _, t := range m.Tiers {
		ok = ok || t == p.Tier
	}
	if !ok {
		return errors.New("模型档位无效")
	}
	check := p
	check.Enabled = true
	if _, err := check.Points(); err != nil {
		return err
	}
	if strings.TrimSpace(note) == "" {
		return errors.New("请填写调价原因")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old string
	_ = tx.QueryRow(`SELECT json_object('cost_micros',cost_micros,'multiplier_bps',multiplier_bps,'point_micros',point_micros,'fixed_points',fixed_points,'enabled',enabled,'version',version) FROM image_prices WHERE model=? AND tier=?`, p.Model, p.Tier).Scan(&old)
	_, err = tx.Exec(`UPDATE image_prices SET enabled=?,cost_micros=?,multiplier_bps=?,point_micros=?,fixed_points=?,version=version+1,actor=?,updated_at=? WHERE model=? AND tier=?`, p.Enabled, p.CostMicros, p.MultiplierBPS, p.PointMicros, p.FixedPoints, actor, nowUTC(), p.Model, p.Tier)
	if err != nil {
		return err
	}
	if err = auditTx(tx, actor, "image_price", p.Model+":"+p.Tier, old, p, note); err != nil {
		return err
	}
	return tx.Commit()
}

type Package struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	AmountMicros int64  `json:"amount_micros"`
	PaidPoints   int64  `json:"paid_points"`
	BonusPoints  int64  `json:"bonus_points"`
	FeeBPS       int64  `json:"fee_bps"`
	Active       bool   `json:"active"`
	Version      int64  `json:"version"`
}

func (s *Store) Packages(activeOnly bool) ([]Package, error) {
	query := `SELECT id,name,amount_micros,paid_points,bonus_points,fee_bps,active,version FROM credit_packages`
	if activeOnly {
		query += ` WHERE active=1`
	}
	rows, err := s.db.Query(query + ` ORDER BY amount_micros`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Package{}
	for rows.Next() {
		var p Package
		if err = rows.Scan(&p.ID, &p.Name, &p.AmountMicros, &p.PaidPoints, &p.BonusPoints, &p.FeeBPS, &p.Active, &p.Version); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) SavePackage(p Package, actor, note string) error {
	if strings.TrimSpace(p.Name) == "" || len(p.Name) > 120 || p.AmountMicros < 10000 || p.AmountMicros > 100000000000 || p.PaidPoints < 1 || p.PaidPoints > 10000000 || p.BonusPoints < 0 || p.BonusPoints > 10000000 || p.FeeBPS < 0 || p.FeeBPS >= 10000 || strings.TrimSpace(note) == "" {
		return errors.New("套餐金额、积分、费用或修改原因无效")
	}
	if p.ID == "" {
		p.ID = newID()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old string
	_ = tx.QueryRow(`SELECT name FROM credit_packages WHERE id=?`, p.ID).Scan(&old)
	_, err = tx.Exec(`INSERT INTO credit_packages(id,name,amount_micros,paid_points,bonus_points,fee_bps,active,updated_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,amount_micros=excluded.amount_micros,paid_points=excluded.paid_points,bonus_points=excluded.bonus_points,fee_bps=excluded.fee_bps,active=excluded.active,version=credit_packages.version+1,updated_at=excluded.updated_at`, p.ID, p.Name, p.AmountMicros, p.PaidPoints, p.BonusPoints, p.FeeBPS, p.Active, nowUTC())
	if err != nil {
		return err
	}
	if err = auditTx(tx, actor, "package", p.ID, old, p, note); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) CreatePurchase(userID, packageID string) (string, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var p Package
	p.ID = packageID
	err = tx.QueryRow(`SELECT name,amount_micros,paid_points,bonus_points,fee_bps,version FROM credit_packages WHERE id=? AND active=1`, packageID).Scan(&p.Name, &p.AmountMicros, &p.PaidPoints, &p.BonusPoints, &p.FeeBPS, &p.Version)
	if err != nil {
		return "", errors.New("套餐已下架")
	}
	var enabled int
	if err = tx.QueryRow(`SELECT managed_enabled FROM account_credits WHERE user_id=?`, userID).Scan(&enabled); err != nil || enabled != 1 {
		return "", errors.New("此账号未开通平台积分")
	}
	id := newID()
	b, _ := json.Marshal(p)
	_, err = tx.Exec(`INSERT INTO purchase_orders(id,user_id,package_id,snapshot_json,amount_micros,paid_points,bonus_points,fee_micros,status,receipt,created_at) VALUES(?,?,?,?,?,?,?,?,'pending',?,?)`, id, userID, packageID, string(b), p.AmountMicros, p.PaidPoints, p.BonusPoints, p.AmountMicros*p.FeeBPS/10000, "pending-"+id, nowUTC())
	if err != nil {
		return "", err
	}
	return id, tx.Commit()
}
func (s *Store) ConfirmPurchase(id, receipt, actor string) error {
	if strings.TrimSpace(receipt) == "" || len(receipt) > 300 {
		return errors.New("请填写唯一收款凭证或交易号")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var user, status string
	var amount, paid, bonus, fee int64
	err = tx.QueryRow(`SELECT user_id,status,amount_micros,paid_points,bonus_points,fee_micros FROM purchase_orders WHERE id=?`, id).Scan(&user, &status, &amount, &paid, &bonus, &fee)
	if err != nil {
		return err
	}
	if status == "credited" {
		return nil
	}
	if status != "pending" {
		return errors.New("订单不可确认")
	}
	_, err = tx.Exec(`UPDATE purchase_orders SET status='credited',receipt=?,actor=?,completed_at=? WHERE id=?`, receipt, actor, nowUTC(), id)
	if err != nil {
		return errors.New("收款凭证已用于其他订单")
	}
	res, err := tx.Exec(`UPDATE account_credits SET balance=balance+? WHERE user_id=?`, paid+bonus, user)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return errors.New("账号积分账户不存在")
	}
	var balance int64
	if err = tx.QueryRow(`SELECT balance FROM account_credits WHERE user_id=?`, user).Scan(&balance); err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO credit_ledger(id,user_id,amount,balance_after,kind,note,actor_id,created_at) VALUES(?,?,?,?,?,?,?,?)`, newID(), user, paid+bonus, balance, "purchase", "购买积分订单 "+id, actor, nowUTC())
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO credit_lots VALUES(?,?,'paid',?,?,?,?,?)`, newID(), user, id, paid, paid, amount-fee, nowUTC())
	if err != nil {
		return err
	}
	if bonus > 0 {
		_, err = tx.Exec(`INSERT INTO credit_lots VALUES(?,?,'bonus',?,?,?,?,?)`, newID(), user, id, bonus, bonus, 0, nowUTC())
		if err != nil {
			return err
		}
	}
	if err = auditTx(tx, actor, "purchase_confirm", id, "pending", "credited", receipt); err != nil {
		return err
	}
	return tx.Commit()
}

// Refund the unused paid portion and retire its remaining bonus points atomically.
func (s *Store) RefundPurchase(id, actor, receipt string) error {
	if strings.TrimSpace(receipt) == "" {
		return errors.New("请填写退款凭证")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var user, status string
	var paid, amount int64
	if err = tx.QueryRow(`SELECT user_id,status,paid_points,amount_micros FROM purchase_orders WHERE id=?`, id).Scan(&user, &status, &paid, &amount); err != nil {
		return err
	}
	if status == "refunded" || status == "partially_refunded" {
		return nil
	}
	if status != "credited" {
		return errors.New("订单不可退款")
	}
	// Reserved allocations may later be returned to a lot; settle them before refund.
	var pending int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM credit_allocations a JOIN credit_lots l ON l.id=a.lot_id JOIN credit_charges c ON c.id=a.charge_id WHERE l.order_id=? AND c.status='reserved'`, id).Scan(&pending); err != nil {
		return err
	}
	if pending > 0 {
		return errors.New("订单积分有任务待结算，请先完成或核对任务")
	}
	var paidRemaining, bonusRemaining int64
	if err = tx.QueryRow(`SELECT COALESCE(SUM(CASE WHEN source='paid' THEN remaining ELSE 0 END),0),COALESCE(SUM(CASE WHEN source='bonus' THEN remaining ELSE 0 END),0) FROM credit_lots WHERE order_id=?`, id).Scan(&paidRemaining, &bonusRemaining); err != nil {
		return err
	}
	if paidRemaining == 0 {
		return errors.New("没有未使用的付费积分可退款")
	}
	points := paidRemaining + bonusRemaining
	refund := amount * paidRemaining / paid
	res, err := tx.Exec(`UPDATE account_credits SET balance=balance-? WHERE user_id=? AND balance>=?`, points, user, points)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return errors.New("余额不足，无法退款")
	}
	if _, err = tx.Exec(`UPDATE credit_lots SET remaining=0 WHERE order_id=?`, id); err != nil {
		return err
	}
	next := "refunded"
	if paidRemaining < paid {
		next = "partially_refunded"
	}
	if _, err = tx.Exec(`UPDATE purchase_orders SET status=?,refund_micros=?,actor=? WHERE id=?`, next, refund, actor, id); err != nil {
		return err
	}
	var balance int64
	if err = tx.QueryRow(`SELECT balance FROM account_credits WHERE user_id=?`, user).Scan(&balance); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO credit_ledger(id,user_id,amount,balance_after,kind,note,actor_id,created_at) VALUES(?,?,?,?,?,?,?,?)`, newID(), user, -points, balance, "purchase_refund", "退款订单 "+id, actor, nowUTC()); err != nil {
		return err
	}
	if err = auditTx(tx, actor, "purchase_refund", id, "credited", map[string]any{"status": next, "refund_micros": refund, "removed_points": points}, receipt); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Orders(userID string) ([]map[string]any, error) {
	q := `SELECT o.id,o.user_id,u.email,o.snapshot_json,o.amount_micros,o.paid_points,o.bonus_points,o.fee_micros,o.status,o.receipt,o.created_at,o.completed_at,o.refund_micros,(SELECT COALESCE(SUM(remaining),0) FROM credit_lots l WHERE l.order_id=o.id AND l.source='paid') AS paid_remaining,(SELECT COALESCE(SUM(remaining),0) FROM credit_lots l WHERE l.order_id=o.id AND l.source='bonus') AS bonus_remaining FROM purchase_orders o JOIN users u ON u.id=o.user_id`
	args := []any{}
	if userID != "" {
		q += ` WHERE o.user_id=?`
		args = append(args, userID)
	}
	return s.queryMaps(q+` ORDER BY o.created_at DESC LIMIT 200`, args...)
}
func (s *Store) queryMaps(q string, args ...any) ([]map[string]any, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		values := make([]any, len(cols))
		ptr := make([]any, len(cols))
		for i := range ptr {
			ptr[i] = &values[i]
		}
		if err = rows.Scan(ptr...); err != nil {
			return nil, err
		}
		item := map[string]any{}
		for i, c := range cols {
			if b, ok := values[i].([]byte); ok {
				item[c] = string(b)
			} else {
				item[c] = values[i]
			}
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

type quoteContextKey struct{}
type quoteSnapshot struct {
	Points int64
	Price  ImagePrice
}

func (s *Store) ReserveImageUsage(ctx context.Context, capability string, o charactergen.Options) (string, error) {
	p, points, err := s.ImageQuote(o)
	if err != nil {
		return "", err
	}
	return s.ReserveUsage(context.WithValue(ctx, quoteContextKey{}, quoteSnapshot{Points: points, Price: p}), capability, 1)
}
func allocateLots(tx *sql.Tx, userID, chargeID string, points int64) error {
	rows, err := tx.Query(`SELECT id,remaining FROM credit_lots WHERE user_id=? AND remaining>0 ORDER BY created_at,id`, userID)
	if err != nil {
		return err
	}
	type lot struct {
		id string
		n  int64
	}
	lots := []lot{}
	for rows.Next() {
		var l lot
		if err = rows.Scan(&l.id, &l.n); err != nil {
			rows.Close()
			return err
		}
		lots = append(lots, l)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	left := points
	for _, l := range lots {
		n := l.n
		if n > left {
			n = left
		}
		if n == 0 {
			break
		}
		if _, err = tx.Exec(`UPDATE credit_lots SET remaining=remaining-? WHERE id=?`, n, l.id); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO credit_allocations VALUES(?,?,?)`, chargeID, l.id, n); err != nil {
			return err
		}
		left -= n
	}
	if left > 0 { // Backwards-compatible grants without source attribution are explicit legacy lots.
		id := newID()
		_, err = tx.Exec(`INSERT INTO credit_lots VALUES(?,?,'legacy','',?,0,NULL,?)`, id, userID, left, nowUTC())
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO credit_allocations VALUES(?,?,?)`, chargeID, id, left)
	}
	return err
}
func restoreLots(tx *sql.Tx, chargeID string) error {
	_, err := tx.Exec(`UPDATE credit_lots SET remaining=remaining+(SELECT points FROM credit_allocations WHERE lot_id=credit_lots.id AND charge_id=?) WHERE id IN (SELECT lot_id FROM credit_allocations WHERE charge_id=?)`, chargeID, chargeID)
	return err
}
func (s *Store) OperationsAudit() ([]map[string]any, error) {
	return s.queryMaps(`SELECT * FROM operations_audit ORDER BY created_at DESC LIMIT 200`)
}
func (s *Store) RevenueSummary() (map[string]any, error) {
	out := map[string]any{}
	for k, q := range map[string]string{"purchase_cash_micros": `SELECT COALESCE(SUM(amount_micros-refund_micros),0) FROM purchase_orders WHERE status IN ('credited','partially_refunded','refunded')`, "confirmed_cost_micros": `SELECT COALESCE(SUM(actual_micros-refund_micros),0) FROM image_provider_jobs WHERE source='managed' AND actual_micros IS NOT NULL AND reconcile_status='reconciled'`, "unreconciled_jobs": `SELECT COUNT(*) FROM image_provider_jobs WHERE source='managed' AND reconcile_status<>'reconciled'`, "estimated_cost_micros": `SELECT COALESCE(SUM(estimated_micros),0) FROM image_provider_jobs WHERE source='managed' AND reconcile_status<>'reconciled'`, "usage_revenue_micros": `SELECT COALESCE(SUM((a.points*l.revenue_micros)/l.total),0) FROM credit_allocations a JOIN credit_lots l ON l.id=a.lot_id JOIN credit_charges c ON c.id=a.charge_id WHERE c.status='consumed' AND l.revenue_micros IS NOT NULL`, "unknown_revenue_points": `SELECT COALESCE(SUM(a.points),0) FROM credit_allocations a JOIN credit_lots l ON l.id=a.lot_id JOIN credit_charges c ON c.id=a.charge_id WHERE c.status='consumed' AND l.revenue_micros IS NULL`} {
		var n int64
		if err := s.db.QueryRow(q).Scan(&n); err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		out[k] = n
	}
	return out, nil
}

// Actor identity always comes from the authenticated context, never request JSON.
func Actor(ctx context.Context) string { return userctx.UserID(ctx) }
