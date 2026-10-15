package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/tian1363/scriptagent/internal/userctx"
	"time"
)

type UsageTask struct {
	UserID     string
	TaskID     string
	ChargeID   string
	Title      string
	ProductID  string
	Model      string
	Source     string
	Capability string
}

type UsageActivity struct {
	Parameters      map[string]any `json:"parameters,omitempty"`
	Provider        string         `json:"provider,omitempty"`
	requestSnapshot string
	ID              string `json:"id"`
	Capability      string `json:"capability"`
	Points          int64  `json:"points"`
	BillingStatus   string `json:"billing_status"`
	CreatedAt       string `json:"created_at"`
	CompletedAt     string `json:"completed_at"`
	TaskID          string `json:"task_id"`
	Title           string `json:"title"`
	ProductTitle    string `json:"product_title"`
	Model           string `json:"model"`
	Source          string `json:"source"`
	TaskStatus      string `json:"task_status"`
	Error           string `json:"error"`
	AssetID         string `json:"asset_id"`
	CharacterID     string `json:"character_id"`
}

func (s *Store) migrateUsageTasks() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS usage_tasks (
 task_id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 charge_id TEXT NOT NULL DEFAULT '', title TEXT NOT NULL, product_id TEXT NOT NULL DEFAULT '',
 model TEXT NOT NULL, source TEXT NOT NULL, capability TEXT NOT NULL, created_at TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS idx_usage_tasks_user ON usage_tasks(user_id,created_at DESC);
 CREATE UNIQUE INDEX IF NOT EXISTS idx_usage_tasks_charge ON usage_tasks(charge_id) WHERE charge_id<>'';`)
	if err != nil {
		return err
	}
	for _, column := range []string{"task_ref", "task_label", "model_name"} {
		if err := s.ensureColumn("credit_charges", column, "TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DescribeUsage(ctx context.Context, chargeID, modelName, ref, label string) error {
	if chargeID == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE credit_charges SET model_name=?,task_ref=?,task_label=? WHERE id=? AND user_id=?`, modelName, ref, label, chargeID, userctx.UserID(ctx))
	return err
}

// Save the model and billing source at submission time; future price changes cannot alter history.
func (s *Store) RecordUsageTasks(tasks []UsageTask) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, task := range tasks {
		if task.UserID == "" || task.TaskID == "" {
			return errors.New("任务计费信息不完整")
		}
		if task.ChargeID != "" {
			var owner string
			if err := tx.QueryRow(`SELECT user_id FROM credit_charges WHERE id=?`, task.ChargeID).Scan(&owner); err != nil {
				return err
			}
			if owner != task.UserID {
				return errors.New("计费任务不属于当前账号")
			}
		} else if task.Source == "managed" {
			if task.UserID != s.localUnlimitedAdminID {
				return errors.New("平台任务缺少积分记录")
			}
			task.Source = "waived"
		}
		_, err = tx.Exec(`INSERT INTO usage_tasks(task_id,user_id,charge_id,title,product_id,model,source,capability,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, task.TaskID, task.UserID, task.ChargeID, task.Title, task.ProductID, task.Model, task.Source, task.Capability, time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

const activityQuery = `WITH events AS (
 SELECT c.id,c.user_id,c.capability,c.points,c.status billing_status,c.created_at,c.completed_at,
 COALESCE(t.task_id,c.task_ref) task_id,COALESCE(t.title,c.task_label) title,COALESCE(t.product_id,'') product_id,COALESCE(t.model,c.model_name) model,COALESCE(t.source,'managed') source
 FROM credit_charges c LEFT JOIN usage_tasks t ON t.charge_id=c.id AND t.user_id=c.user_id WHERE c.user_id=?
 UNION ALL
 SELECT t.task_id,t.user_id,t.capability,0,'unbilled',t.created_at,'',t.task_id,t.title,t.product_id,t.model,t.source FROM usage_tasks t WHERE t.user_id=? AND t.charge_id=''
) SELECT e.id,e.capability,e.points,e.billing_status,e.created_at,e.completed_at,
 COALESCE(NULLIF(e.task_id,''),v.id,''),COALESCE(NULLIF(e.title,''),CASE WHEN v.id IS NOT NULL THEN '视频生成' ELSE '模型调用' END),
 COALESCE(p.title,vp.title,''),COALESCE(ip.model,NULLIF(e.model,''),v.model,''),e.source,
 COALESCE(d.status,i.status,ch.status,v.status,''),COALESCE(d.error_message,i.error_message,ch.error_message,v.error_message,''),COALESCE(d.output_asset_id,i.output_asset_id,''),COALESCE(ch.id,''),COALESCE(ip.provider,''),COALESCE(ip.request_json,''),CASE WHEN v.id IS NOT NULL THEN json_object('resolution',v.resolution,'aspect_ratio',v.ratio,'duration',v.duration,'sound_enabled',v.sound_enabled,'prompt',v.prompt,'negative_prompt',v.negative_prompt) ELSE COALESCE(ch.controls_json,'{}') END,COALESCE(ip.price_json,'')
 FROM events e
 LEFT JOIN image_transform_jobs i ON i.id=e.task_id AND i.user_id=e.user_id
 LEFT JOIN detail_image_jobs d ON d.id=e.task_id AND d.user_id=e.user_id
 LEFT JOIN characters ch ON ch.id=e.task_id AND ch.user_id=e.user_id
 LEFT JOIN video_generations v ON v.credit_charge_id=e.id AND v.user_id=e.user_id
 LEFT JOIN image_provider_jobs ip ON (ip.id=e.task_id OR (ip.charge_id=e.id AND ip.charge_id<>'')) AND ip.user_id=e.user_id
 LEFT JOIN products p ON p.id=e.product_id
 LEFT JOIN products vp ON vp.id=v.product_id`

func (s *Store) scanUsage(rows *sql.Rows) ([]UsageActivity, error) {
	items := []UsageActivity{}
	for rows.Next() {
		var x UsageActivity
		var controls, price string
		if err := rows.Scan(&x.ID, &x.Capability, &x.Points, &x.BillingStatus, &x.CreatedAt, &x.CompletedAt, &x.TaskID, &x.Title, &x.ProductTitle, &x.Model, &x.Source, &x.TaskStatus, &x.Error, &x.AssetID, &x.CharacterID, &x.Provider, &x.requestSnapshot, &controls, &price); err != nil {
			return nil, err
		}
		if x.requestSnapshot != "" {
			x.Parameters = s.usageParameters(x.requestSnapshot)
		} else {
			var c map[string]any
			if json.Unmarshal([]byte(controls), &c) == nil {
				x.Parameters = map[string]any{}
				for _, k := range []string{"size", "seed", "negative", "resolution", "aspect_ratio", "duration", "sound_enabled", "prompt", "negative_prompt"} {
					if v, ok := c[k]; ok {
						x.Parameters[k] = v
					}
				}
			}
		}
		x.Parameters = usagePriceTier(x.Parameters, price)
		items = append(items, x)
	}
	return items, rows.Err()
}
func (s *Store) ListUsageActivity(userID string, offset int) ([]UsageActivity, error) {
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query(activityQuery+` ORDER BY e.created_at DESC,e.id DESC LIMIT 51 OFFSET ?`, userID, userID, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanUsage(rows)
}
func (s *Store) ImageUsage(userID, assetID, characterID string) (*UsageActivity, error) {
	if assetID == "" && characterID == "" {
		return nil, nil
	}
	rows, err := s.db.Query(activityQuery+` WHERE (ch.id=? AND ?<>'') OR (COALESCE(d.output_asset_id,i.output_asset_id)=? AND ?<>'') OR (ch.path<>'' AND ?<>'' AND EXISTS(SELECT 1 FROM product_assets a WHERE a.id=? AND a.path=ch.path)) LIMIT 1`, userID, userID, characterID, characterID, assetID, assetID, assetID, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items, err := s.scanUsage(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		var old UsageActivity
		err := s.db.QueryRow(`SELECT id,name,status,created_at,id,'' FROM characters WHERE user_id=? AND source IN ('ai','edit','pose') AND (id=? OR (path<>'' AND EXISTS(SELECT 1 FROM product_assets a WHERE a.id=? AND a.path=characters.path)))
UNION ALL SELECT id,'商品图精修',status,created_at,'',output_asset_id FROM image_transform_jobs WHERE user_id=? AND output_asset_id=? AND output_asset_id<>'' LIMIT 1`, userID, characterID, assetID, userID, assetID).Scan(&old.TaskID, &old.Title, &old.TaskStatus, &old.CreatedAt, &old.CharacterID, &old.AssetID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		old.ID = old.TaskID
		old.Source = "historical"
		old.BillingStatus = "unknown"
		return &old, nil
	}
	return &items[0], nil
}
func (s *Store) UsageTotals(userID string) (map[string]int64, error) {
	totals := map[string]int64{"consumed": 0, "reserved": 0, "refunded": 0}
	rows, err := s.db.Query(`SELECT status,COALESCE(SUM(points),0) FROM credit_charges WHERE user_id=? GROUP BY status`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var points int64
		if err := rows.Scan(&status, &points); err != nil {
			return nil, err
		}
		totals[status] = points
	}
	return totals, rows.Err()
}
