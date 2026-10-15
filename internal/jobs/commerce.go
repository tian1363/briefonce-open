package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tian1363/scriptagent/internal/userctx"
)

// Credit prices are initial product points, not a claim about provider currency.
// A charge is reserved before the provider request and refunded on failure.
var creditPrices = map[string]int64{
	"text": 1, "multimodal": 3, "embedding": 1,
	"image_generation": 20, "image_edit": 20,
	"video_generation":   20, // per requested second
	"speech_recognition": 5,  // per request of at most 30 seconds
}

type AccountCredit struct {
	UserID         string `json:"user_id"`
	Balance        int64  `json:"balance"`
	TrialEndsAt    string `json:"trial_ends_at"`
	ManagedEnabled bool   `json:"managed_enabled"`
	DailyLimit     int64  `json:"daily_limit"`
	Unlimited      bool   `json:"unlimited,omitempty"`
}

type CreditEntry struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Amount     int64  `json:"amount"`
	Balance    int64  `json:"balance_after"`
	Kind       string `json:"kind"`
	Capability string `json:"capability"`
	Note       string `json:"note"`
	CreatedAt  string `json:"created_at"`
}

func (s *Store) migrateCommerce() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS account_credits (
 user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 balance INTEGER NOT NULL DEFAULT 0 CHECK(balance>=0),
 trial_ends_at TEXT NOT NULL DEFAULT '',
 managed_enabled INTEGER NOT NULL DEFAULT 0,
 daily_limit INTEGER NOT NULL DEFAULT 1000 CHECK(daily_limit>=0)
);
CREATE TABLE IF NOT EXISTS credit_charges (
 id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 capability TEXT NOT NULL, points INTEGER NOT NULL CHECK(points>0),
 status TEXT NOT NULL CHECK(status IN ('reserved','consumed','refunded')),
 created_at TEXT NOT NULL, completed_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_credit_charges_daily ON credit_charges(user_id,created_at,status);
CREATE TABLE IF NOT EXISTS credit_ledger (
 id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 amount INTEGER NOT NULL, balance_after INTEGER NOT NULL, kind TEXT NOT NULL,
 capability TEXT NOT NULL DEFAULT '', note TEXT NOT NULL DEFAULT '',
 charge_id TEXT NOT NULL DEFAULT '', actor_id TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_credit_ledger_user ON credit_ledger(user_id,created_at DESC);
CREATE TABLE IF NOT EXISTS credit_prices (
 capability TEXT PRIMARY KEY, points INTEGER NOT NULL CHECK(points>0),
 updated_at TEXT NOT NULL, actor_id TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS platform_credentials (
 provider TEXT PRIMARY KEY, encrypted_key TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS platform_model_profiles (
 capability TEXT PRIMARY KEY, provider TEXT NOT NULL, model TEXT NOT NULL,
 endpoint TEXT NOT NULL, encrypted_key TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL
);
`)
	if err != nil {
		return err
	}
	for capability, points := range creditPrices {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO credit_prices(capability,points,updated_at) VALUES(?,?,?)`, capability, points, time.Now().UTC().Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return nil
}

// ManagedAPIKey returns the operator's credential. A key saved in the
// database takes precedence over the process environment, and is never sent
// to ordinary account APIs.
func (s *Store) ManagedAPIKey() (string, error) {
	var encrypted string
	err := s.db.QueryRow(`SELECT encrypted_key FROM platform_credentials WHERE provider='dashscope'`).Scan(&encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return strings.TrimSpace(os.Getenv("DASHSCOPE_API_KEY")), nil
	}
	if err != nil {
		return "", err
	}
	return s.decryptSecret(encrypted)
}

func (s *Store) ManagedAPIKeySource() (string, error) {
	var present int
	err := s.db.QueryRow(`SELECT 1 FROM platform_credentials WHERE provider='dashscope'`).Scan(&present)
	if err == nil {
		return "database", nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if strings.TrimSpace(os.Getenv("DASHSCOPE_API_KEY")) != "" {
		return "environment", nil
	}
	return "", nil
}

func (s *Store) SetManagedAPIKey(key string) error {
	key = strings.TrimSpace(key)
	if len(key) < 12 || len(key) > 4096 {
		return errors.New("平台 API Key 长度无效")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	encrypted, err := s.encryptSecret(key)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO platform_credentials(provider,encrypted_key,updated_at) VALUES('dashscope',?,?)
ON CONFLICT(provider) DO UPDATE SET encrypted_key=excluded.encrypted_key,updated_at=excluded.updated_at`, encrypted, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) GetAccountCredit(userID string) (AccountCredit, error) {
	x := AccountCredit{UserID: userID}
	var enabled int
	err := s.db.QueryRow(`SELECT balance,trial_ends_at,managed_enabled,daily_limit FROM account_credits WHERE user_id=?`, userID).
		Scan(&x.Balance, &x.TrialEndsAt, &enabled, &x.DailyLimit)
	x.ManagedEnabled = enabled != 0
	if userID == s.localUnlimitedAdminID {
		x.ManagedEnabled = true
		x.Unlimited = true
		if errors.Is(err, sql.ErrNoRows) {
			return x, nil
		}
	}
	return x, err
}

// SetLocalUnlimitedAdmin bypasses product point accounting for one bound
// administrator on a loopback-only test server. Provider requests still run.
func (s *Store) SetLocalUnlimitedAdmin(userID string) error {
	var role, status string
	if err := s.db.QueryRow(`SELECT role,status FROM users WHERE id=?`, userID).Scan(&role, &status); err != nil {
		return err
	}
	if role != "admin" || status != "active" {
		return errors.New("本地不限积分仅适用于启用中的管理员")
	}
	s.localUnlimitedAdminID = userID
	return nil
}

// DailyCreditUsage uses the same UTC day and charge states as ReserveUsage.
func (s *Store) DailyCreditUsage(userID string) (int64, error) {
	dayStart := time.Now().UTC().Format("2006-01-02") + "T00:00:00Z"
	var spent int64
	err := s.db.QueryRow(`SELECT COALESCE(SUM(points),0) FROM credit_charges WHERE user_id=? AND created_at>=? AND status IN ('reserved','consumed')`, userID, dayStart).Scan(&spent)
	return spent, err
}

func (s *Store) ListDailyCreditUsage() (map[string]int64, error) {
	dayStart := time.Now().UTC().Format("2006-01-02") + "T00:00:00Z"
	rows, err := s.db.Query(`SELECT user_id,COALESCE(SUM(points),0) FROM credit_charges WHERE created_at>=? AND status IN ('reserved','consumed') GROUP BY user_id`, dayStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	usage := map[string]int64{}
	for rows.Next() {
		var id string
		var used int64
		if err := rows.Scan(&id, &used); err != nil {
			return nil, err
		}
		usage[id] = used
	}
	return usage, rows.Err()
}

func (s *Store) ListAccountCredits() ([]AccountCredit, error) {
	rows, err := s.db.Query(`SELECT u.id,COALESCE(c.balance,0),COALESCE(c.trial_ends_at,''),COALESCE(c.managed_enabled,0),COALESCE(c.daily_limit,0) FROM users u LEFT JOIN account_credits c ON c.user_id=u.id ORDER BY u.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []AccountCredit{}
	for rows.Next() {
		var x AccountCredit
		var enabled int
		if err := rows.Scan(&x.UserID, &x.Balance, &x.TrialEndsAt, &enabled, &x.DailyLimit); err != nil {
			return nil, err
		}
		x.ManagedEnabled = enabled != 0
		if x.UserID == s.localUnlimitedAdminID {
			x.ManagedEnabled = true
			x.Unlimited = true
		}
		result = append(result, x)
	}
	return result, rows.Err()
}

func (s *Store) SetAccountCredit(userID, trialEndsAt string, enabled bool, dailyLimit int64) error {
	if dailyLimit < 0 {
		return errors.New("每日额度不能为负")
	}
	if trialEndsAt != "" {
		if _, err := time.Parse(time.RFC3339, trialEndsAt); err != nil {
			return errors.New("试用截止时间格式无效")
		}
	}
	_, err := s.db.Exec(`INSERT INTO account_credits(user_id,trial_ends_at,managed_enabled,daily_limit) VALUES(?,?,?,?)
ON CONFLICT(user_id) DO UPDATE SET trial_ends_at=excluded.trial_ends_at,managed_enabled=excluded.managed_enabled,daily_limit=excluded.daily_limit`, userID, trialEndsAt, enabled, dailyLimit)
	return err
}

func (s *Store) CreateManagedUser(input CreateUserInput, trialEndsAt string, initialCredits, dailyLimit int64, actorID string) (*User, error) {
	if err := ValidateManagedAccount(trialEndsAt, initialCredits, dailyLimit); err != nil {
		return nil, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	user := &User{ID: newID(), Email: strings.ToLower(strings.TrimSpace(input.Email)), Name: strings.TrimSpace(input.Name), Role: "member", Status: "active", PasswordHash: input.PasswordHash, CreatedAt: now, UpdatedAt: now}
	_, err = tx.Exec(`INSERT INTO users(id,email,name,role,status,password_hash,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, user.ID, user.Email, user.Name, user.Role, user.Status, user.PasswordHash, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(`INSERT INTO account_credits(user_id,balance,trial_ends_at,managed_enabled,daily_limit) VALUES(?,?,?,?,?)`, user.ID, initialCredits, trialEndsAt, 1, dailyLimit)
	if err != nil {
		return nil, err
	}
	if initialCredits > 0 {
		if _, err = tx.Exec(`INSERT INTO credit_lots VALUES(?,?,'grant','',?,?,0,?)`, newID(), user.ID, initialCredits, initialCredits, now.Format(time.RFC3339Nano)); err != nil {
			return nil, err
		}
		_, err = tx.Exec(`INSERT INTO credit_ledger(id,user_id,amount,balance_after,kind,note,actor_id,created_at) VALUES(?,?,?,?,?,?,?,?)`, newID(), user.ID, initialCredits, initialCredits, "grant", "初始积分", actorID, now.Format(time.RFC3339Nano))
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Store) UpdateUserPassword(userID, hash string) error {
	if hash == "" {
		return errors.New("密码不能为空")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE users SET password_hash=?,updated_at=? WHERE id=?`, hash, time.Now().UTC().Format(time.RFC3339), userID)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return errors.New("账号不存在")
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id=?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetUserStatus(userID, status string) error {
	if status != "active" && status != "suspended" {
		return errors.New("账号状态无效")
	}
	result, err := s.db.Exec(`UPDATE users SET status=?,updated_at=? WHERE id=? AND role='member'`, status, time.Now().UTC().Format(time.RFC3339), userID)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return errors.New("只能修改普通账号")
	}
	if status != "active" {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE user_id=?`, userID)
	}
	return nil
}

func (s *Store) AdjustCredits(userID, actorID string, amount int64, note string) (AccountCredit, error) {
	if amount == 0 || amount > 1000000 || amount < -1000000 {
		return AccountCredit{}, errors.New("单次积分调整需在 1 到 1000000 之间")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return AccountCredit{}, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE account_credits SET balance=balance+? WHERE user_id=? AND balance+?>=0`, amount, userID, amount)
	if err != nil {
		return AccountCredit{}, err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return AccountCredit{}, errors.New("账号不存在或积分不足")
	}
	var balance int64
	if err := tx.QueryRow(`SELECT balance FROM account_credits WHERE user_id=?`, userID).Scan(&balance); err != nil {
		return AccountCredit{}, err
	}
	if amount > 0 {
		if _, err = tx.Exec(`INSERT INTO credit_lots VALUES(?,?,'grant','',?,?,0,?)`, newID(), userID, amount, amount, nowUTC()); err != nil {
			return AccountCredit{}, err
		}
	} else {
		if err = allocateLots(tx, userID, "adjust-"+newID(), -amount); err != nil {
			return AccountCredit{}, err
		}
	}
	kind := "grant"
	if amount < 0 {
		kind = "adjustment"
	}
	_, err = tx.Exec(`INSERT INTO credit_ledger(id,user_id,amount,balance_after,kind,note,actor_id,created_at) VALUES(?,?,?,?,?,?,?,?)`, newID(), userID, amount, balance, kind, strings.TrimSpace(note), actorID, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return AccountCredit{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountCredit{}, err
	}
	return s.GetAccountCredit(userID)
}

func (s *Store) ListCreditLedger(userID string, limit int) ([]CreditEntry, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id,user_id,amount,balance_after,kind,capability,note,created_at FROM credit_ledger WHERE user_id=? ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CreditEntry{}
	for rows.Next() {
		var x CreditEntry
		if err := rows.Scan(&x.ID, &x.UserID, &x.Amount, &x.Balance, &x.Kind, &x.Capability, &x.Note, &x.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, x)
	}
	return items, rows.Err()
}

func CreditPrice(capability string, units int) (int64, error) {
	price, ok := creditPrices[capability]
	if !ok || units < 1 || units > 30 {
		return 0, errors.New("不支持该积分计费项目")
	}
	return price * int64(units), nil
}

func (s *Store) SetGlobalDailyCreditLimit(points int64) error {
	if points < 1 {
		return errors.New("全站每日积分上限至少为 1")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.globalDailyCreditLimit = points
	return nil
}

// ReserveUsage is safe for concurrent requests. The debit and ledger entry commit together.
func (s *Store) ReserveUsage(ctx context.Context, capability string, units int) (string, error) {
	userID := userctx.UserID(ctx)
	if userID == "" {
		return "", errors.New("无法确认付费账号")
	}
	_, err := CreditPrice(capability, units)
	if err != nil {
		return "", err
	}
	if userID == s.localUnlimitedAdminID {
		return "", nil
	}
	now := time.Now().UTC()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var unitPrice int64
	if err := tx.QueryRowContext(ctx, `SELECT points FROM credit_prices WHERE capability=?`, capability).Scan(&unitPrice); err != nil {
		return "", err
	}
	points := unitPrice * int64(units)
	pricing := ""
	if q, ok := ctx.Value(quoteContextKey{}).(quoteSnapshot); ok {
		points = q.Points * int64(units)
		b, _ := json.Marshal(q.Price)
		pricing = string(b)
	}
	var balance, dailyLimit int64
	var trialEnds string
	var enabled int
	var status string
	err = tx.QueryRowContext(ctx, `SELECT c.balance,c.trial_ends_at,c.managed_enabled,c.daily_limit,u.status FROM account_credits c JOIN users u ON u.id=c.user_id WHERE c.user_id=?`, userID).Scan(&balance, &trialEnds, &enabled, &dailyLimit, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("账号未开通平台积分")
	}
	if err != nil {
		return "", err
	}
	if enabled != 1 || status != "active" {
		return "", errors.New("账号未开通平台额度")
	}
	if trialEnds != "" {
		end, err := time.Parse(time.RFC3339, trialEnds)
		if err != nil || !now.Before(end) {
			return "", errors.New("试用期已结束")
		}
	}
	if balance < points {
		return "", errors.New("积分不足")
	}
	var spent int64
	dayStart := now.Format("2006-01-02") + "T00:00:00Z"
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(points),0) FROM credit_charges WHERE user_id=? AND created_at>=? AND status IN ('reserved','consumed')`, userID, dayStart).Scan(&spent)
	if err != nil {
		return "", err
	}
	if dailyLimit == 0 || spent+points > dailyLimit {
		return "", errors.New("今日使用额度已达上限")
	}
	var globalSpent int64
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(points),0) FROM credit_charges WHERE created_at>=? AND status IN ('reserved','consumed')`, dayStart).Scan(&globalSpent)
	if err != nil {
		return "", err
	}
	if globalSpent+points > s.globalDailyCreditLimit {
		return "", errors.New("平台今日额度已达上限")
	}
	chargeID := newID()
	result, err := tx.ExecContext(ctx, `UPDATE account_credits SET balance=balance-? WHERE user_id=? AND balance>=?`, points, userID, points)
	if err != nil {
		return "", err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return "", errors.New("积分不足")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO credit_charges(id,user_id,capability,points,status,created_at) VALUES(?,?,?,?,?,?)`, chargeID, userID, capability, points, "reserved", now.Format(time.RFC3339Nano))
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO credit_ledger(id,user_id,amount,balance_after,kind,capability,charge_id,created_at) VALUES(?,?,?,?,?,?,?,?)`, newID(), userID, -points, balance-points, "reserve", capability, chargeID, now.Format(time.RFC3339Nano))
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(`UPDATE credit_charges SET pricing_json=? WHERE id=?`, pricing, chargeID); err != nil {
		return "", err
	}
	if err = allocateLots(tx, userID, chargeID, points); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return chargeID, nil
}

func (s *Store) FinishUsage(chargeID string, success bool) error {
	if chargeID == "" {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID, capability, status string
	var points int64
	if err := tx.QueryRow(`SELECT user_id,capability,points,status FROM credit_charges WHERE id=?`, chargeID).Scan(&userID, &capability, &points, &status); err != nil {
		return err
	}
	if status != "reserved" {
		return nil
	}
	if !success {
		var pending int
		if err = tx.QueryRow(`SELECT COUNT(*) FROM image_provider_jobs WHERE charge_id=? AND status NOT IN ('failed','completed')`, chargeID).Scan(&pending); err != nil {
			return err
		}
		if pending > 0 {
			return nil
		}
	}
	newStatus := "consumed"
	if !success {
		newStatus = "refunded"
	}
	_, err = tx.Exec(`UPDATE credit_charges SET status=?,completed_at=? WHERE id=? AND status='reserved'`, newStatus, time.Now().UTC().Format(time.RFC3339Nano), chargeID)
	if err != nil {
		return err
	}
	if !success {
		if err = restoreLots(tx, chargeID); err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE account_credits SET balance=balance+? WHERE user_id=?`, points, userID); err != nil {
			return err
		}
		var balance int64
		if err = tx.QueryRow(`SELECT balance FROM account_credits WHERE user_id=?`, userID).Scan(&balance); err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO credit_ledger(id,user_id,amount,balance_after,kind,capability,charge_id,created_at) VALUES(?,?,?,?,?,?,?,?)`, newID(), userID, points, balance, "refund", capability, chargeID, time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ReconcileStaleCharges settles interrupted calls conservatively. A provider
// may already have charged an interrupted request, so ambiguous usage is kept.
// Active video jobs retain their reservation until their provider status is known.
func (s *Store) ReconcileStaleCharges(olderThan time.Duration) error {
	cutoff := time.Now().UTC().Add(-olderThan).Format(time.RFC3339Nano)
	rows, err := s.db.Query(`SELECT c.id FROM credit_charges c WHERE c.status='reserved' AND c.created_at<?
AND NOT EXISTS (SELECT 1 FROM image_provider_jobs i WHERE i.charge_id=c.id AND i.status NOT IN ('completed','failed'))
AND NOT EXISTS (SELECT 1 FROM video_generations v WHERE v.credit_charge_id=c.id AND v.status IN ('pending','submitting','submitted','running'))`, cutoff)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.FinishUsage(id, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ManagedEnabled(userID string) bool {
	if userID == s.localUnlimitedAdminID {
		return true
	}
	var enabled int
	if err := s.db.QueryRow(`SELECT managed_enabled FROM account_credits WHERE user_id=?`, userID).Scan(&enabled); err != nil {
		return false
	}
	return enabled == 1
}

func (s *Store) CreditPrices() map[string]int64 {
	result := make(map[string]int64, len(creditPrices))
	rows, err := s.db.Query(`SELECT capability,points FROM credit_prices`)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var capability string
		var points int64
		if rows.Scan(&capability, &points) == nil {
			result[capability] = points
		}
	}
	return result
}

func (s *Store) SetCreditPrice(capability string, points int64, actorID string) error {
	if _, ok := creditPrices[capability]; !ok {
		return errors.New("不支持该积分项目")
	}
	if points < 1 || points > 100000 {
		return errors.New("单价需在 1 到 100000 积分之间")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`UPDATE credit_prices SET points=?,updated_at=?,actor_id=? WHERE capability=?`, points, time.Now().UTC().Format(time.RFC3339), actorID, capability)
	return err
}

func ValidateManagedAccount(trialEnds string, points, dailyLimit int64) error {
	if points < 0 || points > 1000000 {
		return fmt.Errorf("初始积分需在 0 到 1000000 之间")
	}
	if dailyLimit < 1 || dailyLimit > 1000000 {
		return fmt.Errorf("每日额度需在 1 到 1000000 之间")
	}
	if trialEnds != "" {
		if _, err := time.Parse(time.RFC3339, trialEnds); err != nil {
			return errors.New("试用截止时间格式无效")
		}
	}
	return nil
}
