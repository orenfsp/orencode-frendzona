package emailcode

import (
	"strings"
	"testing"
)

func TestSendWithoutSMTPLogsCodeAndVerifies(t *testing.T) {
	s := New(Config{})
	delivered, err := s.Send("User@Example.com")
	if err != nil || delivered {
		t.Fatalf("без почтовых настроек код только в логе: delivered=%v err=%v", delivered, err)
	}
	code := s.codes["user@example.com"].code
	if len(code) != 6 {
		t.Fatalf("код должен быть шестизначным: %q", code)
	}
	if s.Verify("user@example.com", "000000") && code != "000000" {
		t.Fatal("неверный код не должен проходить")
	}
	if !s.Verify("USER@example.com ", code) {
		t.Fatal("верный код (регистр и пробелы email не важны) должен проходить")
	}
	if s.Verify("user@example.com", code) {
		t.Fatal("код одноразовый")
	}
}

func TestResendLimitAndAttempts(t *testing.T) {
	s := New(Config{})
	if _, err := s.Send("a@b.ru"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Send("a@b.ru"); err == nil || !strings.Contains(err.Error(), "через минуту") {
		t.Fatalf("повторная отправка раньше минуты должна отклоняться: %v", err)
	}
	for i := 0; i < 5; i++ {
		s.Verify("a@b.ru", "bad")
	}
	if s.Verify("a@b.ru", s.codes["a@b.ru"].code) {
		t.Fatal("после 5 неверных попыток код блокируется")
	}
}

func TestInvalidEmailRejected(t *testing.T) {
	if _, err := New(Config{}).Send("not-an-email"); err == nil {
		t.Fatal("некорректный email должен давать ошибку")
	}
}

// Регрессия: если письмо не ушло, повторить отправку можно сразу, а не ждать минуту.
func TestFailedDeliveryClearsCooldown(t *testing.T) {
	s := New(Config{Host: "127.0.0.1", Port: "1", User: "u", Password: "p", From: "u@x.ru"})
	if _, err := s.Send("x@y.ru"); err == nil {
		t.Fatal("отправка на недоступный SMTP должна вернуть ошибку")
	}
	if _, ok := s.codes["x@y.ru"]; ok {
		t.Fatal("после неудачной отправки код не должен оставаться в памяти")
	}
	if _, err := s.Send("x@y.ru"); err != nil && strings.Contains(err.Error(), "через минуту") {
		t.Fatal("повторная попытка не должна упираться в ограничение «раз в минуту»")
	}
}

func TestPasswordSpacesRemoved(t *testing.T) {
	s := New(Config{Host: " smtp.gmail.com ", Password: "abcd efgh ijkl mnop"})
	if s.cfg.Password != "abcdefghijklmnop" || s.cfg.Host != "smtp.gmail.com" {
		t.Fatalf("настройки не нормализованы: %+v", s.cfg)
	}
}

func TestConfiguredByBrevoKeyOnly(t *testing.T) {
	if New(Config{}).configured() {
		t.Fatal("пустые настройки — не настроено")
	}
	if !New(Config{BrevoKey: "k"}).configured() {
		t.Fatal("ключ Brevo сам по себе включает отправку")
	}
}
