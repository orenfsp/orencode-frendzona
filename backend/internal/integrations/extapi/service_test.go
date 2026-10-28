package extapi

import (
	"strings"
	"testing"
)

func TestSignIsStableAndDependsOnInputs(t *testing.T) {
	body := []byte(`{"event":"webhook.test"}`)
	a := Sign("secret", "1700000000", body)
	if !strings.HasPrefix(a, "sha256=") || len(a) != len("sha256=")+64 {
		t.Fatalf("неожиданный формат подписи: %s", a)
	}
	if a != Sign("secret", "1700000000", body) {
		t.Fatal("подпись должна быть детерминированной")
	}
	if a == Sign("other", "1700000000", body) || a == Sign("secret", "1700000001", body) || a == Sign("secret", "1700000000", []byte("x")) {
		t.Fatal("подпись должна зависеть от секрета, времени и тела")
	}
}

func TestValidateURL(t *testing.T) {
	for _, ok := range []string{"https://example.com/hook", "http://localhost:9000/x"} {
		if err := ValidateURL(ok); err != nil {
			t.Errorf("%s должен приниматься: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "ftp://example.com", "javascript:alert(1)", "http://169.254.169.254/latest", "example.com"} {
		if err := ValidateURL(bad); err == nil {
			t.Errorf("%q должен отклоняться", bad)
		}
	}
}

func TestEndpointWants(t *testing.T) {
	if !(endpoint{Events: "*"}).wants("protocol.published") {
		t.Error("* подписывает на всё")
	}
	e := endpoint{Events: "protocol.published, rank.decided"}
	if !e.wants("rank.decided") || e.wants("competition.published") {
		t.Error("подписка должна учитывать список событий")
	}
}

func TestCatalogCodes(t *testing.T) {
	if !validEvent("application.approved") || validEvent("nope") || !validScope("results:write") || validScope("admin") {
		t.Error("справочники событий и прав работают неверно")
	}
}
