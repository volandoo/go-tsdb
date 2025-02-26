package main

import (
	"testing"
)

func TestNewCollection(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantName    string
		wantTTL     int
		shouldPanic bool
	}{
		{
			name:     "valid simple collection",
			input:    "users:3600",
			wantName: "users",
			wantTTL:  3600,
		},
		{
			name:     "valid collection with wildcard",
			input:    "users.*:7200",
			wantName: "users.*",
			wantTTL:  7200,
		},
		{
			name:        "invalid format - missing TTL",
			input:       "users",
			shouldPanic: true,
		},
		{
			name:        "invalid format - empty string",
			input:       "",
			shouldPanic: true,
		},
		{
			name:        "invalid format - multiple colons",
			input:       "users:3600:extra",
			shouldPanic: true,
		},
		{
			name:        "invalid format - multiple wildcards",
			input:       "users.*.posts.*:3600",
			shouldPanic: true,
		},
		{
			name:        "invalid TTL - not a number",
			input:       "users:abc",
			shouldPanic: true,
		},
		{
			name:        "invalid TTL - negative number",
			input:       "users:-3600",
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("NewCollection(%q) should have panicked", tt.input)
					}
				}()
			}

			got := NewCollection(tt.input)
			if !tt.shouldPanic {
				if got.Name != tt.wantName {
					t.Errorf("NewCollection(%q).Name = %q, want %q", tt.input, got.Name, tt.wantName)
				}
				if got.TTL != tt.wantTTL {
					t.Errorf("NewCollection(%q).TTL = %d, want %d", tt.input, got.TTL, tt.wantTTL)
				}
			}
		})
	}
}

func TestCollection_IsCollection(t *testing.T) {
	tests := []struct {
		name       string
		collection Collection
		input      string
		want       bool
	}{
		{
			name:       "exact match",
			collection: Collection{Name: "users", TTL: 3600},
			input:      "users",
			want:       true,
		},
		{
			name:       "no match",
			collection: Collection{Name: "users", TTL: 3600},
			input:      "posts",
			want:       false,
		},
		{
			name:       "wildcard match",
			collection: Collection{Name: "users.*", TTL: 3600},
			input:      "users.123",
			want:       true,
		},
		{
			name:       "wildcard no match - different prefix",
			collection: Collection{Name: "users.*", TTL: 3600},
			input:      "posts.123",
			want:       false,
		},
		{
			name:       "wildcard no match - extra segments",
			collection: Collection{Name: "users.*", TTL: 3600},
			input:      "users.123.posts",
			want:       false,
		},
		{
			name:       "wildcard no match - missing segment",
			collection: Collection{Name: "users.*", TTL: 3600},
			input:      "users",
			want:       false,
		},
		{
			name:       "empty string",
			collection: Collection{Name: "users", TTL: 3600},
			input:      "",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.collection.IsCollection(tt.input)
			if got != tt.want {
				t.Errorf("Collection{Name: %q}.IsCollection(%q) = %v, want %v",
					tt.collection.Name, tt.input, got, tt.want)
			}
		})
	}
}
