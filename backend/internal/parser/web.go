package parser

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	readability "github.com/go-shiori/go-readability"
	"golang.org/x/net/html/charset"
)

const MaxWebSize int64 = 10 << 20

type Parser struct{ client *http.Client }

func New() *Parser {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// 直连已校验的地址，避免代理绕过地址限制与 DNS 重绑定。
	transport.Proxy = nil
	transport.DialContext = publicDial
	return &Parser{client: &http.Client{Timeout: 20 * time.Second, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("网页重定向次数过多")
		}
		return ValidateURL(req.URL.String())
	}}}
}

func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || len(raw) > 8192 {
		return fmt.Errorf("请提供有效的 HTTP 或 HTTPS 链接")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !publicIP(ip) {
		return fmt.Errorf("不支持访问本地或内网地址")
	}
	if strings.EqualFold(u.Hostname(), "localhost") || strings.HasSuffix(strings.ToLower(u.Hostname()), ".localhost") {
		return fmt.Errorf("不支持访问本地或内网地址")
	}
	return nil
}

func publicIP(ip net.IP) bool {
	shared := net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified() &&
		!shared.Contains(ip)
}

func publicDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("网页地址无效")
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("无法解析网页地址")
	}
	for _, ip := range ips {
		if !publicIP(ip.IP) {
			return nil, fmt.Errorf("不支持访问本地或内网地址")
		}
	}
	var dialer net.Dialer
	for _, ip := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err == nil {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("无法连接网页")
}

func IsVideoURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, domain := range []string{"youtube.com", "youtu.be", "bilibili.com", "b23.tv"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func (p *Parser) web(ctx context.Context, raw string) (Result, error) {
	if err := ValidateURL(raw); err != nil {
		return Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return Result{}, fmt.Errorf("网页地址无效")
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (compatible; KnowNote/1.0)")
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := p.client.Do(request)
	if err != nil {
		return Result{}, fmt.Errorf("网页抓取失败，请检查链接或稍后重试")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("网页返回异常状态码 %d", resp.StatusCode)
	}
	if resp.ContentLength > MaxWebSize {
		return Result{}, fmt.Errorf("网页超过 10MB 限制")
	}
	data, err := readLimited(resp.Body, MaxWebSize)
	if err != nil {
		return Result{}, err
	}
	reader, err := charset.NewReader(strings.NewReader(string(data)), resp.Header.Get("Content-Type"))
	if err != nil {
		return Result{}, fmt.Errorf("网页编码无法识别")
	}
	article, err := readability.FromReader(reader, resp.Request.URL)
	if err != nil {
		return Result{}, fmt.Errorf("无法提取网页正文")
	}
	return Result{Title: Clean(article.Title), Content: article.TextContent}, nil
}
