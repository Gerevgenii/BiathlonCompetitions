package main

import (
	_ "io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseEvent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		line           string
		expectedString string
	}{
		{
			name:           "test_invalid_event_format",
			line:           "hello bad 1",
			expectedString: "",
		},
		{
			name:           "test_time_should_be_in_brackets",
			line:           "09:30:00.000 4 1",
			expectedString: "",
		},
		{
			name:           "test_time_format",
			line:           "[09:30:bad] 4 1",
			expectedString: "",
		},
		{
			name:           "test_success_time",
			line:           "[09:30:01.005] 4 1",
			expectedString: "success",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			event, err := parseEvent(test.line)
			if test.expectedString != "" {
				require.NoError(t, err)
				require.Equal(t, event.EventID, 4)
				require.Equal(t, event.CompetitorID, 1)
				require.Equal(t, event.RawTime, "09:30:01.005")
				expectedTime, _ := time.Parse(timeLayout, "09:30:01.005")
				require.Equal(t, event.Time, expectedTime)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	_, err := loadConfig("config/config.json")
	require.NoError(t, err)
}

func TestParseDelta(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expecting time.Duration
	}{
		{
			name:      "test_parse_seconds",
			input:     "00:00:30",
			expecting: 30 * time.Second,
		},
		{
			name:      "test_parse_minutes_and_hours",
			input:     "01:30:00",
			expecting: time.Hour + 30*time.Minute,
		},
		{
			name:      "test_parse_time",
			input:     "01:23:45.670",
			expecting: 1*time.Hour + 23*time.Minute + 45*time.Second + 670*time.Millisecond,
		},
		{
			name:      "test_incorrect_format_time",
			input:     "30s",
			expecting: 0,
		},
		{
			name:      "test_time_should_match_regular_schedule",
			input:     "1:2",
			expecting: 0,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d, err := parseDelta(test.input)
			if test.expecting != 0 {
				require.NoError(t, err)
				require.Equal(t, test.expecting, d)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestEventRegexGroups(t *testing.T) {
	line := "[12:34:56.789] 5 10 extra params"
	matches := eventRegex.FindStringSubmatch(line)
	expected := []string{"[12:34:56.789] 5 10 extra params", "12:34:56.789", "5", "10", "extra params"}
	require.Equal(t, expected, matches)
}
