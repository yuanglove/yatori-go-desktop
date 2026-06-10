package service

import "testing"

func TestParseWeLearnMinutes(t *testing.T) {
	cases := map[string]int{
		"15":     15,
		"15分钟":   15,
		"15 min": 15,
		"20mins": 20,
		"30m":    30,
	}
	for input, want := range cases {
		got, err := parseWeLearnMinutes(input)
		if err != nil {
			t.Fatalf("parseWeLearnMinutes(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("parseWeLearnMinutes(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestParseWeLearnStudyTime(t *testing.T) {
	noop := func(string, ...interface{}) {}
	if got := parseWeLearnStudyTime("15分钟", noop, "test") / 60; got != 15 {
		t.Fatalf("parseWeLearnStudyTime single value = %d minutes, want 15", got)
	}
	for i := 0; i < 10; i++ {
		got := parseWeLearnStudyTime("15分钟-16分钟", noop, "test") / 60
		if got < 15 || got > 16 {
			t.Fatalf("parseWeLearnStudyTime range = %d minutes, want between 15 and 16", got)
		}
	}
}
