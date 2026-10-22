package app

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
)

// User is the single account. It owns one employee record.
type User struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Theme       string `json:"theme" enum:"dark,light,system"`
	OIDCLinked  bool   `json:"oidcLinked"`
	HasPassword bool   `json:"hasPassword"`
	EmployeeID  int64  `json:"-"`
}

const userCols = `u.id, coalesce(u.username, ''), u.name, u.email, u.theme, u.oidc_subject IS NOT NULL, u.password_hash IS NOT NULL, e.id`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.Theme, &u.OIDCLinked, &u.HasPassword, &u.EmployeeID); err != nil {
		return nil, notFound(err)
	}
	return &u, nil
}

// UserByID loads a user.
func (a *App) UserByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(a.DB.QueryRowContext(ctx, `SELECT `+userCols+` FROM users u JOIN employees e ON e.user_id = u.id WHERE u.id = ?`, id))
}

// NeedsSetup reports whether the account does not exist yet.
func (a *App) NeedsSetup(ctx context.Context) (bool, error) {
	var n int
	err := a.DB.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n == 0, err
}

// PasswordPolicy validates a new password.
func PasswordPolicy(pw string) error {
	if len([]rune(pw)) < 10 {
		return Invalid("password.too_short", "password must have at least 10 characters")
	}
	return nil
}

// createAccount inserts the user with its company and employee records.
func (a *App) createAccount(ctx context.Context, tx *sql.Tx, cols string, args ...any) (int64, error) {
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil {
		return 0, err
	}
	if n > 0 {
		return 0, &Error{Status: 409, Code: "setup.done", Message: "the account already exists"}
	}
	now := a.stamp()
	marks := strings.Repeat("?, ", len(args)) + "?"
	res, err := tx.ExecContext(ctx, `INSERT INTO users(`+cols+`, created_at) VALUES (`+marks+`)`, append(args, now)...)
	if err != nil {
		return 0, err
	}
	userID, _ := res.LastInsertId()
	res, err = tx.ExecContext(ctx, `INSERT INTO companies(created_at) VALUES (?)`, now)
	if err != nil {
		return 0, err
	}
	companyID, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx, `INSERT INTO employees(company_id, user_id, name, created_at) SELECT ?, id, name, ? FROM users WHERE id = ?`, companyID, now, userID); err != nil {
		return 0, err
	}
	return userID, a.audit(ctx, tx, userID, "user", userID, "create", "", nil)
}

// Setup creates the account with a password. It is single-use and requires
// the operator's setup token.
func (a *App) Setup(ctx context.Context, token, wantToken, username, name, password string) (*User, error) {
	if wantToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(wantToken)) != 1 {
		return nil, Invalid("setup.token_invalid", "setup token is invalid")
	}
	username = strings.TrimSpace(strings.ToLower(username))
	name = strings.TrimSpace(name)
	if username == "" || name == "" {
		return nil, Invalid("user.required", "username and name are required")
	}
	if err := PasswordPolicy(password); err != nil {
		return nil, err
	}
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return nil, err
	}
	var id int64
	err = a.tx(ctx, func(tx *sql.Tx) error {
		id, err = a.createAccount(ctx, tx, "username, name, password_hash", username, name, hash)
		return err
	})
	if err != nil {
		return nil, err
	}
	return a.UserByID(ctx, id)
}

// ProfileInput updates the user's own profile.
type ProfileInput struct {
	Name  string `json:"name" minLength:"1" maxLength:"200"`
	Email string `json:"email" maxLength:"200" required:"false"`
	Theme string `json:"theme" enum:"dark,light,system"`
}

// UpdateProfile saves profile fields.
func (a *App) UpdateProfile(ctx context.Context, u *User, in ProfileInput) (*User, error) {
	err := a.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET name = ?, email = ?, theme = ? WHERE id = ?`, strings.TrimSpace(in.Name), strings.TrimSpace(in.Email), in.Theme, u.ID); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, "user", u.ID, "profile", "", nil)
	})
	if err != nil {
		return nil, err
	}
	return a.UserByID(ctx, u.ID)
}

// ChangePassword sets a new password; an existing one must be confirmed.
func (a *App) ChangePassword(ctx context.Context, u *User, current, next string) error {
	var hash sql.NullString
	if err := a.DB.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, u.ID).Scan(&hash); err != nil {
		return err
	}
	if hash.Valid {
		if ok, err := argon2id.ComparePasswordAndHash(current, hash.String); err != nil || !ok {
			return Invalid("password.wrong", "current password is wrong")
		}
	}
	if err := PasswordPolicy(next); err != nil {
		return err
	}
	h, err := argon2id.CreateHash(next, argon2id.DefaultParams)
	if err != nil {
		return err
	}
	return a.tx(ctx, func(tx *sql.Tx) error {
		// A user without a username (OIDC-created) gets one so password login works.
		if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ?, username = coalesce(username, lower(CASE WHEN email <> '' THEN email ELSE 'admin' END)) WHERE id = ?`, h, u.ID); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, "user", u.ID, "password", "", nil)
	})
}

const (
	loginWindow      = 15 * time.Minute
	loginMaxFailures = 5
	// SessionTTL is the lifetime of a login session.
	SessionTTL = 30 * 24 * time.Hour
)

// ErrTooManyAttempts limits password guessing.
var ErrTooManyAttempts = &Error{Status: 429, Code: "login.rate_limited", Message: "too many failed attempts; try again later"}

// dummyHash keeps timing similar for unknown usernames.
var dummyHash, _ = argon2id.CreateHash("dummy-password-for-timing", argon2id.DefaultParams)

// PasswordLogin verifies credentials, limited per username and client.
func (a *App) PasswordLogin(ctx context.Context, username, password, client string) (*User, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	keys := []string{"u:" + username, "c:" + client}
	since := a.now().Add(-loginWindow).UTC().Format(time.RFC3339)
	for _, k := range keys {
		var n int
		if err := a.DB.QueryRowContext(ctx, `SELECT count(*) FROM login_failures WHERE key = ? AND at > ?`, k, since).Scan(&n); err != nil {
			return nil, err
		}
		if n >= loginMaxFailures {
			return nil, ErrTooManyAttempts
		}
	}
	var id int64
	var hash sql.NullString
	err := a.DB.QueryRowContext(ctx, `SELECT id, password_hash FROM users WHERE username = ?`, username).Scan(&id, &hash)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	h := dummyHash
	if hash.Valid {
		h = hash.String
	}
	ok, _ := argon2id.ComparePasswordAndHash(password, h)
	if !ok || !hash.Valid {
		for _, k := range keys {
			if _, err := a.DB.ExecContext(ctx, `INSERT INTO login_failures(key, at) VALUES (?, ?)`, k, a.stamp()); err != nil {
				return nil, err
			}
		}
		return nil, &Error{Status: 401, Code: "login.invalid", Message: "username or password is wrong"}
	}
	if _, err := a.DB.ExecContext(ctx, `DELETE FROM login_failures WHERE key = ? OR at < ?`, keys[0], since); err != nil {
		return nil, err
	}
	return a.UserByID(ctx, id)
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateSession starts a session and returns its opaque token.
func (a *App) CreateSession(ctx context.Context, userID int64, method string) (string, error) {
	token := base64.RawURLEncoding.EncodeToString([]byte(randomHex(32)))
	now := a.now().UTC()
	return token, a.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, now.Format(time.RFC3339)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO sessions(token_hash, user_id, method, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
			tokenHash(token), userID, method, now.Format(time.RFC3339), now.Add(SessionTTL).Format(time.RFC3339)); err != nil {
			return err
		}
		return a.audit(ctx, tx, userID, "session", userID, "login", "", map[string]any{"method": method})
	})
}

// SessionUser resolves a session token to the user.
func (a *App) SessionUser(ctx context.Context, token string) (*User, error) {
	if token == "" {
		return nil, ErrUnauthorized
	}
	u, err := scanUser(a.DB.QueryRowContext(ctx, `SELECT `+userCols+` FROM sessions s JOIN users u ON u.id = s.user_id JOIN employees e ON e.user_id = u.id WHERE s.token_hash = ? AND s.expires_at > ?`,
		tokenHash(token), a.stamp()))
	if err != nil {
		return nil, ErrUnauthorized
	}
	return u, nil
}

// EndSession deletes a session (logout).
func (a *App) EndSession(ctx context.Context, token string) error {
	_, err := a.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash(token))
	return err
}

// Account returns the single account, for background jobs.
func (a *App) Account(ctx context.Context) (*User, error) {
	return scanUser(a.DB.QueryRowContext(ctx, `SELECT `+userCols+` FROM users u JOIN employees e ON e.user_id = u.id ORDER BY u.id LIMIT 1`))
}
