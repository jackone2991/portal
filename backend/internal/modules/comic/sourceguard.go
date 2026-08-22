package comic

// SSRF guard for external-source sync (P1.8).
//
// A sync source URL is supplied by any authenticated comic owner — and migration
// 0025 grants comics:write:own to the base `user` role, so that is effectively
// anyone who can register — then later fetched by a real Chrome running inside the
// compose `internal` network. An unrestricted URL is therefore a direct path to
// api:8080, minio:9000, dragonfly:6379, the host Postgres via host.docker.internal,
// and the cloud metadata endpoint on a VPS.
//
// Two layers, both enforced at source-creation time AND again immediately before
// each scrape (DNS can change in between — classic rebinding):
//
//  1. an optional host allowlist (COMIC_SOURCE_ALLOWLIST), and
//  2. an always-on rejection of every non-public IP the host resolves to.
//
// Layer 2 runs even when the allowlist is empty, so a deployment that has not
// configured an allowlist is still protected against internal targets.

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// lookupIP is a package var so tests can resolve without touching the network.
var lookupIP = func(ctx context.Context, host string) ([]net.IP, error) {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		ips = append(ips, a.IP)
	}
	return ips, nil
}

// ParseSourceAllowlist turns a comma-separated env value into allowlist entries.
// An empty result disables the allowlist layer (the IP check still applies).
func ParseSourceAllowlist(raw string) []string {
	out := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		if h := strings.ToLower(strings.TrimSpace(part)); h != "" {
			out = append(out, strings.TrimSuffix(h, "."))
		}
	}
	return out
}

// hostAllowed matches a host against the allowlist exactly or as a subdomain.
// An empty allowlist allows any host — layer 2 is what keeps that safe.
func hostAllowed(host string, allow []string) bool {
	if len(allow) == 0 {
		return true
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, a := range allow {
		if host == a || strings.HasSuffix(host, "."+a) {
			return true
		}
	}
	return false
}

// isPublicIP reports whether ip is safe to fetch — i.e. not pointing back at our
// own infrastructure. Everything not provably public is rejected (fail-closed).
func isPublicIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return false
	}
	// 100.64.0.0/10 (CGNAT) is not covered by IsPrivate but commonly routes to
	// provider infrastructure. IPv4-mapped IPv6 lands here too, via To4().
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1]&0xc0 == 64 {
		return false
	}
	return true
}

// validateSourceURL enforces both layers and returns the parsed URL.
func validateSourceURL(ctx context.Context, raw string, allow []string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%w: source_url phải là http(s) hợp lệ", ErrValidation)
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("%w: source_url thiếu hostname", ErrValidation)
	}
	if !hostAllowed(host, allow) {
		return nil, fmt.Errorf("%w: nguồn %q không nằm trong danh sách cho phép", ErrValidation, host)
	}
	// A literal IP never hits DNS, but still has to be public.
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return nil, fmt.Errorf("%w: source_url trỏ tới địa chỉ nội bộ", ErrValidation)
		}
		return u, nil
	}
	ips, err := lookupIP(ctx, host)
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("%w: không phân giải được host %q", ErrValidation, host)
	}
	// Every answer must be public: one internal A record is enough to pivot.
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return nil, fmt.Errorf("%w: host %q phân giải ra địa chỉ nội bộ (%s)", ErrValidation, host, ip)
		}
	}
	return u, nil
}
