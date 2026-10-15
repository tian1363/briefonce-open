package jobs

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/tian1363/scriptagent/internal/detailpage"
)

type DetailImageJob struct {
	ID            string          `json:"id"`
	BatchID       string          `json:"batch_id"`
	ProductID     string          `json:"product_id"`
	Task          detailpage.Task `json:"task"`
	Status        string          `json:"status"`
	OutputAssetID string          `json:"output_asset_id"`
	Error         string          `json:"error"`
	CreatedAt     string          `json:"created_at"`
}

func (s *Store) migrateDetailImages() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS detail_image_batches (
 id TEXT PRIMARY KEY,user_id TEXT NOT NULL,product_id TEXT NOT NULL,request_id TEXT NOT NULL,input_hash TEXT NOT NULL,plan_json TEXT NOT NULL,created_at TEXT NOT NULL,
 UNIQUE(user_id,product_id,request_id));
 CREATE TABLE IF NOT EXISTS detail_image_jobs (
 id TEXT PRIMARY KEY,batch_id TEXT NOT NULL REFERENCES detail_image_batches(id),user_id TEXT NOT NULL,product_id TEXT NOT NULL,
 task_json TEXT NOT NULL,status TEXT NOT NULL,output_asset_id TEXT NOT NULL DEFAULT '',error_message TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS idx_detail_image_product ON detail_image_jobs(user_id,product_id,created_at DESC);`)
	return err
}

// Atomic idempotency precedes both reservation and provider calls. A repeated key
// with different content is rejected rather than silently reusing the old batch.
func (s *Store) CreateDetailImageBatch(userID, productID, key, plan string, tasks []detailpage.Task) ([]DetailImageJob, bool, error) {
	if len(tasks) == 0 || len(tasks) > 40 || key == "" || len(key) > 100 {
		return nil, false, errors.New("图片任务数量或请求标识无效")
	}
	raw, _ := json.Marshal(struct {
		Plan  string
		Tasks []detailpage.Task
	}{plan, tasks})
	hash := fmt.Sprintf("%x", sha256.Sum256(raw))
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var batchID, existingHash string
	err := s.db.QueryRow(`SELECT id,input_hash FROM detail_image_batches WHERE user_id=? AND product_id=? AND request_id=?`, userID, productID, key).Scan(&batchID, &existingHash)
	if err == nil {
		if existingHash != hash {
			return nil, false, errors.New("请求标识已用于其他方案，请重新提交")
		}
		list, err := s.detailImageRows(`SELECT id,batch_id,product_id,task_json,status,output_asset_id,error_message,created_at FROM detail_image_jobs WHERE user_id=? AND batch_id=? ORDER BY rowid`, userID, batchID)
		return list, false, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	batchID = newID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.Exec(`INSERT INTO detail_image_batches VALUES(?,?,?,?,?,?,?)`, batchID, userID, productID, key, hash, plan, now); err != nil {
		return nil, false, err
	}
	items := make([]DetailImageJob, 0, len(tasks))
	for _, task := range tasks {
		item := DetailImageJob{ID: newID(), BatchID: batchID, ProductID: productID, Task: task, Status: "queued", CreatedAt: now}
		data, _ := json.Marshal(task)
		if _, err = tx.Exec(`INSERT INTO detail_image_jobs(id,batch_id,user_id,product_id,task_json,status,created_at,updated_at) VALUES(?,?,?,?,?,'queued',?,?)`, item.ID, batchID, userID, productID, string(data), now, now); err != nil {
			return nil, false, err
		}
		items = append(items, item)
	}
	return items, true, tx.Commit()
}
func (s *Store) detailImageRows(query string, args ...any) ([]DetailImageJob, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DetailImageJob{}
	for rows.Next() {
		var item DetailImageJob
		var raw string
		if err := rows.Scan(&item.ID, &item.BatchID, &item.ProductID, &raw, &item.Status, &item.OutputAssetID, &item.Error, &item.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &item.Task); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) ListDetailImageJobs(userID, productID string) ([]DetailImageJob, error) {
	return s.detailImageRows(`SELECT id,batch_id,product_id,task_json,status,output_asset_id,error_message,created_at FROM detail_image_jobs WHERE user_id=? AND product_id=? ORDER BY created_at DESC,rowid DESC LIMIT 240`, userID, productID)
}
func (s *Store) GetDetailImageJob(userID, productID, id string) (DetailImageJob, error) {
	items, err := s.detailImageRows(`SELECT id,batch_id,product_id,task_json,status,output_asset_id,error_message,created_at FROM detail_image_jobs WHERE user_id=? AND product_id=? AND id=?`, userID, productID, id)
	if err != nil {
		return DetailImageJob{}, err
	}
	if len(items) == 0 {
		return DetailImageJob{}, sql.ErrNoRows
	}
	return items[0], nil
}
func (s *Store) UpdateDetailImageJob(userID, id, status, assetID, message string) error {
	if status != "generating" && status != "ready" && status != "failed" {
		return errors.New("任务状态无效")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.Exec(`UPDATE detail_image_jobs SET status=?,output_asset_id=?,error_message=?,updated_at=? WHERE id=? AND user_id=? AND status IN ('queued','generating')`, status, assetID, message, time.Now().UTC().Format(time.RFC3339Nano), id, userID)
	if err == nil {
		n, _ := result.RowsAffected()
		if n != 1 {
			return errors.New("图片任务状态已变化")
		}
	}
	return err
}

// Only queued jobs are known not to have reached the provider. Running jobs
// remain subject to the existing conservative stale-charge reconciliation.
func (s *Store) InterruptDetailImageJobs() error {
	rows, err := s.db.Query(`SELECT t.charge_id FROM detail_image_jobs d JOIN usage_tasks t ON t.task_id=d.id AND t.user_id=d.user_id WHERE d.status='queued' AND t.charge_id<>''`)
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
		if err := s.FinishUsage(id, false); err != nil {
			return err
		}
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = s.db.Exec(`UPDATE detail_image_jobs SET status='failed',error_message=CASE WHEN status='queued' THEN '服务重启，尚未开始的任务已取消，可单张重试。' ELSE '服务重启，任务已中断。请核对积分记录后单张重试。' END,updated_at=? WHERE status IN ('queued','generating')`, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
