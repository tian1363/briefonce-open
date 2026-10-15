package charactergen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

type Task struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	URL      string `json:"url"`
	ImageURL string `json:"image_url"`
	Metadata struct {
		ImageURL string `json:"image_url"`
		Image    string `json:"image"`
	} `json:"metadata"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (t Task) ResultURL() string {
	for _, s := range []string{t.URL, t.ImageURL, t.Metadata.ImageURL, t.Metadata.Image} {
		if s != "" {
			return s
		}
	}
	return ""
}

type CallResult struct {
	Task       Task
	HTTPStatus int
	RequestID  string
}

func (c *Client) ZexRequest(ctx context.Context, endpoint, key, taskID string, body []byte) (CallResult, error) {
	method := http.MethodPost
	if taskID != "" {
		method = http.MethodGet
		endpoint += "/" + url.PathEscape(taskID)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return CallResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return CallResult{}, errors.New("供应商连接失败，任务状态待确认")
	}
	defer res.Body.Close()
	result := CallResult{HTTPStatus: res.StatusCode, RequestID: res.Header.Get("X-Request-Id")}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return result, errors.New("供应商响应读取失败")
	}
	if res.StatusCode/100 != 2 {
		switch res.StatusCode {
		case 401, 403:
			return result, ErrAccessDenied
		case 429:
			return result, ErrRateLimited
		case 404:
			return result, ErrModelUnavailable
		}
		return result, errors.New("供应商请求未完成，请核对任务")
	}
	if json.Unmarshal(raw, &result.Task) != nil {
		return result, errors.New("供应商响应格式无效，任务状态待确认")
	}
	return result, nil
}

// Downloads use a separate client without credentials, proxies or private-network access.
func PublicImageURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return errors.New("图片下载地址无效")
	}
	return nil
}
func SafeDownloadClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !ip.IP.IsGlobalUnicast() || ip.IP.IsPrivate() || ip.IP.IsLoopback() || ip.IP.IsLinkLocalUnicast() {
				return nil, errors.New("图片地址不可指向内网")
			}
		}
		if len(ips) == 0 {
			return nil, errors.New("图片域名无法解析")
		}
		return (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
	return &http.Client{Timeout: 90 * time.Second, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return errors.New("图片重定向过多")
		}
		return PublicImageURL(req.URL.String())
	}}
}
func DownloadZexImage(ctx context.Context, raw string) ([]byte, error) {
	if err := PublicImageURL(raw); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	res, err := SafeDownloadClient().Do(req)
	if err != nil {
		return nil, errors.New("图片生成完成，下载暂时失败")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("图片生成完成，下载暂时失败")
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, (32<<20)+1))
	if err != nil || len(b) == 0 || len(b) > 32<<20 {
		return nil, errors.New("图片下载不完整或超过 32MB")
	}
	return b, nil
}
