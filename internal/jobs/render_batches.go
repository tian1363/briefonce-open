package jobs

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type RenderBatch struct {
	ID              string          `json:"id"`
	ProductID       string          `json:"product_id"`
	SourceAssetID   string          `json:"source_asset_id"`
	SourceVideoID   string          `json:"source_video_id,omitempty"`
	SourceBatchID   string          `json:"source_batch_id,omitempty"`
	SourceVariantID string          `json:"source_variant_id,omitempty"`
	Title           string          `json:"title"`
	Status          string          `json:"status"`
	CreatedAt       string          `json:"created_at"`
	Variants        []RenderVariant `json:"variants"`
}

type RenderVariant struct {
	ID            string           `json:"id"`
	BatchID       string           `json:"batch_id"`
	Title         string           `json:"title"`
	Copy          string           `json:"copy"`
	OverlayPath   string           `json:"-"`
	Segments      []CaptionSegment `json:"segments,omitempty"`
	CoverExisting bool             `json:"cover_existing"`
	OutputPath    string           `json:"-"`
	Status        string           `json:"status"`
	ErrorMessage  string           `json:"error_message,omitempty"`
}

type CaptionSegment struct {
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Text        string  `json:"text"`
	OverlayPath string  `json:"-"`
}

type storedCaptionSegment struct {
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Text        string  `json:"text"`
	OverlayPath string  `json:"overlay_path"`
}

func encodeCaptionSegments(items []CaptionSegment) (string, error) {
	stored := make([]storedCaptionSegment, len(items))
	for i, item := range items {
		stored[i] = storedCaptionSegment{item.Start, item.End, item.Text, item.OverlayPath}
	}
	data, err := json.Marshal(stored)
	return string(data), err
}

func decodeCaptionSegments(data string) ([]CaptionSegment, error) {
	var stored []storedCaptionSegment
	if err := json.Unmarshal([]byte(data), &stored); err != nil {
		return nil, err
	}
	items := make([]CaptionSegment, len(stored))
	for i, item := range stored {
		items[i] = CaptionSegment{item.Start, item.End, item.Text, item.OverlayPath}
	}
	return items, nil
}

func (s *Store) CreateRenderBatch(userID, productID, sourceID, title string, variants []RenderVariant) (*RenderBatch, error) {
	return s.CreateRenderBatchWithVideo(userID, productID, sourceID, "", title, variants)
}

func (s *Store) CreateRenderBatchWithVideo(userID, productID, sourceID, sourceVideoID, title string, variants []RenderVariant) (*RenderBatch, error) {
	return s.CreateRenderBatchWithSource(userID, productID, sourceID, sourceVideoID, "", "", title, variants)
}

func (s *Store) CreateRenderBatchWithSource(userID, productID, sourceID, sourceVideoID, sourceBatchID, sourceVariantID, title string, variants []RenderVariant) (*RenderBatch, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	b := &RenderBatch{ID: newID(), ProductID: productID, SourceAssetID: sourceID, SourceVideoID: sourceVideoID, SourceBatchID: sourceBatchID, SourceVariantID: sourceVariantID, Title: title, Status: "pending", CreatedAt: now}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO render_batches(id,user_id,product_id,source_asset_id,source_video_id,source_batch_id,source_variant_id,title,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, b.ID, userID, productID, sourceID, sourceVideoID, sourceBatchID, sourceVariantID, title, b.Status, now); err != nil {
		return nil, err
	}
	for index, v := range variants {
		v.ID, v.BatchID, v.Status = newID(), b.ID, "pending"
		segmentsJSON, marshalErr := encodeCaptionSegments(v.Segments)
		if marshalErr != nil {
			return nil, marshalErr
		}
		if _, err = tx.Exec(`INSERT INTO render_variants(id,batch_id,title,copy,position,overlay_path,segments_json,cover_existing,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, v.ID, b.ID, v.Title, v.Copy, index, v.OverlayPath, segmentsJSON, v.CoverExisting, v.Status, now, now); err != nil {
			return nil, err
		}
		b.Variants = append(b.Variants, v)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Store) GetRenderBatch(userID, id string) (*RenderBatch, error) {
	var b RenderBatch
	err := s.db.QueryRow(`SELECT id,product_id,source_asset_id,source_video_id,source_batch_id,source_variant_id,title,status,created_at FROM render_batches WHERE user_id=? AND id=?`, userID, id).Scan(&b.ID, &b.ProductID, &b.SourceAssetID, &b.SourceVideoID, &b.SourceBatchID, &b.SourceVariantID, &b.Title, &b.Status, &b.CreatedAt)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT id,batch_id,title,copy,overlay_path,segments_json,cover_existing,COALESCE(output_path,''),status,COALESCE(error_message,'') FROM render_variants WHERE batch_id=? ORDER BY position,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	b.Variants = []RenderVariant{}
	for rows.Next() {
		var v RenderVariant
		var segmentsJSON string
		if err := rows.Scan(&v.ID, &v.BatchID, &v.Title, &v.Copy, &v.OverlayPath, &segmentsJSON, &v.CoverExisting, &v.OutputPath, &v.Status, &v.ErrorMessage); err != nil {
			return nil, err
		}
		v.Segments, err = decodeCaptionSegments(segmentsJSON)
		if err != nil {
			return nil, err
		}
		b.Variants = append(b.Variants, v)
	}
	return &b, rows.Err()
}

func (s *Store) ListRenderBatches(userID string) ([]RenderBatch, error) {
	rows, err := s.db.Query(`SELECT id FROM render_batches WHERE user_id=? ORDER BY created_at DESC LIMIT 50`, userID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	result := make([]RenderBatch, 0, len(ids))
	for _, id := range ids {
		b, err := s.GetRenderBatch(userID, id)
		if err != nil {
			return nil, err
		}
		result = append(result, *b)
	}
	return result, nil
}

func (s *Store) ListUnfinishedRenderBatchIDs(userID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT id FROM render_batches WHERE user_id=? AND status IN ('pending','running') ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) SetRenderVariant(id, status, path, message string) error {
	_, err := s.db.Exec(`UPDATE render_variants SET status=?,output_path=NULLIF(?,''),error_message=NULLIF(?,''),updated_at=? WHERE id=?`, status, path, message, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (s *Store) SetRenderBatchStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE render_batches SET status=? WHERE id=?`, status, id)
	return err
}

func (s *Store) RetryRenderVariant(userID, batchID, variantID string) (*RenderVariant, error) {
	b, err := s.GetRenderBatch(userID, batchID)
	if err != nil {
		return nil, err
	}
	if b.Status == "pending" || b.Status == "running" {
		return nil, errors.New("请等待本批次完成后再重试")
	}
	for _, v := range b.Variants {
		if v.ID == variantID {
			if v.Status == "running" || v.Status == "pending" {
				return nil, errors.New("版本仍在制作")
			}
			if v.Status == "completed" {
				return nil, errors.New("已完成版本不能直接覆盖，请创建新批次")
			}
			tx, err := s.db.Begin()
			if err != nil {
				return nil, err
			}
			defer tx.Rollback()
			result, err := tx.Exec(`UPDATE render_variants SET status='pending',output_path=NULL,error_message=NULL,updated_at=? WHERE id=? AND batch_id=? AND status='failed' AND EXISTS(SELECT 1 FROM render_batches WHERE id=? AND status='needs_review')`, time.Now().UTC().Format(time.RFC3339), v.ID, batchID, batchID)
			if err != nil {
				return nil, err
			}
			if count, _ := result.RowsAffected(); count != 1 {
				return nil, errors.New("版本状态已变化，请刷新后再试")
			}
			if _, err := tx.Exec(`UPDATE render_batches SET status='running' WHERE id=?`, batchID); err != nil {
				return nil, err
			}
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			v.Status = "pending"
			return &v, nil
		}
	}
	return nil, sql.ErrNoRows
}
