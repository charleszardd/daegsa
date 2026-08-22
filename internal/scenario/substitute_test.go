package scenario_test

import (
	"testing"

	"github.com/charleszardd/daegsa/internal/scenario"
)

func TestSubstituteVariables(t *testing.T) {
	vars := map[string]string{
		"token":     "xyz_token_123",
		"item_id":   "456",
		"username":  "alice",
		"base_url":  "https://api.example.com",
		"auth_mode": "Bearer",
	}

	tests := []struct {
		name      string
		template  string
		want      string
		expectErr bool
	}{
		{
			name:     "no variables",
			template: "https://api.example.com/v1/health",
			want:     "https://api.example.com/v1/health",
		},
		{
			name:     "URL path substitution",
			template: "${base_url}/items/${item_id}",
			want:     "https://api.example.com/items/456",
		},
		{
			name:     "query params substitution",
			template: "https://api.example.com/search?user=${username}&token=${token}",
			want:     "https://api.example.com/search?user=alice&token=xyz_token_123",
		},
		{
			name:     "header substitution",
			template: "${auth_mode} ${token}",
			want:     "Bearer xyz_token_123",
		},
		{
			name:     "JSON body substitution",
			template: `{"user":"${username}","itemId":${item_id}}`,
			want:     `{"user":"alice","itemId":456}`,
		},
		{
			name:     "escaping $${LITERAL}",
			template: `{"pattern":"$${NOT_REPLACED}","actual":"${username}"}`,
			want:     `{"pattern":"${NOT_REPLACED}","actual":"alice"}`,
		},
		{
			name:     "dollar sign without curly brace",
			template: "price is $100 and $$200",
			want:     "price is $100 and $$200",
		},
		{
			name:      "missing variable",
			template:  "https://api.example.com/items/${missing_key}",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scenario.SubstituteVariables(tt.template, vars)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got result: %q", got)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("got %q, want %q", got, tt.want)
				}
			}
		})
	}
}
