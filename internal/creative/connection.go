package creative

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Each configured endpoint must belong to exactly one product user. Cookies stay
// inside that user's collector; neither cookies nor access tokens reach the UI.
type Collector struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}
type Connection struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	Username string `json:"username,omitempty"`
	QRCode   string `json:"qr_code,omitempty"`
	Timeout  string `json:"timeout,omitempty"`
}
type ownerKey struct{}

func WithCollectorOwner(ctx context.Context, owner string) context.Context {
	return context.WithValue(ctx, ownerKey{}, owner)
}
func collectorFor(ctx context.Context) (Collector, bool) {
	owner, _ := ctx.Value(ownerKey{}).(string)
	path := os.Getenv("SCRIPT_AGENT_XHS_COLLECTORS_FILE")
	if owner == "" || path == "" {
		return Collector{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Collector{}, false
	}
	var all map[string]Collector
	if json.Unmarshal(data, &all) != nil {
		return Collector{}, false
	}
	c, ok := all[owner]
	if !ok {
		return Collector{}, false
	}
	u, err := url.Parse(c.URL)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "http" && u.Scheme != "https") {
		return Collector{}, false
	}
	if u.Scheme == "http" && u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1" {
		return Collector{}, false
	}
	for id, other := range all {
		if id != owner && strings.TrimRight(other.URL, "/") == strings.TrimRight(c.URL, "/") {
			return Collector{}, false
		}
	}
	c.URL = strings.TrimRight(c.URL, "/")
	return c, true
}
func (c Collector) call(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.URL+"/api/v1"+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("collector redirect refused") }}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("连接助手不可用，请检查是否已启动")
	}
	defer resp.Body.Close()
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   string          `json:"error"`
		Details json.RawMessage `json:"details"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&envelope) != nil {
		return errors.New("连接助手响应异常，请重试")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !envelope.Success {
		// Raw browser errors may contain tokens/paths. Only return a controlled message.
		detail := strings.ToLower(envelope.Error + string(envelope.Details))
		if strings.Contains(detail, "登录") || strings.Contains(detail, "login") {
			return errors.New("登录已失效，请重新连接小红书")
		}
		if strings.Contains(detail, "验证码") || strings.Contains(detail, "captcha") {
			return errors.New("需要完成小红书验证，请在连接助手窗口完成后重试")
		}
		return errors.New("暂时无法读取，请检查链接、网络或小红书访问限制后重试")
	}
	if output == nil {
		return nil
	}
	return json.Unmarshal(envelope.Data, output)
}
func ConnectionStatus(ctx context.Context) Connection {
	c, ok := collectorFor(ctx)
	if !ok {
		return Connection{Status: "not_configured", Message: "当前账号尚未配置独立的小红书连接助手"}
	}
	var status struct {
		LoggedIn bool   `json:"is_logged_in"`
		Username string `json:"username"`
	}
	if err := c.call(ctx, "GET", "/login/status", nil, &status); err != nil {
		return Connection{Status: "unavailable", Message: err.Error()}
	}
	if !status.LoggedIn {
		return Connection{Status: "login_required", Message: "未登录或登录已失效，请连接小红书"}
	}
	return Connection{Status: "connected", Message: "已连接小红书", Username: status.Username}
}
func Connect(ctx context.Context) Connection {
	c, ok := collectorFor(ctx)
	if !ok {
		return ConnectionStatus(ctx)
	}
	var qr struct {
		LoggedIn bool   `json:"is_logged_in"`
		Img      string `json:"img"`
		Timeout  string `json:"timeout"`
	}
	if err := c.call(ctx, "GET", "/login/qrcode", nil, &qr); err != nil {
		return Connection{Status: "unavailable", Message: err.Error()}
	}
	if qr.LoggedIn {
		return ConnectionStatus(ctx)
	}
	// Only image data URLs can be rendered as login QR codes.
	if !strings.HasPrefix(qr.Img, "data:image/png;base64,") && !strings.HasPrefix(qr.Img, "data:image/jpeg;base64,") {
		return Connection{Status: "unavailable", Message: "登录二维码未能加载，请重新获取"}
	}
	return Connection{Status: "awaiting_login", Message: "请使用小红书扫码登录；完成后将继续读取", QRCode: qr.Img, Timeout: qr.Timeout}
}
func Disconnect(ctx context.Context) error {
	c, ok := collectorFor(ctx)
	if !ok {
		return errors.New("当前账号尚未配置连接助手")
	}
	return c.call(ctx, "DELETE", "/login/cookies", nil, nil)
}
func ReadNote(ctx context.Context, raw string) XiaohongshuSnapshot {
	link, heading := parseXiaohongshuInput(raw)
	snapshot := XiaohongshuSnapshot{RequestedURL: link, ShareHeading: heading, FetchedAt: time.Now().UTC().Format(time.RFC3339)}
	if err := validateXiaohongshuURL(ctx, link); err != nil {
		snapshot.Status = "invalid"
		snapshot.Warning = "请输入有效的小红书笔记链接"
		return snapshot
	}
	status := ConnectionStatus(ctx)
	if status.Status != "connected" {
		snapshot.Status = status.Status
		snapshot.Warning = status.Message
		return snapshot
	}
	u, _ := url.Parse(link)
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	id := parts[len(parts)-1]
	if len(id) != 24 || u.Query().Get("xsec_token") == "" {
		snapshot.Status = "invalid"
		snapshot.Warning = "请从小红书重新复制完整分享链接（含访问参数），暂不支持短链接"
		return snapshot
	}
	c, _ := collectorFor(ctx)
	var result struct {
		Data struct {
			Note struct {
				ID        string `json:"noteId"`
				Title     string `json:"title"`
				Desc      string `json:"desc"`
				ImageList []struct {
					Default string `json:"urlDefault"`
					Preview string `json:"urlPre"`
				} `json:"imageList"`
			} `json:"note"`
		} `json:"data"`
	}
	err := c.call(ctx, "POST", "/feeds/detail", map[string]any{"feed_id": id, "xsec_token": u.Query().Get("xsec_token"), "load_all_comments": false}, &result)
	if err != nil {
		snapshot.Status = "unavailable"
		if strings.Contains(err.Error(), "登录已失效") {
			snapshot.Status = "login_required"
		}
		snapshot.Warning = err.Error()
		return snapshot
	}
	n := result.Data.Note
	if n.ID != id || (n.Title == "" && n.Desc == "") {
		snapshot.Status = "unavailable"
		snapshot.Warning = "未取得笔记正文，请重试或补充内容"
		return snapshot
	}
	snapshot.Title = n.Title
	snapshot.Description = n.Desc
	snapshot.PageText = n.Desc
	for _, img := range n.ImageList {
		s := img.Default
		if s == "" {
			s = img.Preview
		}
		if strings.HasPrefix(s, "http://") {
			s = "https://" + strings.TrimPrefix(s, "http://")
		}
		if s != "" {
			snapshot.Images = append(snapshot.Images, s)
		}
	}
	snapshot.Status = "parsed"
	snapshot.Warning = "已读取笔记文本与图片地址；视频与完整评论尚未解析"
	return snapshot
}
