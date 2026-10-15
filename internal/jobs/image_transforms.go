package jobs

import (
	"errors"
	"time"
)

type ImageTransformJob struct {
	ID               string `json:"id"`
	ProductID        string `json:"product_id"`
	SourceAssetID    string `json:"source_asset_id"`
	OutputAssetID    string `json:"output_asset_id"`
	BatchID          string `json:"batch_id"`
	Mode             string `json:"mode"`
	DetailFocus      string `json:"detail_focus"`
	ReferenceAssetID string `json:"reference_asset_id,omitempty"`
	ReferenceFocus   string `json:"reference_focus,omitempty"`
	Status           string `json:"status"`
	Error            string `json:"error"`
	Position         int    `json:"position"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

func (s *Store) CreateImageTransformBatch(userID, productID, sourceAssetID, batchID, focus, referenceAssetID, referenceFocus string, modes []string) ([]ImageTransformJob, error) {
	if len(modes) == 0 || len(modes) > 2 {
		return nil, errors.New("请选择一到两种商品图处理方式")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	items := make([]ImageTransformJob, 0, len(modes))
	for i, mode := range modes {
		item := ImageTransformJob{ID: newID(), ProductID: productID, SourceAssetID: sourceAssetID, BatchID: batchID, Mode: mode, DetailFocus: focus, ReferenceAssetID: referenceAssetID, ReferenceFocus: referenceFocus, Status: "queued", Position: i, CreatedAt: now, UpdatedAt: now}
		if _, err := tx.Exec(`INSERT INTO image_transform_jobs(id,user_id,product_id,source_asset_id,batch_id,mode,detail_focus,reference_asset_id,reference_focus,status,position,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, userID, productID, sourceAssetID, batchID, mode, focus, referenceAssetID, referenceFocus, item.Status, i, now, now); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, tx.Commit()
}

func (s *Store) ListImageTransformJobs(userID, productID string) ([]ImageTransformJob, error) {
	rows, err := s.db.Query(`SELECT id,product_id,source_asset_id,output_asset_id,batch_id,mode,detail_focus,reference_asset_id,reference_focus,status,error_message,position,created_at,updated_at FROM image_transform_jobs WHERE user_id=? AND product_id=? ORDER BY created_at DESC,position ASC LIMIT 100`, userID, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ImageTransformJob{}
	for rows.Next() {
		var item ImageTransformJob
		if err := rows.Scan(&item.ID, &item.ProductID, &item.SourceAssetID, &item.OutputAssetID, &item.BatchID, &item.Mode, &item.DetailFocus, &item.ReferenceAssetID, &item.ReferenceFocus, &item.Status, &item.Error, &item.Position, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) StartImageTransformJob(userID, id string) error {
	_, err := s.db.Exec(`UPDATE image_transform_jobs SET status='generating',updated_at=? WHERE user_id=? AND id=? AND status='queued'`, time.Now().UTC().Format(time.RFC3339Nano), userID, id)
	return err
}

func (s *Store) FinishImageTransformJob(userID, id, outputAssetID, message string) error {
	status := "ready"
	if message != "" {
		status = "failed"
	}
	_, err := s.db.Exec(`UPDATE image_transform_jobs SET status=?,output_asset_id=?,error_message=?,updated_at=? WHERE user_id=? AND id=?`, status, outputAssetID, message, time.Now().UTC().Format(time.RFC3339Nano), userID, id)
	return err
}

func (s *Store) InterruptImageTransformJobs() error {
	_, err := s.db.Exec(`UPDATE image_transform_jobs SET status='failed',error_message='服务已重启，生成被中断。请检查用量记录后重新提交。',updated_at=? WHERE status IN ('generating','queued')`, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
