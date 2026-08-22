package comic

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

// stubDNS points every lookup at ip for the duration of the test.
func stubDNS(t *testing.T, ip string) {
	t.Helper()
	prev := lookupIP
	lookupIP = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP(ip)}, nil
	}
	t.Cleanup(func() { lookupIP = prev })
}

func TestIsPublicIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1",       // loopback
		"10.0.0.5",        // RFC1918
		"172.16.0.1",      // RFC1918
		"192.168.1.1",     // RFC1918
		"169.254.169.254", // cloud metadata
		"0.0.0.0",         // unspecified
		"224.0.0.1",       // multicast
		"100.64.0.1",      // CGNAT
		"100.127.255.255", // CGNAT upper bound
		"::1",             // IPv6 loopback
		"fc00::1",         // IPv6 unique-local
		"fe80::1",         // IPv6 link-local
	}
	for _, s := range blocked {
		if isPublicIP(net.ParseIP(s)) {
			t.Errorf("isPublicIP(%s) = true, want false", s)
		}
	}
	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:2800:220:1::1", "99.255.255.255", "101.0.0.1"}
	for _, s := range allowed {
		if !isPublicIP(net.ParseIP(s)) {
			t.Errorf("isPublicIP(%s) = false, want true", s)
		}
	}
}

func TestHostAllowed(t *testing.T) {
	allow := ParseSourceAllowlist(" TruyenQQko.com , example.org ")
	cases := map[string]bool{
		"truyenqqko.com":         true,
		"www.truyenqqko.com":     true,
		"TRUYENQQKO.COM":         true,
		"truyenqqko.com.":        true, // trailing dot is the same host
		"example.org":            true,
		"evil.com":               false,
		"nottruyenqqko.com":      false, // suffix match must respect the dot
		"truyenqqko.com.evil.io": false,
	}
	for host, want := range cases {
		if got := hostAllowed(host, allow); got != want {
			t.Errorf("hostAllowed(%q) = %v, want %v", host, got, want)
		}
	}
	// An empty allowlist disables the layer.
	if !hostAllowed("anything.test", nil) {
		t.Error("empty allowlist should allow any host")
	}
}

// The shipped default must accept both truyenqq mirrors: truyenqqko.com is what
// existing sources sync from, truyenqqno.com is what the UI placeholder suggests.
// Listing only one silently rejects everyone who follows the other.
func TestShippedAllowlistCoversBothMirrors(t *testing.T) {
	shipped := ParseSourceAllowlist("truyenqqko.com,truyenqqno.com")
	for _, host := range []string{"truyenqqko.com", "truyenqqno.com", "www.truyenqqno.com"} {
		if !hostAllowed(host, shipped) {
			t.Errorf("shipped allowlist rejects %q", host)
		}
	}
}

func TestValidateSourceURL_BlocksInternal(t *testing.T) {
	ctx := context.Background()
	// Literal internal IPs never reach DNS.
	for _, raw := range []string{
		"http://127.0.0.1:9000/",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]:8080/",
		"http://10.0.0.1/",
	} {
		if _, err := validateSourceURL(ctx, raw, nil); err == nil {
			t.Errorf("validateSourceURL(%q) = nil error, want block", raw)
		} else if !errors.Is(err, ErrValidation) {
			t.Errorf("validateSourceURL(%q) error = %v, want ErrValidation", raw, err)
		}
	}
	// A public-looking name that resolves internally is the rebinding case.
	stubDNS(t, "127.0.0.1")
	if _, err := validateSourceURL(ctx, "http://minio.attacker.test/", nil); err == nil {
		t.Error("host resolving to loopback should be blocked")
	}
}

func TestValidateSourceURL_SchemeAndAllowlist(t *testing.T) {
	ctx := context.Background()
	for _, raw := range []string{"file:///etc/passwd", "gopher://x/", "ftp://example.com/", "notaurl", ""} {
		if _, err := validateSourceURL(ctx, raw, nil); err == nil {
			t.Errorf("validateSourceURL(%q) = nil error, want block", raw)
		}
	}
	stubDNS(t, "93.184.216.34")
	allow := ParseSourceAllowlist("truyenqqko.com")
	if _, err := validateSourceURL(ctx, "https://evil.test/x", allow); err == nil {
		t.Error("host outside the allowlist should be blocked")
	} else if !strings.Contains(err.Error(), "danh sách cho phép") {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := validateSourceURL(ctx, "https://truyenqqko.com/truyen/abc", allow); err != nil {
		t.Errorf("allowlisted public host should pass, got %v", err)
	}
}
