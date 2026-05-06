package main

import "testing"

func TestGetenvFallback(t *testing.T) {
	t.Setenv("UI_RUNNER_TEST_VALUE", "")
	if got := getenv("UI_RUNNER_TEST_VALUE", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestGetenvValue(t *testing.T) {
	t.Setenv("UI_RUNNER_TEST_VALUE", "from-env")
	if got := getenv("UI_RUNNER_TEST_VALUE", "fallback"); got != "from-env" {
		t.Fatalf("expected env value, got %q", got)
	}
}
