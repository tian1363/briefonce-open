package jobs

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"
)

// Events contain identifiers and controlled vocabulary only, never content or credentials.
type AnalyticsEvent struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Feature     string `json:"feature"`
	Entry       string `json:"entry"`
	SessionID   string `json:"session_id"`
	RequestID   string `json:"request_id"`
	TaskID      string `json:"task_id"`
	RefID       string `json:"ref_id"`
	Environment string `json:"-"`
	Effective   bool   `json:"-"`
}

var analyticsIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_-]{0,100}$`)

var analyticsFeatures = map[string]bool{"home": true, "spaces": true, "products": true, "toolbox": true, "analytics": true, "history": true, "chat": true, "video": true, "pose": true, "refine": true, "detail": true, "extract": true, "interpret": true, "character": true, "other": true}

func (s *Store) migrateAnalytics() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS analytics_events(
 id TEXT NOT NULL,user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,name TEXT NOT NULL,feature TEXT NOT NULL,entry TEXT NOT NULL,session_id TEXT NOT NULL DEFAULT '',request_id TEXT NOT NULL DEFAULT '',task_id TEXT NOT NULL DEFAULT '',ref_id TEXT NOT NULL DEFAULT '',environment TEXT NOT NULL,effective INTEGER NOT NULL,created_at TEXT NOT NULL,PRIMARY KEY(user_id,id));
 CREATE INDEX IF NOT EXISTS idx_analytics_user_date ON analytics_events(user_id,created_at);
 CREATE INDEX IF NOT EXISTS idx_analytics_active ON analytics_events(environment,effective,created_at,user_id);
 CREATE TABLE IF NOT EXISTS analytics_config(key TEXT PRIMARY KEY,value TEXT NOT NULL);`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT OR IGNORE INTO analytics_config(key,value) VALUES('started_at',?)`, time.Now().UTC().Format(time.RFC3339))
	return err
}
func (s *Store) RecordAnalytics(userID string, e AnalyticsEvent) error {
	if !analyticsIdentifier.MatchString(e.ID) || !analyticsIdentifier.MatchString(e.SessionID) || userID == "" || len(e.ID) > 100 || len(e.SessionID) > 100 || len(e.TaskID) > 100 || len(e.RefID) > 100 || len(e.RequestID) > 100 || !analyticsFeatures[e.Feature] {
		return errors.New("事件字段无效")
	}
	if e.ID == "" {
		e.ID = newID()
	}
	if e.Environment == "" {
		e.Environment = "production"
	}
	if e.Entry != "canvas" && e.Entry != "toolbox" && e.Entry != "composer" && e.Entry != "navigation" && e.Entry != "library" {
		e.Entry = "other"
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO analytics_events(id,user_id,name,feature,entry,session_id,request_id,task_id,ref_id,environment,effective,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, e.ID, userID, e.Name, e.Feature, e.Entry, e.SessionID, e.RequestID, e.TaskID, e.RefID, e.Environment, e.Effective, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

type AnalyticsCount struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}
type AnalyticsDay struct {
	ActiveUsers int    `json:"active_users"`
	Date        string `json:"date"`
	Visits      int    `json:"visits"`
	Actions     int    `json:"actions"`
	Tasks       int    `json:"tasks"`
	Completed   int    `json:"completed"`
}
type AnalyticsModel struct {
	Model   string `json:"model"`
	Count   int    `json:"count"`
	Success int    `json:"success"`
	Latency int64  `json:"latency_ms"`
}
type AnalyticsSummary struct {
	Scope           string           `json:"scope"`
	ActiveUsers     int              `json:"active_users"`
	Activity        map[string]int   `json:"activity,omitempty"`
	FeatureRequests []AnalyticsCount `json:"feature_requests"`
	FeatureOpens    []AnalyticsCount `json:"feature_opens"`
	LatencyP50      int64            `json:"latency_p50_ms"`
	LatencyP95      int64            `json:"latency_p95_ms"`
	LatencySamples  int              `json:"latency_samples"`
	Days            int              `json:"days"`
	StartedAt       string           `json:"started_at"`
	Timezone        string           `json:"timezone"`
	Visits          int              `json:"visits"`
	ActiveDays      int              `json:"active_days"`
	Tasks           int              `json:"tasks"`
	Completed       int              `json:"completed"`
	Failed          int              `json:"failed"`
	Pending         int              `json:"pending"`
	Credits         int64            `json:"credits"`
	Previews        int              `json:"previews"`
	Downloads       int              `json:"downloads"`
	Reused          int              `json:"reused"`
	Requests        int              `json:"requests"`
	Errors          int              `json:"errors"`
	Timeline        []AnalyticsDay   `json:"timeline"`
	Features        []AnalyticsCount `json:"features"`
	Entries         []AnalyticsCount `json:"entries"`
	Models          []AnalyticsModel `json:"models"`
	Parameters      []AnalyticsCount `json:"parameters"`
	Journey         []AnalyticsCount `json:"journey"`
}

// Canonical output tasks. Provider retries are not additional outputs. Parent batches
// and billing rows are deliberately absent, preventing multiplicative counting.
const analyticsTasks = `SELECT id,user_id,'pose' feature,status,created_at FROM characters WHERE source='pose'
 UNION ALL SELECT id,user_id,'character',status,created_at FROM characters WHERE source IN ('ai','edit')
 UNION ALL SELECT id,user_id,'refine',status,created_at FROM image_transform_jobs
 UNION ALL SELECT id,user_id,'detail',status,created_at FROM detail_image_jobs
 UNION ALL SELECT id,user_id,'video',status,created_at FROM video_generations
 UNION ALL SELECT id,user_id,'extract',status,created_at FROM asset_extractions`
const analyticsCalls = `SELECT 'text:'||id id,user_id,model,CASE WHEN COALESCE(error_message,'')='' THEN 1 ELSE 0 END success,latency_ms,created_at FROM model_calls
 UNION ALL SELECT 'image:'||p.id,p.user_id,p.model,CASE WHEN p.status='completed' THEN 1 ELSE 0 END,0,p.created_at FROM image_provider_jobs p
 WHERE NOT EXISTS(SELECT 1 FROM model_calls m WHERE m.user_id=p.user_id AND m.ref_id=p.id AND m.model=p.model)
 UNION ALL SELECT 'video:'||v.id,v.user_id,v.model,CASE WHEN v.status IN ('completed','succeeded','ready') THEN 1 ELSE 0 END,0,v.created_at FROM video_generations v
 WHERE NOT EXISTS(SELECT 1 FROM model_calls m WHERE m.user_id=v.user_id AND m.ref_id=v.id AND m.model=v.model)`

func (s *Store) UserAnalytics(userID string, days int, now time.Time) (AnalyticsSummary, error) {
	if userID == "" {
		return AnalyticsSummary{}, errors.New("账号不能为空")
	}
	return s.analyticsSummary(userID, days, now)
}

// OperationsAnalytics aggregates customer usage; only administrator routes expose it.
func (s *Store) OperationsAnalytics(days int, now time.Time) (AnalyticsSummary, error) {
	return s.analyticsSummary("", days, now)
}

func (s *Store) analyticsSummary(userID string, days int, now time.Time) (AnalyticsSummary, error) {
	if days != 7 && days != 30 && days != 90 {
		return AnalyticsSummary{}, errors.New("请选择 7、30 或 90 天")
	}
	loc := time.FixedZone("Asia/Shanghai", 8*3600)
	now = now.In(loc)
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
	start := end.AddDate(0, 0, -days)
	from, to := start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339)
	a := AnalyticsSummary{Days: days, Timezone: "Asia/Shanghai", Timeline: []AnalyticsDay{}, Features: []AnalyticsCount{}, Entries: []AnalyticsCount{}, Models: []AnalyticsModel{}, Parameters: []AnalyticsCount{}, Journey: []AnalyticsCount{}}
	a.Scope = "user"
	scope := "user_id=?"
	args := []any{userID, from, to}
	if userID == "" {
		a.Scope = "platform"
		scope = "user_id IN (SELECT id FROM users WHERE role<>'admin' AND email NOT LIKE '%.test')"
		args = []any{from, to}
	}
	eventScope := scope
	if a.Scope == "platform" {
		eventScope += " AND environment='production'"
	}
	if err := s.db.QueryRow(`SELECT value FROM analytics_config WHERE key='started_at'`).Scan(&a.StartedAt); err != nil {
		return a, err
	}
	byDay := map[string]*AnalyticsDay{}
	for i := 0; i < days; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		a.Timeline = append(a.Timeline, AnalyticsDay{Date: date})
	}
	for i := range a.Timeline {
		byDay[a.Timeline[i].Date] = &a.Timeline[i]
	}
	rows, err := s.db.Query(`SELECT strftime('%Y-%m-%d',created_at,'+8 hours'),SUM(name='page_view'),SUM(effective),SUM(name='result_preview'),SUM(name='download_served'),SUM(name='asset_imported'),SUM(name='task_accepted'),SUM(name='request_failed'),COUNT(DISTINCT CASE WHEN effective=1 THEN user_id END) FROM analytics_events WHERE `+eventScope+` AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?) GROUP BY 1`, args...)
	if err != nil {
		return a, err
	}
	for rows.Next() {
		var date string
		var visits, actions, previews, downloads, reused, requests, fail, activeUsers int
		if err = rows.Scan(&date, &visits, &actions, &previews, &downloads, &reused, &requests, &fail, &activeUsers); err != nil {
			rows.Close()
			return a, err
		}
		if d := byDay[date]; d != nil {
			d.Visits = visits
			d.Actions = actions
			d.ActiveUsers = activeUsers
		}
		a.Visits += visits
		if actions > 0 {
			a.ActiveDays++
		}
		a.Previews += previews
		a.Downloads += downloads
		a.Reused += reused
		a.Requests += requests
		a.Errors += fail
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return a, err
	}
	rows, err = s.db.Query(`WITH tasks AS (`+analyticsTasks+`) SELECT strftime('%Y-%m-%d',created_at,'+8 hours'),COUNT(*),SUM(status IN ('ready','completed','succeeded')),SUM(status IN ('failed','cancelled','canceled')) FROM tasks WHERE `+scope+` AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?) GROUP BY 1`, args...)
	if err != nil {
		return a, err
	}
	for rows.Next() {
		var date string
		var n, success, failed int
		if err = rows.Scan(&date, &n, &success, &failed); err != nil {
			rows.Close()
			return a, err
		}
		if d := byDay[date]; d != nil {
			d.Tasks = n
			d.Completed = success
		}
		a.Tasks += n
		a.Completed += success
		a.Failed += failed
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return a, err
	}
	a.Pending = a.Tasks - a.Completed - a.Failed
	count := func(q string, args ...any) ([]AnalyticsCount, error) {
		out := []AnalyticsCount{}
		r, e := s.db.Query(q, args...)
		if e != nil {
			return out, e
		}
		defer r.Close()
		for r.Next() {
			var x AnalyticsCount
			if e = r.Scan(&x.Key, &x.Count); e != nil {
				return out, e
			}
			out = append(out, x)
		}
		return out, r.Err()
	}
	a.Features, err = count(`WITH tasks AS (`+analyticsTasks+`) SELECT feature,COUNT(*) FROM tasks WHERE `+scope+` AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?) GROUP BY feature ORDER BY COUNT(*) DESC`, args...)
	if err != nil {
		return a, err
	}
	a.FeatureRequests, err = count(`SELECT feature,COUNT(*) FROM analytics_events WHERE `+eventScope+` AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?) AND name='task_accepted' GROUP BY feature ORDER BY COUNT(*) DESC`, args...)
	if err != nil {
		return a, err
	}
	a.FeatureOpens, err = count(`SELECT feature,COUNT(*) FROM analytics_events WHERE `+eventScope+` AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?) AND name='feature_open' GROUP BY feature ORDER BY COUNT(*) DESC`, args...)
	if err != nil {
		return a, err
	}
	a.Entries, err = count(`SELECT entry,COUNT(*) FROM analytics_events WHERE `+eventScope+` AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?) AND name='task_accepted' GROUP BY entry ORDER BY COUNT(*) DESC`, args...)
	if err != nil {
		return a, err
	}
	a.Journey, err = count(`SELECT name,COUNT(DISTINCT user_id||':'||session_id) FROM analytics_events WHERE `+eventScope+` AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?) AND session_id<>'' AND name IN ('page_view','material_saved','task_accepted','result_preview','download_served','asset_imported') GROUP BY name`, args...)
	if err != nil {
		return a, err
	}
	rows, err = s.db.Query(`WITH calls AS (`+analyticsCalls+`) SELECT COALESCE(NULLIF(model,''),'未记录型号'),COUNT(*),SUM(success),COALESCE(AVG(NULLIF(latency_ms,0)),0) FROM calls WHERE `+scope+` AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?) GROUP BY model ORDER BY COUNT(*) DESC`, args...)
	if err != nil {
		return a, err
	}
	for rows.Next() {
		var x AnalyticsModel
		var avg float64
		if err = rows.Scan(&x.Model, &x.Count, &x.Success, &avg); err != nil {
			rows.Close()
			return a, err
		}
		x.Latency = int64(avg)
		a.Models = append(a.Models, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return a, err
	}
	a.Parameters, err = count(`SELECT ratio||' · '||resolution||' · '||duration||'秒',COUNT(*) FROM video_generations WHERE `+scope+` AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?) GROUP BY ratio,resolution,duration ORDER BY COUNT(*) DESC LIMIT 12`, args...)
	if err != nil {
		return a, err
	}
	rows, err = s.db.Query(`SELECT latency_ms FROM model_calls WHERE `+scope+` AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?) AND latency_ms>0`, args...)
	if err != nil {
		return a, err
	}
	latencies := []int64{}
	for rows.Next() {
		var n int64
		if err = rows.Scan(&n); err != nil {
			rows.Close()
			return a, err
		}
		latencies = append(latencies, n)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return a, err
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	a.LatencySamples = len(latencies)
	if len(latencies) > 0 {
		a.LatencyP50 = latencies[(len(latencies)-1)*50/100]
		a.LatencyP95 = latencies[(len(latencies)*95+99)/100-1]
	}
	err = s.db.QueryRow(`SELECT COALESCE(SUM(points),0) FROM credit_charges WHERE `+scope+` AND status='consumed' AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?)`, args...).Scan(&a.Credits)
	if err != nil {
		return a, err
	}
	err = s.db.QueryRow(`SELECT COUNT(DISTINCT user_id) FROM analytics_events WHERE `+eventScope+` AND effective=1 AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?)`, args...).Scan(&a.ActiveUsers)
	if err != nil {
		return a, err
	}
	if a.Scope == "platform" {
		a.Activity, err = s.AnalyticsActivity(now)
	}
	return a, err
}

// Global activity is an operations metric, never exposed to ordinary users.
func (s *Store) AnalyticsActivity(now time.Time) (map[string]int, error) {
	result := map[string]int{}
	for key, days := range map[string]int{"dau": 1, "wau": 7, "mau": 30} {
		loc := time.FixedZone("Asia/Shanghai", 8*3600)
		d := now.In(loc)
		end := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
		var n int
		err := s.db.QueryRow(`SELECT COUNT(DISTINCT e.user_id) FROM analytics_events e JOIN users u ON u.id=e.user_id WHERE e.environment='production' AND e.effective=1 AND u.role<>'admin' AND u.email NOT LIKE '%.test' AND julianday(e.created_at)>=julianday(?) AND julianday(e.created_at)<julianday(?)`, end.AddDate(0, 0, -days).UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339)).Scan(&n)
		if err != nil {
			return nil, err
		}
		result[key] = n
	}

	if result["mau"] > 0 {
		result["stickiness_bps"] = result["dau"] * 10000 / result["mau"]
	}
	// Only matured first-effective-activity cohorts enter retention denominators.
	for _, day := range []int{1, 7, 30} {
		var eligible, returned int
		err := s.db.QueryRow(`WITH firsts AS (
   SELECT e.user_id,MIN(e.created_at) first_at FROM analytics_events e JOIN users u ON u.id=e.user_id
   WHERE e.environment='production' AND e.effective=1 AND u.role<>'admin' AND u.email NOT LIKE '%.test' GROUP BY e.user_id
  ) SELECT COUNT(*),COALESCE(SUM(EXISTS(SELECT 1 FROM analytics_events r WHERE r.user_id=f.user_id AND r.environment='production' AND r.effective=1 AND date(r.created_at,'+8 hours')=date(f.first_at,'+8 hours',?))),0)
  FROM firsts f WHERE date(f.first_at,'+8 hours',?)<=date(?,'+8 hours')`, fmt.Sprintf("+%d days", day), fmt.Sprintf("+%d days", day), now.UTC().Format(time.RFC3339)).Scan(&eligible, &returned)
		if err != nil {
			return nil, err
		}
		prefix := fmt.Sprintf("retention_d%d_", day)
		result[prefix+"eligible"] = eligible
		result[prefix+"returned"] = returned
		if eligible > 0 {
			result[prefix+"bps"] = returned * 10000 / eligible
		}
	}
	return result, nil
}
