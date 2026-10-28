// Package emailcode отправляет одноразовые коды подтверждения на почту (SMTP).
// Если SMTP не настроен, код печатается в лог сервера (удобно для демо).
package emailcode

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Host, Port, User, Password, From string
	// BrevoKey — ключ HTTP API Brevo (https://brevo.com). Нужен на хостингах, которые блокируют исходящий SMTP (Render и др.).
	BrevoKey string
}

type entry struct {
	code     string
	expires  time.Time
	sentAt   time.Time
	attempts int
}

type Service struct {
	cfg   Config
	mu    sync.Mutex
	codes map[string]entry
}

func New(cfg Config) *Service {
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.User = strings.TrimSpace(cfg.User)
	// Пароли приложений Gmail часто копируют с пробелами: «abcd efgh ijkl mnop».
	cfg.Password = strings.ReplaceAll(strings.TrimSpace(cfg.Password), " ", "")
	return &Service{cfg: cfg, codes: map[string]entry{}}
}

func (s *Service) configured() bool {
	return strings.TrimSpace(s.cfg.Host) != "" || strings.TrimSpace(s.cfg.BrevoKey) != ""
}

// Send генерирует 6-значный код и отправляет его на email.
// Возвращает delivered=false, если SMTP не настроен и код записан только в лог.
func (s *Service) Send(email string) (delivered bool, err error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return false, errors.New("Укажите корректный email")
	}
	s.mu.Lock()
	if old, ok := s.codes[email]; ok && time.Since(old.sentAt) < 60*time.Second {
		s.mu.Unlock()
		return false, errors.New("Код уже отправлен, повторить можно через минуту")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		s.mu.Unlock()
		return false, err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	s.codes[email] = entry{code: code, expires: time.Now().Add(10 * time.Minute), sentAt: time.Now()}
	s.mu.Unlock()

	if !s.configured() {
		log.Printf("[emailcode] SMTP не настроен. Код для %s: %s", email, code)
		return false, nil
	}
	if err := s.send(email, code); err != nil {
		log.Printf("[emailcode] ошибка отправки на %s: %v", email, err)
		// письмо не ушло — снимаем ограничение «раз в минуту», чтобы можно было сразу повторить
		s.mu.Lock()
		delete(s.codes, email)
		s.mu.Unlock()
		return false, errors.New("Не удалось отправить письмо, попробуйте ещё раз. Если ошибка повторяется — сообщите организаторам (письмо не доходит от хостинга)")
	}
	return true, nil
}

// Verify проверяет код; успешный код одноразовый.
func (s *Service) Verify(email, code string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.codes[email]
	if !ok || time.Now().After(e.expires) || e.attempts >= 5 {
		return false
	}
	if strings.TrimSpace(code) != e.code {
		e.attempts++
		s.codes[email] = e
		return false
	}
	delete(s.codes, email)
	return true
}

// send пробует HTTP API (если задан ключ), затем SMTP на основном и запасном порту, по две попытки.
func (s *Service) send(to, code string) error {
	var errs []string
	if strings.TrimSpace(s.cfg.BrevoKey) != "" {
		for i := 0; i < 2; i++ {
			err := s.sendBrevo(to, code)
			if err == nil {
				return nil
			}
			errs = append(errs, "brevo: "+err.Error())
			time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
		}
	}
	if strings.TrimSpace(s.cfg.Host) != "" {
		primary := s.cfg.Port
		if primary == "" {
			primary = "587"
		}
		ports := []string{primary}
		if primary == "587" {
			ports = append(ports, "465")
		} else if primary == "465" {
			ports = append(ports, "587")
		}
		for _, port := range ports {
			for i := 0; i < 2; i++ {
				err := s.sendSMTP(to, code, port)
				if err == nil {
					return nil
				}
				errs = append(errs, "smtp:"+port+": "+err.Error())
				var ne net.Error
				if errors.As(err, &ne) && ne.Timeout() {
					errs = append(errs, "(таймаут: хостинг, вероятно, блокирует исходящий SMTP — используйте BREVO_API_KEY)")
					break
				}
				time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
			}
		}
	}
	return errors.New(strings.Join(errs, "; "))
}

func (s *Service) fromAddr() string {
	if s.cfg.From != "" {
		return s.cfg.From
	}
	return s.cfg.User
}

func (s *Service) sendBrevo(to, code string) error {
	body, _ := json.Marshal(map[string]any{
		"sender":      map[string]string{"name": "ФСП 56", "email": s.fromAddr()},
		"to":          []map[string]string{{"email": to}},
		"subject":     "Код подтверждения ФСП 56",
		"textContent": "Ваш код подтверждения: " + code + "\nКод действует 10 минут.",
	})
	req, err := http.NewRequest(http.MethodPost, "https://api.brevo.com/v3/smtp/email", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("api-key", strings.TrimSpace(s.cfg.BrevoKey))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b := make([]byte, 300)
		n, _ := resp.Body.Read(b)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b[:n])))
	}
	return nil
}

func (s *Service) sendSMTP(to, code, port string) error {
	from := s.fromAddr()
	addr := net.JoinHostPort(s.cfg.Host, port)
	msg := "From: " + from + "\r\nTo: " + to + "\r\nSubject: =?UTF-8?B?" + b64("Код подтверждения ФСП 56") + "?=\r\n" +
		"MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" +
		"Ваш код подтверждения: " + code + "\r\nКод действует 10 минут.\r\n"
	var auth smtp.Auth
	if s.cfg.User != "" {
		auth = smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)
	}
	if port != "465" { // STARTTLS (587/25) с таймаутом, чтобы запрос не зависал
		return s.sendSTARTTLS(addr, auth, from, to, msg)
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, &tls.Config{ServerName: s.cfg.Host})
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return err
	}
	defer c.Close()
	if auth != nil {
		if err = c.Auth(auth); err != nil {
			return err
		}
	}
	if err = c.Mail(from); err != nil {
		return err
	}
	if err = c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write([]byte(msg)); err != nil {
		return err
	}
	return w.Close()
}

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func (s *Service) sendSTARTTLS(addr string, auth smtp.Auth, from, to, msg string) error {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err = c.StartTLS(&tls.Config{ServerName: s.cfg.Host}); err != nil {
			return err
		}
	}
	if auth != nil {
		if err = c.Auth(auth); err != nil {
			return err
		}
	}
	if err = c.Mail(from); err != nil {
		return err
	}
	if err = c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write([]byte(msg)); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
