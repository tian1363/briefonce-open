package web

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tian1363/scriptagent/internal/auth"
	"github.com/tian1363/scriptagent/internal/jobs"
)

func TestPlatformCredentialIsAdministratorOnly(t *testing.T) {
	store, err := jobs.OpenStore(filepath.Join(t.TempDir(), "provider.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))); err != nil {
		t.Fatal(err)
	}
	owner, err := store.CreateUser(jobs.CreateUserInput{Email: "admin@example.com", PasswordHash: "hash", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetSoleAdministrator(owner.ID); err != nil {
		t.Fatal(err)
	}
	member, err := store.CreateUser(jobs.CreateUserInput{Email: "member@example.com", PasswordHash: "hash", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	ownerSession, err := store.CreateSession(jobs.CreateSessionInput{Token: "owner-token", UserID: owner.ID, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	memberSession, err := store.CreateSession(jobs.CreateSessionInput{Token: "member-token", UserID: member.ID, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(Config{AdminUserID: owner.ID, AllowManagedMode: true}, store, nil, nil, nil, nil, nil)
	for _, tc := range []struct {
		token string
		want  int
	}{{memberSession.Token, 403}, {ownerSession.Token, 200}} {
		req := httptest.NewRequest(http.MethodPut, "/api/owner/provider", strings.NewReader(`{"api_key":"new-platform-secret"}`))
		req.AddCookie(&http.Cookie{Name: auth.CookieName(), Value: tc.token})
		out := httptest.NewRecorder()
		h.Routes().ServeHTTP(out, req)
		if out.Code != tc.want {
			t.Fatalf("status=%d response=%s", out.Code, out.Body.String())
		}
		if strings.Contains(out.Body.String(), "new-platform-secret") {
			t.Fatal("platform secret exposed in response")
		}
	}
	for _, tc := range []struct {
		token string
		want  int
	}{{memberSession.Token, 403}, {ownerSession.Token, 200}} {
		body := `{"provider":"zexapi","model":"seedance-2.0","endpoint":"https://zexapi.com/v1/videos","api_key":"capability-secret"}`
		req := httptest.NewRequest(http.MethodPut, "/api/owner/provider/profiles/video_generation", strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: auth.CookieName(), Value: tc.token})
		out := httptest.NewRecorder()
		h.Routes().ServeHTTP(out, req)
		if out.Code != tc.want || strings.Contains(out.Body.String(), "capability-secret") {
			t.Fatalf("profile status=%d response=%s", out.Code, out.Body.String())
		}
	}
}

func TestAdministratorCanProvisionAccountAndMemberCannotManageCredits(t *testing.T) {
	t.Setenv("DASHSCOPE_API_KEY", "test-platform-key")
	store, err := jobs.OpenStore(filepath.Join(t.TempDir(), "accounts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))); err != nil {
		t.Fatal(err)
	}
	owner, err := store.CreateUser(jobs.CreateUserInput{Email: "owner@example.com", Status: "active", PasswordHash: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetSoleAdministrator(owner.ID); err != nil {
		t.Fatal(err)
	}
	ownerSession, err := store.CreateSession(jobs.CreateSessionInput{Token: "owner-token", UserID: owner.ID, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(Config{AdminUserID: owner.ID, AllowManagedMode: true, RegistrationMode: "closed"}, store, nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/owner/accounts", strings.NewReader(`{"email":"trial@example.com","name":"Trial","trial_days":7,"credits":12,"daily_limit":5}`))
	request.AddCookie(&http.Cookie{Name: auth.CookieName(), Value: ownerSession.Token})
	response := httptest.NewRecorder()
	h.Routes().ServeHTTP(response, request)
	if response.Code != 201 {
		t.Fatalf("provision status %d: %s", response.Code, response.Body.String())
	}
	var created struct {
		User     jobs.User          `json:"user"`
		Credit   jobs.AccountCredit `json:"credit"`
		Password string             `json:"temporary_password"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Password == "" || created.Credit.Balance != 12 || !created.Credit.ManagedEnabled {
		t.Fatalf("unexpected account: %+v", created)
	}
	if strings.Contains(response.Body.String(), "password_hash") {
		t.Fatal("password hash exposed")
	}

	login := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"trial@example.com","password":"`+created.Password+`"}`))
	loginResponse := httptest.NewRecorder()
	h.Routes().ServeHTTP(loginResponse, login)
	if loginResponse.Code != 200 {
		t.Fatalf("new account cannot log in: %s", loginResponse.Body.String())
	}
	memberCookie := loginResponse.Result().Cookies()[0]
	creditRequest := httptest.NewRequest(http.MethodGet, "/api/credits", nil)
	creditRequest.AddCookie(memberCookie)
	creditResponse := httptest.NewRecorder()
	h.Routes().ServeHTTP(creditResponse, creditRequest)
	if creditResponse.Code != 200 || !strings.Contains(creditResponse.Body.String(), `"balance":12`) || !strings.Contains(creditResponse.Body.String(), `"daily_used":0`) {
		t.Fatalf("credits unavailable: %d %s", creditResponse.Code, creditResponse.Body.String())
	}
	settingsRequest := httptest.NewRequest(http.MethodGet, "/api/settings/model", nil)
	settingsRequest.AddCookie(memberCookie)
	settingsResponse := httptest.NewRecorder()
	h.Routes().ServeHTTP(settingsResponse, settingsRequest)
	if settingsResponse.Code != 200 || !strings.Contains(settingsResponse.Body.String(), `"mode":"managed"`) || strings.Contains(settingsResponse.Body.String(), "test-platform-key") {
		t.Fatalf("trial account model status is wrong: %d %s", settingsResponse.Code, settingsResponse.Body.String())
	}
	personalKeyRequest := httptest.NewRequest(http.MethodPut, "/api/settings/model", strings.NewReader(`{"profiles":[{"capability":"text","mode":"byok","api_key":"trial-private-key"}]}`))
	personalKeyRequest.AddCookie(memberCookie)
	personalKeyResponse := httptest.NewRecorder()
	h.Routes().ServeHTTP(personalKeyResponse, personalKeyRequest)
	if personalKeyResponse.Code != 403 {
		t.Fatalf("trial account could change API key: %d", personalKeyResponse.Code)
	}
	if err := store.SavePlatformModelProfile(jobs.PlatformModelInput{Capability: "video_generation", Provider: "zexapi", Model: "seedance-2.0", Endpoint: "https://zexapi.com/v1/videos", APIKey: "video-key"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVideoPrice(jobs.ImagePrice{Model: "seedance-2.0", Tier: "default:standard", Enabled: true, CostMicros: 100000, MultiplierBPS: 20000, PointMicros: 10000}, owner.ID, "supplier price"); err != nil {
		t.Fatal(err)
	}
	videoRequest := httptest.NewRequest(http.MethodPost, "/api/videos", strings.NewReader(`{"prompt":"测试","mode":"text","model":"seedance-2.0","resolution":"default","duration":5}`))
	videoRequest.AddCookie(memberCookie)
	videoResponse := httptest.NewRecorder()
	h.Routes().ServeHTTP(videoResponse, videoRequest)
	if videoResponse.Code != http.StatusPaymentRequired || !strings.Contains(videoResponse.Body.String(), "积分不足") {
		t.Fatalf("trial video pricing was not checked before dispatch: %d %s", videoResponse.Code, videoResponse.Body.String())
	}
	accountsRequest := httptest.NewRequest(http.MethodGet, "/api/owner/accounts", nil)
	accountsRequest.AddCookie(&http.Cookie{Name: auth.CookieName(), Value: ownerSession.Token})
	accountsResponse := httptest.NewRecorder()
	h.Routes().ServeHTTP(accountsResponse, accountsRequest)
	if accountsResponse.Code != 200 || !strings.Contains(accountsResponse.Body.String(), `"daily_used":0`) {
		t.Fatalf("admin usage unavailable: %d %s", accountsResponse.Code, accountsResponse.Body.String())
	}
	change := httptest.NewRequest(http.MethodPost, "/api/owner/accounts/"+created.User.ID+"/credits", strings.NewReader(`{"amount":100}`))
	change.AddCookie(memberCookie)
	changeResponse := httptest.NewRecorder()
	h.Routes().ServeHTTP(changeResponse, change)
	if changeResponse.Code != 403 {
		t.Fatalf("member changed credits: %d", changeResponse.Code)
	}
}
