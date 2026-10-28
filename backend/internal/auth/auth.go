package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"orenkod/backend/internal/rules"
)

type User struct {
	ID           int64    `json:"id"`
	Email        string   `json:"email"`
	FullName     string   `json:"full_name"`
	BirthDate    string   `json:"birth_date,omitempty"`
	Municipality string   `json:"municipality"`
	Organization string   `json:"organization"`
	Phone        string   `json:"phone"`
	Roles        []string `json:"roles"`
	ActiveRole   string   `json:"active_role"`
	AthleteID    int64    `json:"athlete_id,omitempty"`
	CoachID      int64    `json:"coach_id,omitempty"`
	JudgeID      int64    `json:"judge_id,omitempty"`
	Rusada       bool     `json:"rusada_completed"`
	RusadaURL    string   `json:"rusada_url"`
	Guardian     string   `json:"guardian_name,omitempty"`
}

type contextKey string

const userKey contextKey = "auth-user"

var ErrUnauthorized = errors.New("Нужна авторизация")
var ErrForbidden = errors.New("Недостаточно прав для активной роли")

type Service struct {
	db *sql.DB
}

func New(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Login(ctx context.Context, email, password string) (string, User, error) {
	var id int64
	var hash, name string
	err := s.db.QueryRowContext(ctx, `SELECT id, password_hash, full_name FROM users WHERE lower(email)=lower($1)`, strings.TrimSpace(email)).
		Scan(&id, &hash, &name)
	if err != nil {
		return "", User{}, errors.New("Неверный логин или пароль")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", User{}, errors.New("Неверный логин или пароль")
	}
	roles, err := s.roles(ctx, id)
	if err != nil || len(roles) == 0 {
		return "", User{}, errors.New("У учётной записи нет ролей")
	}
	token, err := randomToken()
	if err != nil {
		return "", User{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO sessions(token,user_id,active_role,expires_at) VALUES($1,$2,$3,$4)`,
		token, id, roles[0], time.Now().Add(12*time.Hour))
	if err != nil {
		return "", User{}, err
	}
	_, _ = s.db.ExecContext(ctx, `INSERT INTO audit_log(user_id,role,action,entity,details) VALUES($1,$2,'вход','sessions','успешная авторизация')`, id, roles[0])
	user, err := s.loadUser(ctx, id, roles[0])
	return token, user, err
}

func (s *Service) SwitchRole(ctx context.Context, token, role string) (User, error) {
	user, err := s.UserByToken(ctx, token)
	if err != nil {
		return User{}, err
	}
	ok := false
	for _, item := range user.Roles {
		if item == role {
			ok = true
			break
		}
	}
	if !ok {
		return User{}, errors.New("Эта роль не назначена аккаунту")
	}
	_, err = s.db.ExecContext(ctx, `UPDATE sessions SET active_role=$1 WHERE token=$2`, role, token)
	if err != nil {
		return User{}, err
	}
	_, _ = s.db.ExecContext(ctx, `INSERT INTO audit_log(user_id,role,action,entity,details) VALUES($1,$2,'смена роли','sessions','активная роль изменена')`, user.ID, role)
	return s.loadUser(ctx, user.ID, role)
}

func (s *Service) Logout(ctx context.Context, token string) {
	_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token=$1`, token)
}

func (s *Service) UserByToken(ctx context.Context, token string) (User, error) {
	var userID int64
	var role string
	var expires time.Time
	err := s.db.QueryRowContext(ctx, `SELECT user_id, active_role, expires_at FROM sessions WHERE token=$1`, token).
		Scan(&userID, &role, &expires)
	if err != nil || time.Now().After(expires) {
		return User{}, ErrUnauthorized
	}
	return s.loadUser(ctx, userID, role)
}

func (s *Service) loadUser(ctx context.Context, id int64, active string) (User, error) {
	var u User
	var birth sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT id,email,full_name,birth_date,municipality,organization,phone,rusada_completed,rusada_url,guardian_name FROM users WHERE id=$1`, id).
		Scan(&u.ID, &u.Email, &u.FullName, &birth, &u.Municipality, &u.Organization, &u.Phone, &u.Rusada, &u.RusadaURL, &u.Guardian)
	if err != nil {
		return User{}, err
	}
	if birth.Valid {
		u.BirthDate = birth.Time.Format("2006-01-02")
	}
	u.Roles, _ = s.roles(ctx, id)
	u.ActiveRole = active
	_ = s.db.QueryRowContext(ctx, `SELECT id FROM athletes WHERE user_id=$1`, id).Scan(&u.AthleteID)
	_ = s.db.QueryRowContext(ctx, `SELECT id FROM coaches WHERE user_id=$1`, id).Scan(&u.CoachID)
	_ = s.db.QueryRowContext(ctx, `SELECT id FROM judges WHERE user_id=$1`, id).Scan(&u.JudgeID)
	return u, nil
}

func (s *Service) roles(ctx context.Context, userID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT role FROM user_roles WHERE user_id=$1 ORDER BY role`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		items = append(items, role)
	}
	return items, rows.Err()
}

func TokenFrom(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	if cookie, err := r.Cookie("orenkod_token"); err == nil {
		return cookie.Value
	}
	return strings.TrimSpace(r.Header.Get("X-Auth-Token"))
}

func (s *Service) Middleware(roles ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			token := TokenFrom(r)
			user, err := s.UserByToken(r.Context(), token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, ErrUnauthorized.Error())
				return
			}
			if len(roles) > 0 {
				allowed := false
				for _, role := range roles {
					if user.ActiveRole == role {
						allowed = true
						break
					}
				}
				if !allowed {
					writeError(w, http.StatusForbidden, ErrForbidden.Error()+": активна роль «"+rules.RoleLabel(user.ActiveRole)+"»")
					return
				}
			}
			ctx := context.WithValue(r.Context(), userKey, user)
			next(w, r.WithContext(ctx))
		}
	}
}

func From(r *http.Request) User {
	user, _ := r.Context().Value(userKey).(User)
	return user
}

func randomToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// Register creates a standard account. Federation accounts are provisioned by an administrator only.
func (s *Service) Register(ctx context.Context, input Registration) error {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.FullName = strings.TrimSpace(input.FullName)
	input.Organization = strings.TrimSpace(input.Organization)
	input.Municipality = strings.TrimSpace(input.Municipality)
	input.Phone = strings.TrimSpace(input.Phone)
	input.Role = strings.TrimSpace(input.Role)
	if input.Role == "" {
		input.Role = rules.RoleAthlete
	}
	if input.Email == "" || !strings.Contains(input.Email, "@") || input.FullName == "" {
		return errors.New("Укажите корректный email и ФИО")
	}
	if len(input.Password) < 8 {
		return errors.New("Пароль должен содержать не менее 8 символов")
	}
	if input.Role != rules.RoleAthlete && input.Role != rules.RoleCoach && input.Role != rules.RoleJudge {
		return errors.New("Самостоятельная регистрация доступна только для спортсмена, тренера или судьи")
	}
	if !input.ConsentPersonal {
		return errors.New("Необходимо согласие на обработку персональных данных")
	}
	if input.Organization == "" {
		return errors.New("Укажите организацию")
	}
	if input.Municipality == "" {
		input.Municipality = "Оренбург"
	}
	if input.Role == rules.RoleAthlete {
		if input.BirthDate == "" || input.Discipline == "" || input.AgeGroup == "" {
			return errors.New("Для профиля спортсмена укажите дату рождения, дисциплину и возрастную группу")
		}
		validDiscipline := false
		for _, d := range rules.Disciplines {
			if d == input.Discipline {
				validDiscipline = true
				break
			}
		}
		if !validDiscipline {
			return errors.New("Выберите дисциплину из официального справочника")
		}
		validAge := false
		for _, a := range rules.AgeGroups {
			if a == input.AgeGroup {
				validAge = true
				break
			}
		}
		if !validAge || input.AgeGroup == "Все группы" {
			return errors.New("Выберите конкретную возрастную группу спортсмена")
		}
		birth, err := time.Parse("2006-01-02", input.BirthDate)
		if err != nil {
			return errors.New("Дата рождения должна быть в формате ГГГГ-ММ-ДД")
		}
		age := rules.AgeAt(birth, time.Now())
		if age < 14 {
			return errors.New("Регистрация спортсмена доступна с 14 лет; для иных случаев обратитесь в Федерацию")
		}
		if !rules.MatchesAgeGroup(input.AgeGroup, age) {
			return errors.New("Возраст не соответствует выбранной возрастной группе")
		}
		if age < 18 && (strings.TrimSpace(input.GuardianName) == "" || !input.GuardianConsent) {
			return errors.New("Для несовершеннолетнего спортсмена нужны ФИО законного представителя и его согласие")
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID int64
	_, err = tx.ExecContext(ctx, `INSERT INTO users(email,password_hash,full_name,birth_date,municipality,organization,phone,consent_personal,guardian_name,guardian_consent)
		VALUES($1,$2,$3,NULLIF($4,'')::date,$5,$6,$7,$8,$9,$10)`, input.Email, string(hash), input.FullName, input.BirthDate, input.Municipality, input.Organization, input.Phone, input.ConsentPersonal, strings.TrimSpace(input.GuardianName), input.GuardianConsent)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			return errors.New("Аккаунт с таким email уже существует")
		}
		return fmt.Errorf("не удалось создать аккаунт: %w", err)
	}
	if err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE lower(email)=lower($1)`, input.Email).Scan(&userID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO user_roles(user_id,role) VALUES($1,$2)`, userID, input.Role); err != nil {
		return err
	}
	switch input.Role {
	case rules.RoleCoach:
		_, err = tx.ExecContext(ctx, `INSERT INTO coaches(user_id,organization) VALUES($1,$2)`, userID, input.Organization)
	case rules.RoleJudge:
		_, err = tx.ExecContext(ctx, `INSERT INTO judges(user_id,category,organization) VALUES($1,'Кандидат на подтверждение',$2)`, userID, input.Organization)
	case rules.RoleAthlete:
		_, err = tx.ExecContext(ctx, `INSERT INTO athletes(full_name,organization,discipline,age_group,coach_name,status,user_id,birth_date,municipality,year_started,rank_title)
			VALUES($1,$2,$3,$4,'','На проверке',$5,$6::date,$7,EXTRACT(YEAR FROM CURRENT_DATE)::integer,'Без разряда')`, input.FullName, input.Organization, input.Discipline, input.AgeGroup, userID, input.BirthDate, input.Municipality)
		if err == nil {
			var athleteID int64
			if e := tx.QueryRowContext(ctx, `SELECT id FROM athletes WHERE user_id=$1`, userID).Scan(&athleteID); e == nil {
				_, err = tx.ExecContext(ctx, `INSERT INTO athlete_disciplines(athlete_id,discipline) VALUES($1,$2) ON CONFLICT DO NOTHING`, athleteID, input.Discipline)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("не удалось создать профиль роли: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO inbox(user_id,kind,title,message) VALUES($1,'info','Добро пожаловать','Аккаунт создан. Доступные действия зависят от вашей роли; данные спортсмена могут пройти проверку Федерации.')`, userID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_log(user_id,role,action,entity,details) VALUES($1,$2,'регистрация','users',$3)`, userID, input.Role, input.Email); err != nil {
		return err
	}
	return tx.Commit()
}

type Registration struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	FullName        string `json:"full_name"`
	BirthDate       string `json:"birth_date"`
	Municipality    string `json:"municipality"`
	Organization    string `json:"organization"`
	Phone           string `json:"phone"`
	Role            string `json:"role"`
	Discipline      string `json:"discipline"`
	AgeGroup        string `json:"age_group"`
	ConsentPersonal bool   `json:"consent_personal"`
	GuardianName    string `json:"guardian_name"`
	GuardianConsent bool   `json:"guardian_consent"`
	EmailCode       string `json:"email_code"`
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
