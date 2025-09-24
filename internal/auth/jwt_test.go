/*
Copyright (c) 2024 Ansible Project

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"testing"
	"time"
)

func TestJWTManager_GenerateToken(t *testing.T) {
	jwtManager, err := NewJWTManager("ansible-go", []string{"ansible-api"}, time.Hour)
	if err != nil {
		t.Fatalf("Failed to create JWT manager: %v", err)
	}

	userID := "test-user"
	roles := []string{"admin", "operator"}

	token, err := jwtManager.GenerateToken(userID, roles)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	if token == "" {
		t.Fatal("Generated token is empty")
	}

	// Validate the generated token
	claims, err := jwtManager.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate generated token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("Expected UserID %s, got %s", userID, claims.UserID)
	}

	if len(claims.Roles) != len(roles) {
		t.Errorf("Expected %d roles, got %d", len(roles), len(claims.Roles))
	}

	for i, role := range roles {
		if claims.Roles[i] != role {
			t.Errorf("Expected role %s at index %d, got %s", role, i, claims.Roles[i])
		}
	}
}

func TestJWTManager_ValidateToken_InvalidToken(t *testing.T) {
	jwtManager, err := NewJWTManager("ansible-go", []string{"ansible-api"}, time.Hour)
	if err != nil {
		t.Fatalf("Failed to create JWT manager: %v", err)
	}

	invalidToken := "invalid.token.string"

	_, err = jwtManager.ValidateToken(invalidToken)
	if err == nil {
		t.Fatal("Expected error for invalid token, got nil")
	}
}

func TestClaims_HasRole(t *testing.T) {
	claims := &Claims{
		Roles: []string{"admin", "operator", "reader"},
	}

	tests := []struct {
		role     string
		expected bool
	}{
		{"admin", true},
		{"operator", true},
		{"reader", true},
		{"writer", false},
		{"", false},
	}

	for _, test := range tests {
		result := claims.HasRole(test.role)
		if result != test.expected {
			t.Errorf("HasRole(%s): expected %v, got %v", test.role, test.expected, result)
		}
	}
}

func TestClaims_HasAnyRole(t *testing.T) {
	claims := &Claims{
		Roles: []string{"admin", "operator"},
	}

	tests := []struct {
		roles    []string
		expected bool
	}{
		{[]string{"admin"}, true},
		{[]string{"operator"}, true},
		{[]string{"admin", "operator"}, true},
		{[]string{"reader"}, false},
		{[]string{"reader", "writer"}, false},
		{[]string{"admin", "reader"}, true},
		{[]string{}, false},
	}

	for _, test := range tests {
		result := claims.HasAnyRole(test.roles...)
		if result != test.expected {
			t.Errorf("HasAnyRole(%v): expected %v, got %v", test.roles, test.expected, result)
		}
	}
}

func TestClaims_Valid(t *testing.T) {
	tests := []struct {
		name    string
		claims  Claims
		wantErr bool
	}{
		{
			name: "valid claims",
			claims: Claims{
				UserID: "test-user",
				Roles:  []string{"admin"},
			},
			wantErr: false,
		},
		{
			name: "missing user ID",
			claims: Claims{
				Roles: []string{"admin"},
			},
			wantErr: true,
		},
		{
			name: "empty user ID",
			claims: Claims{
				UserID: "",
				Roles:  []string{"admin"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.claims.Valid()
			if (err != nil) != tt.wantErr {
				t.Errorf("Claims.Valid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateJTI(t *testing.T) {
	// Generate multiple JTIs to check uniqueness
	jtis := make(map[string]bool)
	for i := 0; i < 100; i++ {
		jti := generateJTI()
		if jti == "" {
			t.Fatal("Generated JTI is empty")
		}
		if jtis[jti] {
			t.Fatalf("Duplicate JTI generated: %s", jti)
		}
		jtis[jti] = true
	}
}