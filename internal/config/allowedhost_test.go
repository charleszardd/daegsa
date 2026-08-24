package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeAllowedHost(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		allowWildcard bool
		want          string
	}{
		{name: "DNS lowercase and terminal dot", input: " API.Example.COM. ", want: "api.example.com"},
		{name: "IPv4", input: "127.0.0.1", want: "127.0.0.1"},
		{name: "IPv6 canonical", input: "0:0:0:0:0:0:0:1", want: "::1"},
		{name: "declarative wildcard", input: "*.Example.COM.", allowWildcard: true, want: "*.example.com"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := NormalizeAllowedHost(testCase.input, testCase.allowWildcard)
			if err != nil {
				t.Fatalf("NormalizeAllowedHost() error = %v", err)
			}
			if got != testCase.want {
				t.Fatalf("NormalizeAllowedHost() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestNormalizeAllowedHostRejectsNonHostInputs(t *testing.T) {
	invalidHosts := []string{
		"",
		"https://api.example.com",
		"user@api.example.com",
		"api.example.com:443",
		"api.example.com/items",
		"api.example.com?region=us",
		"api.example.com#fragment",
		"bad_label.example.com",
		"-bad.example.com",
		"bad-.example.com",
		"api..example.com",
		"*.example.com",
	}

	for _, invalidHost := range invalidHosts {
		t.Run(strings.ReplaceAll(invalidHost, "/", "_"), func(t *testing.T) {
			if _, err := NormalizeAllowedHost(invalidHost, false); err == nil {
				t.Fatalf("NormalizeAllowedHost(%q) unexpectedly succeeded", invalidHost)
			}
		})
	}
}

func TestNormalizeAllowedHostsDeduplicatesCanonicalEntries(t *testing.T) {
	got, err := NormalizeAllowedHosts([]string{"API.EXAMPLE.COM", "api.example.com.", "127.0.0.1"}, false)
	if err != nil {
		t.Fatalf("NormalizeAllowedHosts() error = %v", err)
	}
	want := []string{"api.example.com", "127.0.0.1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeAllowedHosts() = %v, want %v", got, want)
	}
}

func TestIsLoopbackHost(t *testing.T) {
	tests := map[string]bool{
		"localhost":       true,
		"LOCALHOST.":      true,
		"127.0.0.1":       true,
		"127.255.255.254": true,
		"::1":             true,
		"api.example.com": false,
		"192.168.1.10":    false,
	}
	for host, want := range tests {
		if got := IsLoopbackHost(host); got != want {
			t.Errorf("IsLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}
