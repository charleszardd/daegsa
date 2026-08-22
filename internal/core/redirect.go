package core

// Allowed redirect policies (§6, §8).
const (
	// RedirectPolicySameOrigin restricts redirects to the same origin (scheme, host, port).
	RedirectPolicySameOrigin = "same-origin"

	// RedirectPolicyNone disables all redirect following.
	RedirectPolicyNone = "none"

	// RedirectPolicyAll allows cross-origin redirects if target host is allowlisted.
	RedirectPolicyAll = "all"
)
