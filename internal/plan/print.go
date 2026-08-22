package plan

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
)

// FormatPlanSummary generates a sanitized, formatted human-readable summary of the execution plan (§4, §12, §13).
func FormatPlanSummary(p *Plan) string {
	if p == nil {
		return "Execution Plan: <nil>\n"
	}

	var sb strings.Builder

	shortFP := p.Fingerprint
	if len(shortFP) > 12 {
		shortFP = shortFP[:12]
	}

	redactedURL := "<nil>"
	if p.TargetURL != nil {
		redactedURL = config.RedactURL(p.TargetURL.String())
	}

	sb.WriteString("================================================================================\n")
	sb.WriteString("                         DAEGSA EXECUTION PLAN\n")
	sb.WriteString("================================================================================\n")
	sb.WriteString(fmt.Sprintf("Plan Name:            %s\n", p.Name))
	sb.WriteString(fmt.Sprintf("Schema Version:       %d\n", p.SchemaVersion))
	sb.WriteString(fmt.Sprintf("Config Fingerprint:   %s\n", shortFP))
	sb.WriteString(fmt.Sprintf("Workload Model:       %s\n", p.Model))
	sb.WriteString(fmt.Sprintf("Target URL:           %s\n", redactedURL))
	sb.WriteString(fmt.Sprintf("HTTP Method:          %s\n", p.Method))
	sb.WriteString(fmt.Sprintf("Expected Statuses:    %v\n", p.ExpectedStatuses))
	sb.WriteString(fmt.Sprintf("Request Timeout:      %v\n", p.RequestTimeout))
	sb.WriteString(fmt.Sprintf("Response Body Limit:  %d bytes\n", p.ResponseBodyLimit))
	sb.WriteString(fmt.Sprintf("Redirect Policy:      %s\n", p.RedirectPolicy))

	if p.Model == core.WorkloadModelOpen {
		sb.WriteString("--------------------------------------------------------------------------------\n")
		sb.WriteString("                         OPEN WORKLOAD PARAMETERS\n")
		sb.WriteString("--------------------------------------------------------------------------------\n")
		sb.WriteString(fmt.Sprintf("Target Rate:          %.2f requests / %v\n", p.Rate, p.TimeUnit))
		sb.WriteString(fmt.Sprintf("Max In-Flight:        %d\n", p.MaxInFlight))
		sb.WriteString(fmt.Sprintf("Test Duration:        %v\n", p.Duration))
		sb.WriteString(fmt.Sprintf("Graceful Stop:        %v\n", p.GracefulStop))
	} else if p.Model == core.WorkloadModelClosed {
		sb.WriteString("--------------------------------------------------------------------------------\n")
		sb.WriteString("                        CLOSED WORKLOAD PARAMETERS\n")
		sb.WriteString("--------------------------------------------------------------------------------\n")
		sb.WriteString(fmt.Sprintf("Virtual Users:        %d\n", p.Users))
		sb.WriteString(fmt.Sprintf("Think Time:           %v\n", p.ThinkTime))
		sb.WriteString(fmt.Sprintf("Test Duration:        %v\n", p.Duration))
		sb.WriteString(fmt.Sprintf("Graceful Stop:        %v\n", p.GracefulStop))
	}

	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString("                         AUTHENTICATION & SECRETS\n")
	sb.WriteString("--------------------------------------------------------------------------------\n")
	authMode := "none"
	tokenCount := 0
	if p.Authenticator != nil && p.Authenticator.AuthMode() != "" {
		authMode = p.Authenticator.AuthMode()
		tokenCount = p.Authenticator.TokenCount()
	} else if p.AuthType != "" {
		authMode = p.AuthType
	}
	cookieJarStr := "disabled"
	if p.CookieJarEnabled {
		cookieJarStr = "enabled"
	}
	sb.WriteString(fmt.Sprintf("Auth Mode:            %s\n", authMode))
	if authMode != "none" {
		headerName := p.AuthHeaderName
		if headerName == "" {
			headerName = "Authorization"
		}
		sb.WriteString(fmt.Sprintf("Header Name:          %s\n", headerName))
		sb.WriteString(fmt.Sprintf("Token Pool Size:      %d\n", tokenCount))
	}
	sb.WriteString(fmt.Sprintf("Cookie Jar Isolation: %s\n", cookieJarStr))

	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString("                         SAFETY & NETWORK PREFLIGHT\n")
	sb.WriteString("--------------------------------------------------------------------------------\n")
	if len(p.AllowedHosts) > 0 {
		sb.WriteString(fmt.Sprintf("Allowed Hosts:        %v\n", p.AllowedHosts))
	} else {
		sb.WriteString("Allowed Hosts:        <all hosts permitted>\n")
	}
	sb.WriteString(fmt.Sprintf("Destructive Auth:     %v\n", p.AllowNonIdempotent))

	if len(p.ResolvedIPs) > 0 {
		var ipStrs []string
		for _, ip := range p.ResolvedIPs {
			ipStrs = append(ipStrs, ip.String())
		}
		sb.WriteString(fmt.Sprintf("Resolved Target IPs:  %s\n", strings.Join(ipStrs, ", ")))
	} else {
		sb.WriteString("Resolved Target IPs:  <none>\n")
	}

	if len(p.Headers) > 0 {
		sb.WriteString("--------------------------------------------------------------------------------\n")
		sb.WriteString("                            CONFIGURED HEADERS\n")
		sb.WriteString("--------------------------------------------------------------------------------\n")
		redactedHeaders := config.RedactHTTPHeaders(p.Headers)
		var keys []string
		for k := range redactedHeaders {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, strings.Join(redactedHeaders[k], ", ")))
		}
	}
	sb.WriteString("================================================================================\n")

	return sb.String()
}
