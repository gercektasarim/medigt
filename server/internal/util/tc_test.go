package util

import "testing"

func TestValidateTC(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty", "", false},
		{"too short", "1234567890", false},
		{"non-digit", "1234567890a", false},
		{"leading zero", "01234567890", false},
		{"all zeros", "00000000000", false},
		// Known-valid test TC (frequently used in SGK test documentation):
		{"valid example", "10000000146", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateTC(tc.input)
			if got != tc.want {
				t.Errorf("ValidateTC(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestMaskTC(t *testing.T) {
	if got := MaskTC("12345678901"); got != "*******8901" {
		t.Errorf("MaskTC: got %q", got)
	}
	if got := MaskTC("short"); got != "short" {
		t.Errorf("MaskTC short passthrough: got %q", got)
	}
}

func TestFormatMRN(t *testing.T) {
	cases := map[int64]string{
		1:        "00000001",
		100000:   "00100000",
		99999999: "99999999",
		0:        "00000000",
	}
	for in, want := range cases {
		if got := FormatMRN(in); got != want {
			t.Errorf("FormatMRN(%d) = %q, want %q", in, got, want)
		}
	}
}
