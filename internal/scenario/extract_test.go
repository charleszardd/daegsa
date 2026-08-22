package scenario_test

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"

	"github.com/charleszardd/daegsa/internal/scenario"
)

func TestExtractJSON(t *testing.T) {
	jsonPayload := []byte(`{
		"token": "tok_12345",
		"data": {
			"user": {
				"id": 99,
				"name": "Alice",
				"active": true,
				"score": 87.5
			},
			"items": [
				{"id": "item-0", "price": 10},
				{"id": "item-1", "price": 20}
			],
			"tags": ["alpha", "beta", "gamma"]
		},
		"empty_val": null
	}`)

	tests := []struct {
		name      string
		expr      string
		want      string
		expectErr bool
	}{
		{
			name: "top-level string",
			expr: "token",
			want: "tok_12345",
		},
		{
			name: "top-level string with leading $.",
			expr: "$.token",
			want: "tok_12345",
		},
		{
			name: "nested integer",
			expr: "data.user.id",
			want: "99",
		},
		{
			name: "nested string",
			expr: "$.data.user.name",
			want: "Alice",
		},
		{
			name: "nested boolean",
			expr: "data.user.active",
			want: "true",
		},
		{
			name: "nested float",
			expr: "data.user.score",
			want: "87.5",
		},
		{
			name: "array indexing nested object",
			expr: "data.items[0].id",
			want: "item-0",
		},
		{
			name: "array indexing second element",
			expr: "$.data.items[1].id",
			want: "item-1",
		},
		{
			name: "array scalar indexing",
			expr: "data.tags[2]",
			want: "gamma",
		},
		{
			name:      "missing key",
			expr:      "data.user.nonexistent",
			expectErr: true,
		},
		{
			name:      "array index out of bounds",
			expr:      "data.items[99].id",
			expectErr: true,
		},
		{
			name:      "traverse key on non-object",
			expr:      "token.id",
			expectErr: true,
		},
		{
			name:      "array index on non-array",
			expr:      "token[0]",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := scenario.ExtractValue(nil, jsonPayload, scenario.ExtractionRule{
				From:       scenario.SourceJSON,
				Expression: tt.expr,
			}, nil)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got val: %q", val)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if val != tt.want {
					t.Errorf("got %q, want %q", val, tt.want)
				}
			}
		})
	}
}

func TestExtractHeader(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"X-Request-Id": []string{"req_98765"},
			"Content-Type": []string{"application/json; charset=utf-8"},
		},
	}

	tests := []struct {
		name      string
		header    string
		want      string
		expectErr bool
	}{
		{
			name:   "case exact header",
			header: "X-Request-Id",
			want:   "req_98765",
		},
		{
			name:   "case insensitive header",
			header: "x-request-id",
			want:   "req_98765",
		},
		{
			name:      "missing header",
			header:    "X-Missing-Header",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := scenario.ExtractValue(resp, nil, scenario.ExtractionRule{
				From:       scenario.SourceHeader,
				Expression: tt.header,
			}, nil)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got val: %q", val)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if val != tt.want {
					t.Errorf("got %q, want %q", val, tt.want)
				}
			}
		})
	}
}

func TestExtractCookie(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	testURL, _ := url.Parse("https://example.com/api")
	jar.SetCookies(testURL, []*http.Cookie{
		{Name: "jar_cookie", Value: "jar_val_456"},
	})

	resp := &http.Response{
		Header: http.Header{
			"Set-Cookie": []string{"session_id=sess_123; Path=/", "other=abc; Path=/"},
		},
		Request: &http.Request{
			URL: testURL,
		},
	}

	state := scenario.NewVUState(1, jar, nil)

	tests := []struct {
		name       string
		cookieName string
		want       string
		expectErr  bool
	}{
		{
			name:       "cookie from Set-Cookie",
			cookieName: "session_id",
			want:       "sess_123",
		},
		{
			name:       "cookie from cookie jar",
			cookieName: "jar_cookie",
			want:       "jar_val_456",
		},
		{
			name:       "missing cookie",
			cookieName: "nonexistent_cookie",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := scenario.ExtractValue(resp, nil, scenario.ExtractionRule{
				From:       scenario.SourceCookie,
				Expression: tt.cookieName,
			}, state)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got val: %q", val)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if val != tt.want {
					t.Errorf("got %q, want %q", val, tt.want)
				}
			}
		})
	}
}

func TestExtractRegex(t *testing.T) {
	body := []byte(`{"message": "user created with id=12345&status=confirmed"}`)

	tests := []struct {
		name      string
		pattern   string
		want      string
		expectErr bool
	}{
		{
			name:    "regex with capture group",
			pattern: `id=([0-9]+)`,
			want:    "12345",
		},
		{
			name:    "regex whole match without group",
			pattern: `confirmed`,
			want:    "confirmed",
		},
		{
			name:      "regex no match",
			pattern:   `error_code=([0-9]+)`,
			expectErr: true,
		},
		{
			name:      "invalid regex syntax",
			pattern:   `[a-z(`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := scenario.ExtractValue(nil, body, scenario.ExtractionRule{
				From:       scenario.SourceRegex,
				Expression: tt.pattern,
			}, nil)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got val: %q", val)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if val != tt.want {
					t.Errorf("got %q, want %q", val, tt.want)
				}
			}
		})
	}
}

func TestExtractAll(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"X-Request-Id": []string{"req-100"},
			"Set-Cookie":   []string{"sess=s1"},
		},
	}
	body := []byte(`{"user":{"id":42,"token":"jwt.secret"}}`)

	state := scenario.NewVUState(0, nil, nil)
	rules := map[string]scenario.ExtractionRule{
		"token": {
			From:       scenario.SourceJSON,
			Expression: "user.token",
		},
		"user_id": {
			From:       scenario.SourceJSON,
			Expression: "user.id",
		},
		"req_id": {
			From:       scenario.SourceHeader,
			Expression: "X-Request-Id",
		},
		"session": {
			From:       scenario.SourceCookie,
			Expression: "sess",
		},
	}

	err := scenario.ExtractAll(resp, body, rules, state)
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	if state.Variables["token"] != "jwt.secret" {
		t.Errorf("expected token 'jwt.secret', got %q", state.Variables["token"])
	}
	if state.Variables["user_id"] != "42" {
		t.Errorf("expected user_id '42', got %q", state.Variables["user_id"])
	}
	if state.Variables["req_id"] != "req-100" {
		t.Errorf("expected req_id 'req-100', got %q", state.Variables["req_id"])
	}
	if state.Variables["session"] != "s1" {
		t.Errorf("expected session 's1', got %q", state.Variables["session"])
	}
}
