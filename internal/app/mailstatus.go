package app

import "sync"

// MailStatus is the state of the mail import shown in the settings.
type MailStatus struct {
	Enabled   bool   `json:"enabled"`
	Mailbox   string `json:"mailbox" doc:"user@host/folder"`
	LastRun   string `json:"lastRun"`
	LastError string `json:"lastError"`
	Imported  int    `json:"imported" doc:"receipts imported since the server started"`
}

type mailState struct {
	mu sync.Mutex
	s  MailStatus
}

// MailStatus returns the current mail import state.
func (a *App) MailStatus() MailStatus {
	a.mail.mu.Lock()
	defer a.mail.mu.Unlock()
	return a.mail.s
}

// UpdateMailStatus lets the importer record its state.
func (a *App) UpdateMailStatus(fn func(*MailStatus)) {
	a.mail.mu.Lock()
	defer a.mail.mu.Unlock()
	fn(&a.mail.s)
}
