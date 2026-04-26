package duration

import "testing"

func TestParseMillis(t *testing.T) {
	tests := map[string]int64{
		"1":    1000,
		"120s": 120000,
		"2m":   120000,
		"1h":   3600000,
	}
	for input, want := range tests {
		got, err := ParseMillis(input)
		if err != nil {
			t.Fatalf("ParseMillis(%q) error = %v", input, err)
		}
		if got != want {
			t.Fatalf("ParseMillis(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestParseMillisRejectsInvalidValues(t *testing.T) {
	for _, input := range []string{"", "0", "01", "010s", "00m", "-1", "1.5s", "5d", "abc", "9223372036854775807s", "9223372036854775807m", "9223372036854775807h"} {
		if _, err := ParseMillis(input); err == nil {
			t.Fatalf("ParseMillis(%q) unexpectedly succeeded", input)
		}
	}
}
