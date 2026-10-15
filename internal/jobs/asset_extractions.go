package jobs

import (
	"database/sql"
	"time"
)

// AssetExtraction records the source frame and the generated, reusable asset.
type AssetExtraction struct {
	ID             string                   `json:"id"`
	UserID         string                   `json:"-"`
	ProductID      string                   `json:"product_id,omitempty"`
	Kind           string                   `json:"kind"`
	Label          string                   `json:"label"`
	SourceType     string                   `json:"source_type"`
	SourceID       string                   `json:"source_id"`
	SourceAuxID    string                   `json:"source_aux_id,omitempty"`
	AtSeconds      float64                  `json:"at_seconds"`
	Status         string                   `json:"status"`
	FramePath      string                   `json:"-"`
	ImagePath      string                   `json:"-"`
	TargetID       string                   `json:"target_id,omitempty"`
	ProductAssetID string                   `json:"product_asset_id,omitempty"`
	Analysis       string                   `json:"analysis,omitempty"`
	Variants       []AssetExtractionVariant `json:"variants,omitempty"`
	Error          string                   `json:"error,omitempty"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
}

// AssetExtractionVariant is backed by a distinct observed video frame. Paths
// stay server-side; callers fetch images through authenticated routes.
type AssetExtractionVariant struct {
	View           string  `json:"view"`
	AtSeconds      float64 `json:"at_seconds"`
	FramePath      string  `json:"-"`
	ImagePath      string  `json:"-"`
	VideoPath      string  `json:"-"`
	ProductAssetID string  `json:"product_asset_id,omitempty"`
}

func (s *Store) migrateAssetExtractions() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS asset_extractions (
		id TEXT PRIMARY KEY, user_id TEXT NOT NULL, product_id TEXT NOT NULL DEFAULT '',
		kind TEXT NOT NULL, label TEXT NOT NULL, source_type TEXT NOT NULL,
		source_id TEXT NOT NULL, source_aux_id TEXT NOT NULL DEFAULT '', at_seconds REAL NOT NULL,
		status TEXT NOT NULL, frame_path TEXT NOT NULL DEFAULT '', image_path TEXT NOT NULL DEFAULT '',
		target_id TEXT NOT NULL DEFAULT '', error_message TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_asset_extractions_user ON asset_extractions(user_id, created_at DESC);`)
	if err != nil {
		return err
	}
	if err = s.ensureColumn("asset_extractions", "product_asset_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err = s.ensureColumn("asset_extractions", "analysis", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS asset_extraction_variants (
		extraction_id TEXT NOT NULL, variant_index INTEGER NOT NULL, view TEXT NOT NULL,
		at_seconds REAL NOT NULL, frame_path TEXT NOT NULL, image_path TEXT NOT NULL,
		video_path TEXT NOT NULL DEFAULT '',
		product_asset_id TEXT NOT NULL DEFAULT '',
		PRIMARY KEY(extraction_id, variant_index),
		FOREIGN KEY(extraction_id) REFERENCES asset_extractions(id) ON DELETE CASCADE
	)`)
	if err != nil {
		return err
	}
	return s.ensureColumn("asset_extraction_variants", "video_path", "TEXT NOT NULL DEFAULT ''")
}

func (s *Store) CreateAssetExtraction(item AssetExtraction) (*AssetExtraction, error) {
	item.ID = newID()
	item.CreatedAt, item.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	item.Status = "generating"
	_, err := s.db.Exec(`INSERT INTO asset_extractions
		(id,user_id,product_id,kind,label,source_type,source_id,source_aux_id,at_seconds,status,frame_path,image_path,target_id,error_message,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.UserID, item.ProductID, item.Kind, item.Label,
		item.SourceType, item.SourceID, item.SourceAuxID, item.AtSeconds, item.Status, item.FramePath,
		item.ImagePath, item.TargetID, item.Error, item.CreatedAt.Format(time.RFC3339), item.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) GetAssetExtraction(userID, id string) (*AssetExtraction, error) {
	var item AssetExtraction
	var created, updated string
	err := s.db.QueryRow(`SELECT id,user_id,product_id,kind,label,source_type,source_id,source_aux_id,at_seconds,status,frame_path,image_path,target_id,error_message,created_at,updated_at,product_asset_id,analysis FROM asset_extractions WHERE user_id=? AND id=?`, userID, id).
		Scan(&item.ID, &item.UserID, &item.ProductID, &item.Kind, &item.Label, &item.SourceType, &item.SourceID, &item.SourceAuxID, &item.AtSeconds, &item.Status, &item.FramePath, &item.ImagePath, &item.TargetID, &item.Error, &created, &updated, &item.ProductAssetID, &item.Analysis)
	item.CreatedAt, item.UpdatedAt = parseTime(created), parseTime(updated)
	if err == nil {
		rows, queryErr := s.db.Query(`SELECT view,at_seconds,frame_path,image_path,video_path,product_asset_id FROM asset_extraction_variants WHERE extraction_id=? ORDER BY variant_index`, id)
		if queryErr != nil {
			return nil, queryErr
		}
		defer rows.Close()
		for rows.Next() {
			var v AssetExtractionVariant
			if scanErr := rows.Scan(&v.View, &v.AtSeconds, &v.FramePath, &v.ImagePath, &v.VideoPath, &v.ProductAssetID); scanErr != nil {
				return nil, scanErr
			}
			item.Variants = append(item.Variants, v)
		}
		if queryErr = rows.Err(); queryErr != nil {
			return nil, queryErr
		}
	}
	return &item, err
}

func (s *Store) ListAssetExtractions(userID string) ([]AssetExtraction, error) {
	rows, err := s.db.Query(`SELECT id FROM asset_extractions WHERE user_id=? ORDER BY created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	ids := []string{}
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
	items := make([]AssetExtraction, 0, len(ids))
	for _, id := range ids {
		item, err := s.GetAssetExtraction(userID, id)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, nil
}

func (s *Store) FinishAssetExtraction(userID, id, status, framePath, imagePath, targetID, message string) error {
	result, err := s.db.Exec(`UPDATE asset_extractions SET status=?,frame_path=?,image_path=?,target_id=?,error_message=?,updated_at=? WHERE user_id=? AND id=? AND status='generating'`, status, framePath, imagePath, targetID, message, time.Now().UTC().Format(time.RFC3339), userID, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) CompleteAssetExtraction(userID, id, framePath, imagePath, targetID, productAssetID, analysis string) error {
	return s.CompleteAssetExtractionWithVariants(userID, id, framePath, imagePath, targetID, productAssetID, analysis, nil)
}

func (s *Store) CompleteAssetExtractionWithVariants(userID, id, framePath, imagePath, targetID, productAssetID, analysis string, variants []AssetExtractionVariant) error {
	return s.completeAssetExtractionWithVariants(userID, id, "ready", framePath, imagePath, targetID, productAssetID, analysis, "", variants)
}

func (s *Store) CompleteAssetExtractionForReview(userID, id, framePath, imagePath, analysis, message string, variants []AssetExtractionVariant) error {
	return s.completeAssetExtractionWithVariants(userID, id, "needs_review", framePath, imagePath, "", "", analysis, message, variants)
}

func (s *Store) completeAssetExtractionWithVariants(userID, id, status, framePath, imagePath, targetID, productAssetID, analysis, message string, variants []AssetExtractionVariant) error {
	selectedAt := -1.0
	if len(variants) > 0 {
		selectedAt = variants[0].AtSeconds
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE asset_extractions SET status=?,frame_path=?,image_path=?,target_id=?,product_asset_id=?,analysis=?,at_seconds=CASE WHEN ?>=0 THEN ? ELSE at_seconds END,error_message=?,updated_at=? WHERE user_id=? AND id=? AND status='generating'`, status, framePath, imagePath, targetID, productAssetID, analysis, selectedAt, selectedAt, message, time.Now().UTC().Format(time.RFC3339), userID, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	for index, variant := range variants {
		if _, err = tx.Exec(`INSERT INTO asset_extraction_variants(extraction_id,variant_index,view,at_seconds,frame_path,image_path,video_path,product_asset_id) VALUES(?,?,?,?,?,?,?,?)`, id, index, variant.View, variant.AtSeconds, variant.FramePath, variant.ImagePath, variant.VideoPath, variant.ProductAssetID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ApproveAssetExtractionReview(userID, id string) error {
	result, err := s.db.Exec(`UPDATE asset_extractions SET status='ready',error_message='',analysis=analysis||? ,updated_at=? WHERE user_id=? AND id=? AND status='needs_review' AND image_path<>''`, "\n\n用户已对照原帧确认使用此生成图；请在发布前再次核对商品与人物细节。", time.Now().UTC().Format(time.RFC3339), userID, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) SetAssetExtractionTarget(userID, id, targetID string) error {
	result, err := s.db.Exec(`UPDATE asset_extractions SET target_id=?,updated_at=? WHERE user_id=? AND id=? AND status='ready' AND target_id=''`, targetID, time.Now().UTC().Format(time.RFC3339), userID, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) SetAssetExtractionVariantAssets(userID, id, productID string, assetIDs []string) error {
	if len(assetIDs) == 0 {
		return sql.ErrNoRows
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM asset_extraction_variants WHERE extraction_id=?`, id).Scan(&count); err != nil {
		return err
	}
	if count != len(assetIDs) {
		return sql.ErrNoRows
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE asset_extractions SET product_id=?,product_asset_id=?,updated_at=? WHERE user_id=? AND id=? AND status='ready' AND product_id=''`, productID, assetIDs[0], time.Now().UTC().Format(time.RFC3339), userID, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	for index, assetID := range assetIDs {
		if _, err = tx.Exec(`UPDATE asset_extraction_variants SET product_asset_id=? WHERE extraction_id=? AND variant_index=?`, assetID, id, index); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SetAssetExtractionProduct(userID, id, productID, productAssetID string) error {
	result, err := s.db.Exec(`UPDATE asset_extractions SET product_id=?,product_asset_id=?,updated_at=? WHERE user_id=? AND id=? AND status='ready' AND product_id=''`, productID, productAssetID, time.Now().UTC().Format(time.RFC3339), userID, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) InterruptAssetExtractions() error {
	_, err := s.db.Exec(`UPDATE asset_extractions SET status='failed',error_message='服务重启，请重新提取',updated_at=? WHERE status='generating'`, time.Now().UTC().Format(time.RFC3339))
	return err
}
