package platform

import (
	"regexp"
	"testing"
)

func TestNewTelegramLinkCodeIsEightDigits(t *testing.T) {
	code, err := newTelegramLinkCode()
	if err != nil {
		t.Fatalf("newTelegramLinkCode returned error: %v", err)
	}
	if !regexp.MustCompile(`^[0-9]{8}$`).MatchString(code) {
		t.Fatalf("link code %q is not eight digits", code)
	}
}
