package auth

import (
	"net/http"
	"net/http/cookiejar"
	"sync"
)

// VUJarManager manages isolated cookie jars per virtual user and worker lane (§8, §11).
type VUJarManager struct {
	enabled bool
	jars    []http.CookieJar
	mu      sync.RWMutex
	lazyMap map[int]http.CookieJar
}

// NewVUJarManager instantiates a VUJarManager. If enabled, it pre-allocates cookie jars for numVUs.
func NewVUJarManager(enabled bool, numVUs int) (*VUJarManager, error) {
	if !enabled {
		return &VUJarManager{enabled: false}, nil
	}

	m := &VUJarManager{
		enabled: true,
		lazyMap: make(map[int]http.CookieJar),
	}

	if numVUs > 0 {
		m.jars = make([]http.CookieJar, numVUs)
		for i := 0; i < numVUs; i++ {
			jar, err := cookiejar.New(nil)
			if err != nil {
				return nil, err
			}
			m.jars[i] = jar
		}
	}

	return m, nil
}

// Enabled reports whether cookie jar management is active.
func (m *VUJarManager) Enabled() bool {
	if m == nil {
		return false
	}
	return m.enabled
}

// GetJar returns the dedicated http.CookieJar for a configured worker index.
// Out-of-range IDs return nil so runtime state remains bounded by the scheduler's
// validated concurrency limit.
func (m *VUJarManager) GetJar(vuID int) http.CookieJar {
	if m == nil || !m.enabled || vuID < 0 || vuID >= len(m.jars) {
		return nil
	}
	return m.jars[vuID]
}
