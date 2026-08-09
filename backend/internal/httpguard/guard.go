// Package httpguard 提供 SSRF（Server-Side Request Forgery）防护。
//
// 设计背景：canvas-api 携带用户凭据请求「用户自填的 baseUrl」，这是本次
// 生成编排下沉改造新增的攻击面。用户可把 baseUrl 设为 http://169.254.169.254/...
// 让服务端带 key 去打云元数据端点；上游返回的媒体 URL 同理。
//
// 防护策略：
//   - scheme 限 http / https
//   - DNS 解析后的 IP 落私有/回环/链路本地/唯一本地段 → 拒绝，除非命中白名单
//   - DialContext 二次校验：只在 ValidateURL 预检还不够——预检通过后如果 DNS
//     被重新解析到内网（DNS 重绑定），请求仍会打到内网。必须在实际建连时再校验一次。
//   - 开发环境的正常路径是 localhost（SUB2API_BASE_URL=http://localhost:8080），
//     所以「默认禁 + 白名单放行」，不能直接禁掉所有私有 IP。
package httpguard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Config 是 Guard 的配置。
type Config struct {
	// AllowedHosts 是允许访问的主机名或 IP（不含端口）白名单。
	// 空列表意味着所有私有/回环地址都被拒绝；公网地址不受此限制。
	// 典型开发配置：[]string{"localhost", "127.0.0.1"}
	AllowedHosts []string
}

// Guard 实施 SSRF 防护。
type Guard struct {
	allowed map[string]struct{}
}

// New 创建一个 Guard。
func New(cfg Config) *Guard {
	g := &Guard{allowed: make(map[string]struct{})}
	for _, h := range cfg.AllowedHosts {
		g.allowed[h] = struct{}{}
	}
	return g
}

// ValidateURL 检查 rawURL 是否允许作为上游目标。
// 返回 non-nil error 意味着请求应被拒绝。
func (g *Guard) ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("httpguard: malformed url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("httpguard: scheme %q is not allowed (only http/https)", u.Scheme)
	}
	hostname := u.Hostname()
	if g.isAllowed(hostname) {
		return nil
	}
	return g.checkHostname(hostname)
}

// HTTPClient 返回一个带 DialContext SSRF 守卫的 http.Client。
// 即使 ValidateURL 通过，建连时仍会对实际 IP 做二次校验，防 DNS 重绑定。
func (g *Guard) HTTPClient() *http.Client {
	transport := &http.Transport{
		DialContext: g.dialContext,
		// 复用默认值
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{Transport: transport}
}

// dialContext 是 http.Transport.DialContext 的拦截实现。
// 在实际建连时对解析出的 IP 再做一次 SSRF 校验（防止 DNS 重绑定）。
func (g *Guard) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("httpguard: split host:port %q: %w", addr, err)
	}

	// 解析实际 IP
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("httpguard: dns lookup %q: %w", host, err)
	}

	for _, ip := range ips {
		if g.isAllowed(ip.IP.String()) || g.isAllowed(host) {
			continue
		}
		if isPrivateIP(ip.IP) {
			return nil, fmt.Errorf("httpguard: target %s resolved to private address %s", host, ip.IP)
		}
	}

	dialer := &net.Dialer{}
	return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
}

// checkHostname 解析主机名并检查 IP 是否落在私有段。
func (g *Guard) checkHostname(hostname string) error {
	// 如果已经是 IP 字面量
	if ip := net.ParseIP(hostname); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("httpguard: ip %s is in a private/reserved range", ip)
		}
		return nil
	}
	// 域名解析
	ips, err := net.LookupHost(hostname)
	if err != nil {
		// DNS 查询失败时拒绝，不降级放行
		return fmt.Errorf("httpguard: dns lookup %q failed: %w", hostname, err)
	}
	for _, raw := range ips {
		ip := net.ParseIP(raw)
		if ip == nil {
			continue
		}
		if isPrivateIP(ip) {
			return fmt.Errorf("httpguard: hostname %q resolves to private address %s", hostname, ip)
		}
	}
	return nil
}

// isAllowed 检查 host（主机名或 IP，不含端口）是否在白名单中。
func (g *Guard) isAllowed(host string) bool {
	_, ok := g.allowed[host]
	return ok
}

// isPrivateIP 判断 IP 是否落在私有/保留/回环/链路本地/唯一本地段。
func isPrivateIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		// 127.0.0.0/8 回环
		if ip4[0] == 127 {
			return true
		}
		// 10.0.0.0/8
		if ip4[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip4[0] == 172 && ip4[1]&0xf0 == 16 {
			return true
		}
		// 192.168.0.0/16
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
		// 169.254.0.0/16 链路本地（云元数据端点）
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
		// 0.0.0.0/8
		if ip4[0] == 0 {
			return true
		}
		return false
	}
	// IPv6
	// ::1 回环
	if ip.Equal(net.IPv6loopback) {
		return true
	}
	// fc00::/7 唯一本地
	if len(ip) == 16 && ip[0]&0xfe == 0xfc {
		return true
	}
	// fe80::/10 链路本地
	if len(ip) == 16 && ip[0] == 0xfe && ip[1]&0xc0 == 0x80 {
		return true
	}
	return false
}
