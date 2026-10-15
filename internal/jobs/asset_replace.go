package jobs

import (
	"database/sql"
	"time"
)

// ReplaceProductAsset versions a product attachment without rewriting historical media.
func (s *Store) ReplaceProductAsset(oldID string, asset ProductAsset) (*ProductAsset, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var productID string
	if err = tx.QueryRow(`SELECT product_id FROM product_assets WHERE id=? AND id NOT IN (SELECT asset_id FROM product_asset_detachments)`, oldID).Scan(&productID); err != nil {
		return nil, err
	}
	if productID != asset.ProductID {
		return nil, sql.ErrNoRows
	}
	asset.ID, asset.CreatedAt = newID(), time.Now().UTC()
	_, err = tx.Exec(`INSERT INTO product_assets(id,product_id,kind,path,original_name,mime_type,size_bytes,created_at) VALUES(?,?,?,?,?,?,?,?)`, asset.ID, asset.ProductID, asset.Kind, asset.Path, asset.OriginalName, asset.MimeType, asset.SizeBytes, asset.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO product_asset_detachments(asset_id,detached_at) VALUES(?,?)`, oldID, asset.CreatedAt.Format(time.RFC3339)); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO product_asset_replacements(old_asset_id,new_asset_id) VALUES(?,?)`, oldID, asset.ID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &asset, nil
}

func (s *Store) CurrentProductAssetID(id string) string {
	for i := 0; i < 100; i++ {
		var next string
		if err := s.db.QueryRow(`SELECT new_asset_id FROM product_asset_replacements WHERE old_asset_id=?`, id).Scan(&next); err != nil {
			return id
		}
		id = next
	}
	return id
}
