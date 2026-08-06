package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"organizing-app-backend/internal/model"
	"organizing-app-backend/internal/service"
)

const sessionCookie = "tidy_session"

type contextKey string

const userContextKey contextKey = "authenticatedUser"

// AuthController handles signup, login, logout, and the session cookie.
// The cookie is HttpOnly (no JS access) and SameSite=Lax (CSRF mitigation);
// Secure is set whenever the request arrived over HTTPS.
type AuthController struct {
	service *service.AuthService
}

func NewAuthController(s *service.AuthService) *AuthController {
	return &AuthController{service: s}
}

type credentialsBody struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (c *AuthController) Signup(w http.ResponseWriter, r *http.Request) {
	var body credentialsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	user, err := c.service.Signup(r.Context(), body.Name, body.Email, body.Password)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, service.ErrInvalidSignupInput):
			status = http.StatusBadRequest
		case errors.Is(err, service.ErrEmailTaken):
			status = http.StatusConflict
		}
		writeError(w, status, err)
		return
	}

	if err := c.startSession(w, r, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (c *AuthController) Login(w http.ResponseWriter, r *http.Request) {
	var body credentialsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	user, err := c.service.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if err := c.startSession(w, r, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (c *AuthController) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if err := c.service.DeleteSession(r.Context(), cookie.Value); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPS(r),
	})
	w.WriteHeader(http.StatusNoContent)
}

func (c *AuthController) Me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, CurrentUser(r.Context()))
}

// RequireAuth resolves the session cookie to a user and stores it in the
// request context, rejecting the request with 401 otherwise.
func (c *AuthController) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, errors.New("not signed in"))
			return
		}
		user, err := c.service.UserForToken(r.Context(), cookie.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, errors.New("not signed in"))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

// CurrentUser returns the authenticated user placed in the context by
// RequireAuth; the zero User if the middleware did not run.
func CurrentUser(ctx context.Context) model.User {
	user, _ := ctx.Value(userContextKey).(model.User)
	return user
}

func (c *AuthController) startSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	token, expires, err := c.service.CreateSession(r.Context(), userID)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPS(r),
	})
	return nil
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
