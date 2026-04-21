package utils

import "testing"

func TestGetEnvReturnsFallbackWhenUnset(t *testing.T) {
	t.Setenv("API_HEALTH_MONITOR_TEST_ENV", "")
	if got := GetEnv("API_HEALTH_MONITOR_DOES_NOT_EXIST", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback value, got %q", got)
	}
}

func TestGetEnvReturnsConfiguredValue(t *testing.T) {
	t.Setenv("API_HEALTH_MONITOR_TEST_ENV", "configured")
	if got := GetEnv("API_HEALTH_MONITOR_TEST_ENV", "fallback"); got != "configured" {
		t.Fatalf("expected configured value, got %q", got)
	}
}
