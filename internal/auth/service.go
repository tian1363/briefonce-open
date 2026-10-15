package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"crypto/pbkdf2"

	"github.com/tian1363/scriptagent/internal/jobs"
)

const (
	sessionCookieName = "scriptagent_session"
	passwordIters     = 210000
	passwordKeyLen    = 32
)

type Config struct {
	RegistrationMode string
	InviteCodes      []string
}

type Service struct {
	store            *jobs.Store
	registrationMode string
	initErr          error
}

func NewService(store *jobs.Store, cfg Config) *Service {
	mode := strings.ToLower(strings.TrimSpace(cfg.RegistrationMode))
	if mode != "open" && mode != "closed" {
		mode = "invite"
	}
	s := &Service{store: store, registrationMode: mode}
	for _, code := range cfg.InviteCodes {
		code = strings.TrimSpace(code)
		if code != "" {
			if err := store.AddRegistrationInvite(inviteHash(code), 1); err != nil {
				s.initErr = err
			}
		}
	}
	return s
}
func CookieName() string { return sessionCookieName }

func (s *Service) RegistrationAvailable() (bool, error) {
	if s.initErr != nil {
		return false, s.initErr
	}
	count, err := s.store.CountUsers()
	if err != nil || count == 0 {
		return count == 0, err
	}
	if s.registrationMode == "open" {
		return true, nil
	}
	if s.registrationMode == "invite" {
		return s.store.HasAvailableRegistrationInvite()
	}
	return false, nil
}

func (s *Service) Register(email, password, name, inviteCode string) (*jobs.User, *jobs.Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, nil, errors.New("请输入邮箱")
	}
	address, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(address.Address, email) || len(email) > 254 {
		return nil, nil, errors.New("请输入有效邮箱")
	}
	if len(password) < 8 {
		return nil, nil, errors.New("密码至少需要 8 位")
	}
	existingCount, err := s.store.CountUsers()
	if err != nil {
		return nil, nil, err
	}
	useInvite := false
	if existingCount > 0 {
		switch s.registrationMode {
		case "closed":
			return nil, nil, errors.New("当前实例未开放注册")
		case "invite":
			useInvite = true
		}
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, nil, err
	}
	// Registration never grants administrative privileges, including bootstrap.
	userInput := jobs.CreateUserInput{Email: email, Name: strings.TrimSpace(name), Role: "member", Status: "active", PasswordHash: hash}
	var user *jobs.User
	if useInvite {
		user, err = s.store.CreateUserWithInvite(userInput, inviteHash(strings.TrimSpace(inviteCode)))
	} else {
		user, err = s.store.CreateUser(userInput)
	}
	if err != nil {
		return nil, nil, err
	}
	if existingCount == 0 {
		if err := s.store.AdoptLegacyData(user.ID); err != nil {
			return nil, nil, err
		}
	}
	session, err := s.createSession(user.ID)
	return user, session, err
}

func inviteHash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (s *Service) Login(email, password string) (*jobs.User, *jobs.Session, error) {
	user, err := s.store.GetUserByEmail(strings.ToLower(strings.TrimSpace(email)))
	if err != nil || !checkPassword(user.PasswordHash, password) {
		return nil, nil, errors.New("邮箱或密码不正确")
	}
	if user.Status != "active" {
		return nil, nil, errors.New("账号已停用")
	}
	session, err := s.createSession(user.ID)
	return user, session, err
}

// ProvisionManagedAccount creates a member with a one-time temporary password.
// Only an administrator route may call this method.
func (s *Service) ProvisionManagedAccount(email, name string, trialDays int, credits, dailyLimit int64, actorID string) (*jobs.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(address.Address, email) || len(email) > 254 {
		return nil, "", errors.New("请输入有效邮箱")
	}
	if trialDays < 1 || trialDays > 365 {
		return nil, "", errors.New("试用期限需为 1 到 365 天")
	}
	if dailyLimit == 0 {
		dailyLimit = 1000
	}
	trialEnds := time.Now().UTC().Add(time.Duration(trialDays) * 24 * time.Hour).Format(time.RFC3339)
	if err := jobs.ValidateManagedAccount(trialEnds, credits, dailyLimit); err != nil {
		return nil, "", err
	}
	password, err := randomToken(18)
	if err != nil {
		return nil, "", err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, "", err
	}
	user, err := s.store.CreateManagedUser(jobs.CreateUserInput{Email: email, Name: name, PasswordHash: hash}, trialEnds, credits, dailyLimit, actorID)
	if err != nil {
		return nil, "", err
	}
	return user, password, nil
}

func (s *Service) ResetPassword(userID string) (string, error) {
	password, err := randomToken(18)
	if err != nil {
		return "", err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return "", err
	}
	return password, s.store.UpdateUserPassword(userID, hash)
}

func (s *Service) Authenticate(token string) (*jobs.User, error) {
	session, err := s.store.GetSession(strings.TrimSpace(token))
	if err != nil {
		return nil, errors.New("登录已失效")
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		_ = s.store.DeleteSession(token)
		return nil, errors.New("登录已过期")
	}
	user, err := s.store.GetUser(session.UserID)
	if err != nil || user.Status != "active" {
		return nil, errors.New("账号不可用")
	}
	return user, nil
}

func (s *Service) Logout(token string) error { return s.store.DeleteSession(strings.TrimSpace(token)) }

func (s *Service) createSession(userID string) (*jobs.Session, error) {
	token, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	return s.store.CreateSession(jobs.CreateSessionInput{UserID: userID, Token: token, ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour)})
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, passwordIters, passwordKeyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", passwordIters, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func checkPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}
	var iterations int
	if _, err := fmt.Sscanf(parts[1], "%d", &iterations); err != nil || iterations < 1 {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[2])
	expected, err2 := base64.RawStdEncoding.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(expected))
	return err == nil && subtle.ConstantTimeCompare(key, expected) == 1
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
