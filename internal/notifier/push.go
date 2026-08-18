// Package notifier 负责向 Uptime Kuma Push 接口上报监控状态。
package notifier

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Push 向 Uptime Kuma 上报状态的客户端。
type Push struct {
	url       string
	httpClient *http.Client
}

// New 创建 Push 上报客户端。
func New(pushURL string, timeout time.Duration) (*Push, error) {
	if pushURL == "" {
		return nil, fmt.Errorf("push url 不能为空")
	}
	return &Push{
		url: pushURL,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

// Report 上报状态。
//   status: up 或 down
//   msg:    附加说明(可空)
//   ping:   本次检查耗时(毫秒), 用于展示
func (p *Push) Report(status, msg string, pingMs int64) error {
	u, err := url.Parse(p.url)
	if err != nil {
		return fmt.Errorf("解析 push url 失败: %w", err)
	}
	q := u.Query()
	q.Set("status", status)
	if msg != "" {
		q.Set("msg", msg)
	}
	if pingMs >= 0 {
		q.Set("ping", fmt.Sprintf("%d", pingMs))
	}
	u.RawQuery = q.Encode()

	resp, err := p.httpClient.Get(u.String())
	if err != nil {
		return fmt.Errorf("上报请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上报返回非200: HTTP %d", resp.StatusCode)
	}
	return nil
}
