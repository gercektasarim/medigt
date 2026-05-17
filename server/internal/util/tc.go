package util

import "fmt"

// ValidateTC validates a Turkish TC kimlik number using the official algorithm.
// Format: 11 digits, first non-zero, satisfies two checksum rules.
func ValidateTC(tc string) bool {
	if len(tc) != 11 {
		return false
	}
	digits := [11]int{}
	for i, r := range tc {
		if r < '0' || r > '9' {
			return false
		}
		digits[i] = int(r - '0')
	}
	if digits[0] == 0 {
		return false
	}

	oddSum := digits[0] + digits[2] + digits[4] + digits[6] + digits[8]
	evenSum := digits[1] + digits[3] + digits[5] + digits[7]
	check10 := (oddSum*7 - evenSum) % 10
	if check10 < 0 {
		check10 += 10
	}
	if check10 != digits[9] {
		return false
	}

	totalSum := 0
	for i := 0; i < 10; i++ {
		totalSum += digits[i]
	}
	if totalSum%10 != digits[10] {
		return false
	}
	return true
}

// MaskTC masks all but the last 4 digits, used for log/audit output (KVKK).
func MaskTC(tc string) string {
	if len(tc) != 11 {
		return tc
	}
	return "*******" + tc[7:]
}

// FormatMRN zero-pads an MRN sequence value into an 8-digit string.
// e.g. 1 -> "00000001", 100000 -> "00100000". Numbers larger than 8 digits
// are returned as-is (no truncation).
func FormatMRN(n int64) string {
	return fmt.Sprintf("%08d", n)
}
