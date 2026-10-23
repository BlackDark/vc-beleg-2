package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// FolderStatus is the state of the folder import shown in the settings.
type FolderStatus struct {
	Enabled   bool   `json:"enabled"`
	Folder    string `json:"folder" doc:"URL of the watched folder, without credentials"`
	LastRun   string `json:"lastRun"`
	LastError string `json:"lastError"`
	Imported  int    `json:"imported" doc:"receipts imported since the server started"`
}

type folderState struct {
	mu sync.Mutex
	s  FolderStatus
}

// FolderStatus returns the current folder import state.
func (a *App) FolderStatus() FolderStatus {
	a.folder.mu.Lock()
	defer a.folder.mu.Unlock()
	return a.folder.s
}

// UpdateFolderStatus lets the importer record its state.
func (a *App) UpdateFolderStatus(fn func(*FolderStatus)) {
	a.folder.mu.Lock()
	defer a.folder.mu.Unlock()
	fn(&a.folder.s)
}

// FileImported reports whether a file with the same content was already
// imported from the folder, so a file whose move failed is not imported
// twice.
func (a *App) FileImported(ctx context.Context, u *User, data []byte) (bool, error) {
	sum := sha256.Sum256(data)
	var n int
	err := a.DB.QueryRowContext(ctx, `SELECT count(*) FROM receipts WHERE employee_id = ? AND source = 'folder' AND sha256 = ?`, u.EmployeeID, hex.EncodeToString(sum[:])).Scan(&n)
	return n > 0, err
}
