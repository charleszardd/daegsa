package auth_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/charleszardd/daegsa/internal/auth"
)

func TestVUJarManager_Isolation(t *testing.T) {
	mgr, err := auth.NewVUJarManager(true, 3)
	if err != nil {
		t.Fatalf("failed to create VUJarManager: %v", err)
	}

	if !mgr.Enabled() {
		t.Errorf("expected manager to be enabled")
	}

	targetURL, _ := url.Parse("http://example.com/api")

	jar0 := mgr.GetJar(0)
	jar1 := mgr.GetJar(1)
	jar2 := mgr.GetJar(2)

	if jar0 == nil || jar1 == nil || jar2 == nil {
		t.Fatalf("expected non-nil jars for preallocated VUs")
	}

	// VU 0 receives cookie session=A
	jar0.SetCookies(targetURL, []*http.Cookie{
		{Name: "session", Value: "session_A", Path: "/"},
	})

	// VU 1 receives cookie session=B
	jar1.SetCookies(targetURL, []*http.Cookie{
		{Name: "session", Value: "session_B", Path: "/"},
	})

	// Verify VU 0 jar only has session_A
	cookies0 := jar0.Cookies(targetURL)
	if len(cookies0) != 1 || cookies0[0].Value != "session_A" {
		t.Errorf("VU 0 jar corrupted, got: %v", cookies0)
	}

	// Verify VU 1 jar only has session_B
	cookies1 := jar1.Cookies(targetURL)
	if len(cookies1) != 1 || cookies1[0].Value != "session_B" {
		t.Errorf("VU 1 jar corrupted, got: %v", cookies1)
	}

	// Verify VU 2 jar has no cookies
	cookies2 := jar2.Cookies(targetURL)
	if len(cookies2) != 0 {
		t.Errorf("VU 2 jar should have 0 cookies, got: %v", cookies2)
	}
}

func TestVUJarManager_SessionPersistence(t *testing.T) {
	mgr, err := auth.NewVUJarManager(true, 2)
	if err != nil {
		t.Fatalf("failed to create VUJarManager: %v", err)
	}

	targetURL, _ := url.Parse("https://app.internal/dashboard")
	jar := mgr.GetJar(0)

	// Step 1: Initial cookie set
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "auth_token", Value: "token_xyz", Path: "/"},
	})

	// Step 2: Second iteration cookie set (adds another cookie)
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "user_role", Value: "admin", Path: "/"},
	})

	cookies := jar.Cookies(targetURL)
	if len(cookies) != 2 {
		t.Fatalf("expected 2 persisted cookies, got %d", len(cookies))
	}

	cookieMap := make(map[string]string)
	for _, c := range cookies {
		cookieMap[c.Name] = c.Value
	}

	if cookieMap["auth_token"] != "token_xyz" {
		t.Errorf("expected auth_token='token_xyz', got %q", cookieMap["auth_token"])
	}
	if cookieMap["user_role"] != "admin" {
		t.Errorf("expected user_role='admin', got %q", cookieMap["user_role"])
	}
}

func TestVUJarManager_DisabledAndEdgeCases(t *testing.T) {
	// Disabled manager
	disabledMgr, err := auth.NewVUJarManager(false, 5)
	if err != nil {
		t.Fatalf("failed to create disabled VUJarManager: %v", err)
	}
	if disabledMgr.Enabled() {
		t.Errorf("expected disabled manager to report Enabled() == false")
	}
	if disabledMgr.GetJar(0) != nil {
		t.Errorf("expected nil jar from disabled manager")
	}

	// Negative index
	enabledMgr, _ := auth.NewVUJarManager(true, 2)
	if enabledMgr.GetJar(-1) != nil {
		t.Errorf("expected nil jar for negative VU ID")
	}

	// IDs outside the configured scheduler capacity must not allocate state.
	if enabledMgr.GetJar(10) != nil {
		t.Errorf("expected nil jar for out-of-range VU ID")
	}
}
