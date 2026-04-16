package service

import (
	"testing"
)

func TestNormalizeHandle(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr error
	}{
		{"alice", "alice", nil},
		{"  Alice  ", "alice", nil},
		{"UPPER", "upper", nil},
		{"user_123", "user_123", nil},
		{"abc", "abc", nil},
		{"a_32_char_handle_that_is_valid_x", "a_32_char_handle_that_is_valid_x", nil},

		// too short
		{"ab", "", ErrInvalidHandle},
		{"", "", ErrInvalidHandle},
		{"  ", "", ErrInvalidHandle},

		// invalid characters
		{"has space", "", ErrInvalidHandle},
		{"has-dash", "", ErrInvalidHandle},
		{"has.dot", "", ErrInvalidHandle},
		{"HAS@SIGN", "", ErrInvalidHandle},

		// too long (33 chars)
		{"a_33_char_handle_that_is_invalid_xy", "", ErrInvalidHandle},
	}
	for _, tt := range tests {
		got, err := normalizeHandle(tt.input)
		if err != tt.wantErr {
			t.Errorf("normalizeHandle(%q): err = %v, want %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("normalizeHandle(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr error
	}{
		{"alice@example.com", "alice@example.com", nil},
		{"  Alice@Example.COM  ", "alice@example.com", nil},
		{"user+tag@example.com", "user+tag@example.com", nil},

		// invalid
		{"", "", ErrInvalidEmail},
		{"not-an-email", "", ErrInvalidEmail},
		{"@example.com", "", ErrInvalidEmail},
		{"  ", "", ErrInvalidEmail},
	}
	for _, tt := range tests {
		got, err := normalizeEmail(tt.input)
		if err != tt.wantErr {
			t.Errorf("normalizeEmail(%q): err = %v, want %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("normalizeEmail(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		input   string
		wantErr error
	}{
		{"password123", nil},
		{"12345678", nil},
		{"exactly8", nil},

		// too short
		{"1234567", ErrInvalidPassword},
		{"short", ErrInvalidPassword},
		{"", ErrInvalidPassword},

		// whitespace-only or whitespace-padded short
		{"       ", ErrInvalidPassword},
		{"  short ", ErrInvalidPassword},
	}
	for _, tt := range tests {
		if err := validatePassword(tt.input); err != tt.wantErr {
			t.Errorf("validatePassword(%q) = %v, want %v", tt.input, err, tt.wantErr)
		}
	}
}

func TestNormalizeRoles(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"nil input always includes user", nil, []string{"user"}},
		{"empty input always includes user", []string{}, []string{"user"}},
		{"user only", []string{"user"}, []string{"user"}},
		{"admin adds user", []string{"admin"}, []string{"user", "admin"}},
		{"all valid roles", []string{"admin", "tester", "user"}, []string{"user", "admin", "tester"}},
		{"ignores unknown roles", []string{"superadmin", "root"}, []string{"user"}},
		{"trims and lowercases", []string{" Admin ", " TESTER "}, []string{"user", "admin", "tester"}},
		{"deduplicates", []string{"user", "user", "admin", "admin"}, []string{"user", "admin"}},
		{"deterministic order", []string{"tester", "admin", "user"}, []string{"user", "admin", "tester"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeRoles(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("normalizeRoles(%v) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("normalizeRoles(%v)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
