package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"

	"organizing-app-backend/internal/model"
)

var (
	ErrEmailTaken         = errors.New("an account with this email already exists")
	ErrInvalidCredentials = errors.New("incorrect email or password")
	ErrSessionInvalid     = errors.New("session is invalid or expired")
	// ErrInvalidSignupInput marks a Signup argument-validation failure (bad
	// name/email/password), surfaced by the controller as 400. Errors
	// returned for a specific field still carry their own message but all
	// satisfy errors.Is(err, ErrInvalidSignupInput).
	ErrInvalidSignupInput = errors.New("invalid signup input")
)

// signupValidationError carries a message specific to the validation
// failure while still matching errors.Is(err, ErrInvalidSignupInput). Mirrors
// duplicateNameError in category_service.go.
type signupValidationError struct{ message string }

func (e *signupValidationError) Error() string        { return e.message }
func (e *signupValidationError) Is(target error) bool { return target == ErrInvalidSignupInput }

func newSignupValidationError(message string) error {
	return &signupValidationError{message: message}
}

// Argon2id parameters per OWASP's password storage recommendation
// (m=19 MiB, t=2, p=1).
const (
	argonMemoryKiB   = 19 * 1024
	argonIterations  = 2
	argonParallelism = 1
	argonSaltLen     = 16
	argonKeyLen      = 32

	sessionTTL = 30 * 24 * time.Hour
)

// AuthService owns user credentials and sessions. Passwords are stored only
// as argon2id hashes; session cookies carry an opaque random token whose
// SHA-256 (never the token itself) is stored server-side.
type AuthService struct {
	db *pgxpool.Pool
}

func NewAuthService(db *pgxpool.Pool) *AuthService {
	return &AuthService{db: db}
}

// Signup creates a user with an argon2id-hashed password, seeds their own
// copy of the default categories, and returns the user. The user insert and
// the seeding run in one transaction, so a seeding failure leaves no user
// row behind and the email is free to retry immediately.
func (s *AuthService) Signup(ctx context.Context, name, email, password string) (model.User, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	if name == "" || !strings.Contains(email, "@") {
		return model.User{}, newSignupValidationError("name and a valid email are required")
	}
	if len(password) < 8 {
		return model.User{}, newSignupValidationError("password must be at least 8 characters")
	}

	hash, err := hashPassword(password)
	if err != nil {
		return model.User{}, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return model.User{}, err
	}
	defer tx.Rollback(ctx)

	var u model.User
	err = tx.QueryRow(
		ctx,
		`INSERT INTO Users (name, email, passwordHash, createdAt, updatedAt)
		 VALUES ($1, $2, $3, now(), now())
		 RETURNING id, name, profileImageURL, currency, createdAt, updatedAt`,
		name, email, hash,
	).Scan(&u.ID, &u.Name, &u.ProfileImageURL, &u.Currency, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "users_email_unique") {
			return model.User{}, ErrEmailTaken
		}
		return model.User{}, err
	}

	if err := seedDefaultCategories(ctx, tx, u.ID); err != nil {
		return model.User{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return model.User{}, err
	}
	return u, nil
}

// Login verifies the credentials and returns the matching user.
func (s *AuthService) Login(ctx context.Context, email, password string) (model.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	var (
		u    model.User
		hash sql.NullString
	)
	err := s.db.QueryRow(
		ctx,
		`SELECT id, name, profileImageURL, currency, createdAt, updatedAt, passwordHash
		 FROM Users WHERE lower(email) = $1`,
		email,
	).Scan(&u.ID, &u.Name, &u.ProfileImageURL, &u.Currency, &u.CreatedAt, &u.UpdatedAt, &hash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !hash.Valid) {
		// Burn a hash anyway so a missing account takes as long as a wrong
		// password (limits user enumeration via timing).
		verifyPassword(password, dummyHash)
		return model.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return model.User{}, err
	}

	ok, err := verifyPassword(password, hash.String)
	if err != nil {
		return model.User{}, err
	}
	if !ok {
		return model.User{}, ErrInvalidCredentials
	}
	return u, nil
}

// CreateSession mints an opaque token for the user, stores its SHA-256 hash,
// and returns the raw token (only ever held by the client) with its expiry.
func (s *AuthService) CreateSession(ctx context.Context, userID int64) (token string, expires time.Time, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	expires = time.Now().Add(sessionTTL)

	_, err = s.db.Exec(
		ctx,
		`INSERT INTO Sessions (userId, tokenHash, expiresAt) VALUES ($1, $2, $3)`,
		userID, hashToken(token), expires,
	)
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

// UserForToken resolves a session token to its user, rejecting expired sessions.
func (s *AuthService) UserForToken(ctx context.Context, token string) (model.User, error) {
	if token == "" {
		return model.User{}, ErrSessionInvalid
	}

	var u model.User
	err := s.db.QueryRow(
		ctx,
		`SELECT u.id, u.name, u.profileImageURL, u.currency, u.createdAt, u.updatedAt
		 FROM Sessions s JOIN Users u ON u.id = s.userId
		 WHERE s.tokenHash = $1 AND s.expiresAt > now()`,
		hashToken(token),
	).Scan(&u.ID, &u.Name, &u.ProfileImageURL, &u.Currency, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, ErrSessionInvalid
	}
	return u, err
}

// DeleteSession revokes one session (logout). Unknown tokens are a no-op.
func (s *AuthService) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM Sessions WHERE tokenHash = $1`, hashToken(token))
	return err
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// hashPassword derives an argon2id hash and encodes it in PHC string format,
// so the parameters and salt travel with the hash.
func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemoryKiB, argonParallelism, argonKeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonIterations, argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// verifyPassword re-derives the key using the parameters stored in the PHC
// string and compares in constant time.
func verifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("unsupported password hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, err
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}

	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// dummyHash is a valid hash used only to equalize login timing for unknown
// emails; computed once at startup.
var dummyHash = func() string {
	hash, _ := hashPassword("dummy-password-for-timing")
	return hash
}()
