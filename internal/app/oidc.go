package app

import (
	"context"
	"database/sql"
	"slices"
	"strings"
)

// OIDCIdentity is a verified identity from the OIDC provider. The stable
// key is (Issuer, Subject), never the email.
type OIDCIdentity struct {
	Issuer  string
	Subject string
	Name    string
	Email   string
	Groups  []string
}

// OIDCPolicy says which identities may become the account's login. An empty
// policy allows only an identity that is already linked.
type OIDCPolicy struct {
	AllowedGroups   []string
	AllowedEmails   []string
	AllowedSubjects []string
}

func (p OIDCPolicy) allows(id OIDCIdentity) bool {
	for _, g := range id.Groups {
		if slices.Contains(p.AllowedGroups, g) {
			return true
		}
	}
	return slices.Contains(p.AllowedSubjects, id.Subject) ||
		(id.Email != "" && slices.ContainsFunc(p.AllowedEmails, func(e string) bool { return strings.EqualFold(e, id.Email) }))
}

// OIDCLogin resolves an identity to the account. An allowed identity creates
// the account when none exists, or is linked to an account that has no
// identity yet.
func (a *App) OIDCLogin(ctx context.Context, id OIDCIdentity, p OIDCPolicy) (*User, error) {
	var userID int64
	err := a.tx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE oidc_issuer = ? AND oidc_subject = ?`, id.Issuer, id.Subject).Scan(&userID)
		if err != sql.ErrNoRows {
			return err
		}
		if !p.allows(id) {
			return &Error{Status: 403, Code: "oidc.not_allowed", Message: "this identity may not log in"}
		}
		var linked bool
		err = tx.QueryRowContext(ctx, `SELECT id, oidc_subject IS NOT NULL FROM users`).Scan(&userID, &linked)
		switch {
		case err == sql.ErrNoRows:
			name := id.Name
			if name == "" {
				name = id.Email
			}
			userID, err = a.createAccount(ctx, tx, "name, email, oidc_issuer, oidc_subject", name, id.Email, id.Issuer, id.Subject)
			return err
		case err != nil:
			return err
		case linked:
			return &Error{Status: 403, Code: "oidc.other_identity", Message: "the account is linked to another identity"}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET oidc_issuer = ?, oidc_subject = ? WHERE id = ?`, id.Issuer, id.Subject, userID); err != nil {
			return err
		}
		return a.audit(ctx, tx, userID, "user", userID, "oidc_link", "", map[string]any{"issuer": id.Issuer})
	})
	if err != nil {
		return nil, err
	}
	return a.UserByID(ctx, userID)
}
