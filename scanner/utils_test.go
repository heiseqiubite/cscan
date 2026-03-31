package scanner

import (
	"errors"
	"net"
	"testing"
)

type testResolver struct {
	ips map[string][]net.IP
	err map[string]error
}

func (r *testResolver) resolve(domain string) ([]net.IP, error) {
	if err, ok := r.err[domain]; ok {
		return nil, err
	}
	if ips, ok := r.ips[domain]; ok {
		return ips, nil
	}
	return nil, nil
}

func TestGenerateAssetsFromTargetsWithResolver_PureDomainFillsIP(t *testing.T) {
	resolver := &testResolver{
		ips: map[string][]net.IP{
			"example.com": {net.ParseIP("1.2.3.4"), net.ParseIP("2001:db8::1")},
		},
	}

	assets := generateAssetsFromTargetsWithResolver("example.com", resolver)
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}

	asset := assets[0]
	if asset.Host != "example.com" {
		t.Fatalf("expected host example.com, got %s", asset.Host)
	}
	if asset.Category != "domain" {
		t.Fatalf("expected category domain, got %s", asset.Category)
	}
	if len(asset.IPV4) != 1 || asset.IPV4[0].IP != "1.2.3.4" {
		t.Fatalf("expected ipv4 to be filled, got %+v", asset.IPV4)
	}
	if len(asset.IPV6) != 1 || asset.IPV6[0].IP != "2001:db8::1" {
		t.Fatalf("expected ipv6 to be filled, got %+v", asset.IPV6)
	}
}

func TestGenerateAssetsFromTargetsWithResolver_DomainWithSchemeFillsIP(t *testing.T) {
	resolver := &testResolver{
		ips: map[string][]net.IP{
			"example.com": {net.ParseIP("5.6.7.8")},
		},
	}

	assets := generateAssetsFromTargetsWithResolver("https://example.com/login", resolver)
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}

	asset := assets[0]
	if asset.Port != 443 {
		t.Fatalf("expected port 443, got %d", asset.Port)
	}
	if asset.Service != "https" {
		t.Fatalf("expected service https, got %s", asset.Service)
	}
	if asset.Path != "/login" {
		t.Fatalf("expected path /login, got %s", asset.Path)
	}
	if len(asset.IPV4) != 1 || asset.IPV4[0].IP != "5.6.7.8" {
		t.Fatalf("expected ipv4 to be filled, got %+v", asset.IPV4)
	}
}

func TestGenerateAssetsFromTargetsWithResolver_DNSFailureKeepsAsset(t *testing.T) {
	resolver := &testResolver{
		err: map[string]error{
			"example.com": errors.New("lookup failed"),
		},
	}

	assets := generateAssetsFromTargetsWithResolver("example.com", resolver)
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}

	asset := assets[0]
	if asset.Host != "example.com" {
		t.Fatalf("expected host example.com, got %s", asset.Host)
	}
	if len(asset.IPV4) != 0 || len(asset.IPV6) != 0 {
		t.Fatalf("expected empty ip lists on dns failure, got ipv4=%+v ipv6=%+v", asset.IPV4, asset.IPV6)
	}
}

func TestGenerateAssetsFromTargetsWithResolver_IPTargetUnchanged(t *testing.T) {
	resolver := &testResolver{}

	assets := generateAssetsFromTargetsWithResolver("1.1.1.1", resolver)
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}

	asset := assets[0]
	if asset.Category != "ipv4" {
		t.Fatalf("expected category ipv4, got %s", asset.Category)
	}
	if len(asset.IPV4) != 0 || len(asset.IPV6) != 0 {
		t.Fatalf("expected no dns enrichment for ip target, got ipv4=%+v ipv6=%+v", asset.IPV4, asset.IPV6)
	}
}
