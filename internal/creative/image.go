package creative

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxNoteImageBytes = 20 << 20

func validateNoteImageURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return errors.New("图片地址无效")
	}
	host := strings.ToLower(u.Hostname())
	if host != "xhscdn.com" && !strings.HasSuffix(host, ".xhscdn.com") {
		return errors.New("仅支持小红书图片地址")
	}
	return nil
}

// FetchNoteImage serves a bounded raster from the note CDN through our own
// authenticated origin. It does not forward browser cookies or credentials.
func FetchNoteImage(ctx context.Context, raw string) ([]byte, string, error) {
	client := &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: nil, DialContext: safeDialContext}, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("图片重定向次数过多")
		}
		return validateNoteImageURL(req.URL.String())
	}}
	return fetchNoteImage(ctx, raw, client)
}
func fetchNoteImage(ctx context.Context, raw string, client *http.Client) ([]byte, string, error) {
	if err := validateNoteImageURL(raw); err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://www.xiaohongshu.com/")
	req.Header.Set("Accept", "image/webp,image/png,image/jpeg,image/gif")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", errors.New("图片暂时无法加载，请重试")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", errors.New("图片链接可能已失效，请重新读取笔记")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxNoteImageBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxNoteImageBytes {
		return nil, "", errors.New("图片内容无效或超过20MB")
	}
	kind := http.DetectContentType(data)
	switch kind {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
	default:
		return nil, "", errors.New("返回内容不是支持的图片")
	}
	return data, kind, nil
}
