package bot

import "testing"

func TestTestModeDefault(t *testing.T) {
	if TestMode {
		t.Fatal("TestMode should default to false")
	}
}

func TestTestModeToggle(t *testing.T) {
	TestMode = true
	if !TestMode {
		t.Fatal("TestMode should be true after set")
	}
	TestMode = false
	if TestMode {
		t.Fatal("TestMode should be false after reset")
	}
}
