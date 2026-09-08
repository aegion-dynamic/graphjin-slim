package http

import "testing"

func TestQueryName(t *testing.T) {
	tests := []struct {
		path string
		want string
		ok   bool
	}{
		{path: "/api/v1/rest/GetOrders", want: "GetOrders", ok: true},
		{path: "/rest/GetOrders", want: "GetOrders", ok: true},
		{path: "/rest/GetOrders/", want: "GetOrders", ok: true},
		{path: "/api/v1/rest/GetOrders?limit=10", want: "GetOrders", ok: true},
		{path: "/rest/GetOrders?limit=10", want: "GetOrders", ok: true},
		{path: "/api/v1/rest/", ok: false},
		{path: "/rest/", ok: false},
		{path: "/rest", ok: false},
		{path: "/api/v1/graphql", ok: false},
		{path: "/api/v1/users/me", ok: false},
		{path: "", ok: false},
	}
	for _, tt := range tests {
		got, err := QueryName(tt.path)
		if tt.ok {
			if err != nil {
				t.Fatalf("QueryName(%q) unexpected error: %v", tt.path, err)
			}
			if got != tt.want {
				t.Fatalf("QueryName(%q) = %q, want %q", tt.path, got, tt.want)
			}
			continue
		}
		if err == nil {
			t.Fatalf("QueryName(%q) = %q, want error", tt.path, got)
		}
	}
}
