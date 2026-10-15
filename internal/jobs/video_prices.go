package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/tian1363/scriptagent/internal/model"
	"github.com/tian1363/scriptagent/internal/videogen"
	"strings"
)

func validVideoTier(model, tier string) bool {
	m, ok := videogen.FindModel(model)
	if !ok {
		return false
	}
	for _, r := range m.Resolutions {
		if tier == r+":standard" || tier == r+":video_reference" {
			return true
		}
	}
	return false
}
func (s *Store) migrateVideoPrices() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS video_runtime_snapshots(video_id TEXT PRIMARY KEY,user_id TEXT NOT NULL,config TEXT NOT NULL); CREATE TABLE IF NOT EXISTS video_prices(model TEXT NOT NULL,tier TEXT NOT NULL,enabled INTEGER NOT NULL DEFAULT 0,cost_micros INTEGER NOT NULL DEFAULT 0,multiplier_bps INTEGER NOT NULL DEFAULT 0,point_micros INTEGER NOT NULL DEFAULT 0,fixed_points INTEGER NOT NULL DEFAULT 0,version INTEGER NOT NULL DEFAULT 1,actor TEXT NOT NULL DEFAULT '',updated_at TEXT NOT NULL,PRIMARY KEY(model,tier))`)
	if err != nil {
		return err
	}
	for _, m := range videogen.Models {
		full, _ := videogen.FindModel(m.ID)
		for _, r := range full.Resolutions {
			for _, kind := range []string{"standard", "video_reference"} {
				if _, err = s.db.Exec(`INSERT OR IGNORE INTO video_prices(model,tier,updated_at) VALUES(?,?,?)`, m.ID, r+":"+kind, nowUTC()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
func (s *Store) VideoQuote(model, resolution string, seconds int, videoReference bool, version int64) (ImagePrice, int64, error) {
	p := ImagePrice{}
	if seconds < 4 || seconds > 15 {
		return p, 0, errors.New("时长需为 4 到 15 秒")
	}
	tier := strings.ToLower(resolution) + ":standard"
	if videoReference {
		tier = strings.ToLower(resolution) + ":video_reference"
	}
	if !validVideoTier(model, tier) {
		return p, 0, errors.New("视频模型参数无效")
	}
	err := s.db.QueryRow(`SELECT model,tier,enabled,cost_micros,multiplier_bps,point_micros,fixed_points,version,actor,updated_at FROM video_prices WHERE model=? AND tier=?`, model, tier).Scan(&p.Model, &p.Tier, &p.Enabled, &p.CostMicros, &p.MultiplierBPS, &p.PointMicros, &p.FixedPoints, &p.Version, &p.Actor, &p.UpdatedAt)
	if err != nil {
		return p, 0, err
	}
	if version != 0 && version != p.Version {
		return p, 0, errors.New("视频报价已更新，请刷新后重试")
	}
	if _, err = p.Points(); err != nil {
		return p, 0, errors.New("此视频组合尚未配置供应商成本或启用，请联系管理员")
	}
	total := p
	total.CostMicros *= int64(seconds)
	total.FixedPoints *= int64(seconds)
	points, err := total.Points()
	return total, points, err
}
func (s *Store) ReserveVideoUsage(ctx context.Context, model, resolution string, seconds int, videoReference bool, version int64) (string, ImagePrice, error) {
	p, points, err := s.VideoQuote(model, resolution, seconds, videoReference, version)
	if err != nil {
		return "", p, err
	}
	id, err := s.ReserveUsage(context.WithValue(ctx, quoteContextKey{}, quoteSnapshot{Points: points, Price: p}), "video_generation", 1)
	return id, p, err
}
func (s *Store) VideoPrices() ([]ImagePrice, error) {
	rows, err := s.db.Query(`SELECT model,tier,enabled,cost_micros,multiplier_bps,point_micros,fixed_points,version,actor,updated_at FROM video_prices ORDER BY rowid`)
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
func (s *Store) SaveVideoPrice(p ImagePrice, actor, note string) error {
	if !validVideoTier(p.Model, p.Tier) {
		return errors.New("视频模型或计价档位无效")
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
	_ = tx.QueryRow(`SELECT json_object('cost_micros',cost_micros,'multiplier_bps',multiplier_bps,'point_micros',point_micros,'fixed_points',fixed_points,'enabled',enabled,'version',version) FROM video_prices WHERE model=? AND tier=?`, p.Model, p.Tier).Scan(&old)
	_, err = tx.Exec(`UPDATE video_prices SET enabled=?,cost_micros=?,multiplier_bps=?,point_micros=?,fixed_points=?,version=version+1,actor=?,updated_at=? WHERE model=? AND tier=?`, p.Enabled, p.CostMicros, p.MultiplierBPS, p.PointMicros, p.FixedPoints, actor, nowUTC(), p.Model, p.Tier)
	if err != nil {
		return err
	}
	if err = auditTx(tx, actor, "video_price", p.Model+":"+p.Tier, old, p, note); err != nil {
		return err
	}
	return tx.Commit()
}

// Replace retired video profiles without touching generated works or other capabilities.
func (s *Store) migrateVideoProfiles() error {
	_, err := s.db.Exec(`INSERT INTO platform_model_profiles(capability,provider,model,endpoint,encrypted_key,updated_at,account_ref)
 SELECT 'video_generation','zexapi','seedance-2.0','https://zexapi.com/v1/videos',encrypted_key,updated_at,account_ref FROM platform_model_profiles WHERE capability='image_generation' AND provider='zexapi'
 ON CONFLICT(capability) DO UPDATE SET provider=excluded.provider,model=excluded.model,endpoint=excluded.endpoint,encrypted_key=excluded.encrypted_key,account_ref=excluded.account_ref WHERE platform_model_profiles.provider<>'zexapi';
 UPDATE platform_model_profiles SET provider='zexapi',model='seedance-2.0',endpoint='https://zexapi.com/v1/videos',encrypted_key='' WHERE capability='video_generation' AND provider<>'zexapi';
 DELETE FROM user_model_capability_settings WHERE capability='video_generation' AND provider<>'zexapi';`)
	return err
}

func (s *Store) SaveVideoRuntime(id, userID string, cfg model.RuntimeConfig) error {
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	encrypted, err := s.encryptSecret(string(b))
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO video_runtime_snapshots(video_id,user_id,config) VALUES(?,?,?) ON CONFLICT(video_id) DO UPDATE SET config=excluded.config WHERE video_runtime_snapshots.user_id=excluded.user_id`, id, userID, encrypted)
	return err
}
func (s *Store) VideoRuntime(id, userID string) (model.RuntimeConfig, error) {
	var cfg model.RuntimeConfig
	var encrypted string
	if err := s.db.QueryRow(`SELECT config FROM video_runtime_snapshots WHERE video_id=? AND user_id=?`, id, userID).Scan(&encrypted); err != nil {
		return cfg, err
	}
	raw, err := s.decryptSecret(encrypted)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal([]byte(raw), &cfg)
	return cfg, err
}
