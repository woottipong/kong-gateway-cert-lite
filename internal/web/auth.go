package web

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	authCookieName  = "kong_cert_lite_session"
	authSessionTTL  = 12 * time.Hour
	loginTemplate   = "templates/login.html"
	defaultLoginURL = "/login"
)

type BasicAuthConfig struct {
	Username string
	Password string
}

type LoginPage struct {
	Title      string
	ReturnTo   string
	Error      string
	HasAuth    bool
	Username   string
	CurrentUTC string
}

type sessionPayload struct {
	Username  string `json:"username"`
	ExpiresAt int64  `json:"expires_at"`
}

func (c BasicAuthConfig) Enabled() bool {
	return strings.TrimSpace(c.Username) != "" && c.Password != ""
}

func (c BasicAuthConfig) normalizedUsername() string {
	return strings.TrimSpace(c.Username)
}

func (c BasicAuthConfig) signingKey() []byte {
	sum := sha256.Sum256([]byte(c.normalizedUsername() + "\x00" + c.Password))
	return sum[:]
}

func NewSessionAuthMiddleware(cfg BasicAuthConfig) fiber.Handler {
	if !cfg.Enabled() {
		return func(c *fiber.Ctx) error {
			return c.Next()
		}
	}

	return func(c *fiber.Ctx) error {
		if validSessionCookie(cfg, c.Cookies(authCookieName), time.Now()) {
			return c.Next()
		}

		if wantsHTML(c) {
			returnTo := c.OriginalURL()
			if returnTo == "" || strings.HasPrefix(returnTo, defaultLoginURL) {
				returnTo = "/certificates"
			}
			return c.Redirect(defaultLoginURL+"?return_to="+url.QueryEscape(returnTo), fiber.StatusSeeOther)
		}

		return c.SendStatus(fiber.StatusUnauthorized)
	}
}

func LoginPageHandler(cfg BasicAuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if cfg.Enabled() && validSessionCookie(cfg, c.Cookies(authCookieName), time.Now()) {
			return c.Redirect(safeReturnTo(c.Query("return_to")), fiber.StatusSeeOther)
		}

		return renderLogin(c, LoginPage{
			Title:      "Login",
			ReturnTo:   safeReturnTo(c.Query("return_to")),
			HasAuth:    cfg.Enabled(),
			CurrentUTC: time.Now().UTC().Format("15:04 UTC"),
		})
	}
}

func LoginPostHandler(cfg BasicAuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		returnTo := safeReturnTo(c.FormValue("return_to"))
		username := strings.TrimSpace(c.FormValue("username"))
		password := c.FormValue("password")

		if !cfg.Enabled() {
			return c.Redirect(returnTo, fiber.StatusSeeOther)
		}

		if !credentialsMatch(cfg, username, password) {
			return renderLogin(c.Status(fiber.StatusUnauthorized), LoginPage{
				Title:      "Login",
				ReturnTo:   returnTo,
				Error:      "Username or password is incorrect.",
				HasAuth:    true,
				Username:   username,
				CurrentUTC: time.Now().UTC().Format("15:04 UTC"),
			})
		}

		setSessionCookie(c, cfg, time.Now())
		return c.Redirect(returnTo, fiber.StatusSeeOther)
	}
}

func LogoutHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:     authCookieName,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HTTPOnly: true,
			SameSite: fiber.CookieSameSiteLaxMode,
			Secure:   c.Protocol() == "https",
		})
		return c.Redirect(defaultLoginURL, fiber.StatusSeeOther)
	}
}

func credentialsMatch(cfg BasicAuthConfig, username string, password string) bool {
	expectedUsername := sha256.Sum256([]byte(cfg.normalizedUsername()))
	expectedPassword := sha256.Sum256([]byte(cfg.Password))
	actualUsername := sha256.Sum256([]byte(username))
	actualPassword := sha256.Sum256([]byte(password))
	usernameMatches := subtle.ConstantTimeCompare(actualUsername[:], expectedUsername[:]) == 1
	passwordMatches := subtle.ConstantTimeCompare(actualPassword[:], expectedPassword[:]) == 1
	return usernameMatches && passwordMatches
}

func setSessionCookie(c *fiber.Ctx, cfg BasicAuthConfig, now time.Time) {
	expires := now.Add(authSessionTTL).UTC()
	c.Cookie(&fiber.Cookie{
		Name:     authCookieName,
		Value:    createSessionToken(cfg, expires),
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(authSessionTTL.Seconds()),
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
		Secure:   c.Protocol() == "https",
	})
}

func createSessionToken(cfg BasicAuthConfig, expires time.Time) string {
	payloadBytes, err := json.Marshal(sessionPayload{
		Username:  cfg.normalizedUsername(),
		ExpiresAt: expires.Unix(),
	})
	if err != nil {
		return ""
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	mac := hmac.New(sha256.New, cfg.signingKey())
	_, _ = mac.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + signature
}

func validSessionCookie(cfg BasicAuthConfig, token string, now time.Time) bool {
	if !cfg.Enabled() || token == "" {
		return false
	}

	payload, signature, ok := strings.Cut(token, ".")
	if !ok || payload == "" || signature == "" {
		return false
	}

	mac := hmac.New(sha256.New, cfg.signingKey())
	_, _ = mac.Write([]byte(payload))
	expectedSignature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(signature), []byte(expectedSignature)) != 1 {
		return false
	}

	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return false
	}
	var session sessionPayload
	if err := json.Unmarshal(decoded, &session); err != nil {
		return false
	}
	if !now.Before(time.Unix(session.ExpiresAt, 0)) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(session.Username), []byte(cfg.normalizedUsername())) == 1
}

func renderLogin(c *fiber.Ctx, data LoginPage) error {
	tmpl, err := template.ParseFS(templateFiles, loginTemplate)
	if err != nil {
		return err
	}

	var body bytes.Buffer
	if err := tmpl.ExecuteTemplate(&body, "login", data); err != nil {
		return err
	}

	c.Type("html", "utf-8")
	return c.Send(body.Bytes())
}

func wantsHTML(c *fiber.Ctx) bool {
	accept := c.Get(fiber.HeaderAccept)
	return accept == "" || strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*")
}

func safeReturnTo(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.HasPrefix(value, "/login") {
		return "/certificates"
	}
	return value
}
