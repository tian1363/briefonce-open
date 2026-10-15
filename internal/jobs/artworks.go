package jobs

// Artwork points to a durable generated image. Canvas layout state is never its source.
type Artwork struct {
	ID           string `json:"id"`
	AssetID      string `json:"asset_id"`
	ProductID    string `json:"product_id"`
	ProductTitle string `json:"product_title"`
	Name         string `json:"name"`
	Source       string `json:"source"`
	URL          string `json:"url"`
	CreatedAt    string `json:"created_at"`
}

func (s *Store) ListArtworks(userID, productID string, offset int) ([]Artwork, error) {
	rows, err := s.db.Query(`WITH owned_assets AS (
 SELECT a.*,p.title AS product_title FROM product_assets a JOIN products p ON p.id=a.product_id
 JOIN resource_owners o ON o.resource_id=p.id AND o.resource_type='product' AND o.user_id=?
 ), works AS (
 SELECT a.id,a.id AS asset_id,a.product_id,a.product_title,a.original_name AS name,
 CASE WHEN a.kind='artwork' THEN 'detail' ELSE 'product-photo' END AS source,
 '/api/assets/'||a.id||'/file' AS url,a.created_at
 FROM owned_assets a WHERE a.kind='artwork' OR EXISTS (
 SELECT 1 FROM image_transform_jobs j WHERE j.output_asset_id=a.id AND j.user_id=? AND j.status='ready')
 UNION ALL
 SELECT c.id,COALESCE(a.id,''),COALESCE(a.product_id,''),COALESCE(a.product_title,''),c.name,c.source,
 '/api/characters/'||c.id||'/image',c.created_at
 FROM characters c LEFT JOIN owned_assets a ON a.id=(SELECT x.id FROM owned_assets x WHERE x.path=c.path ORDER BY x.id LIMIT 1)
 WHERE c.user_id=? AND c.status='ready' AND c.source IN ('pose','ai','edit')
 ) SELECT id,asset_id,product_id,product_title,name,source,url,created_at FROM works
 WHERE (?='' OR product_id=? OR (?='unassigned' AND product_id=''))
 ORDER BY created_at DESC,id DESC LIMIT 49 OFFSET ?`, userID, userID, userID, productID, productID, productID, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Artwork{}
	for rows.Next() {
		var a Artwork
		if err := rows.Scan(&a.ID, &a.AssetID, &a.ProductID, &a.ProductTitle, &a.Name, &a.Source, &a.URL, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}
