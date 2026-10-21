package dueevent

import "testing"

func TestUUIDValidation(t *testing.T) {
	cases := []struct {
		id    string
		valid bool
	}{
		{"e26d9ad3-94bf-40cf-9264-04a09682f4ee", true},
		{"e26d9ad3-94bf-40cf-9264-aaaa-04a09682f4ee", false},
		{"garbage", false},
		{"", false},
	}
	for _, tc := range cases {
		if validUUID(tc.id) != tc.valid {
			t.Errorf("validUUID(%q) = %v, want %v", tc.id, !tc.valid, tc.valid)
		}
	}
}
