package user

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	"SuperBotGo/internal/model"
	pluginCore "SuperBotGo/internal/plugin/core"
)

const (
	codeLength = 6
	codeTTL    = 5 * time.Minute
	codeChars  = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

type linkEntry struct {
	UserID    model.GlobalUserID
	ExpiresAt time.Time
}

type AccountLinkerImpl struct {
	mu          sync.Mutex
	codes       map[string]*linkEntry
	accountRepo AccountRepository
}

func NewAccountLinker(accountRepo AccountRepository) *AccountLinkerImpl {
	return &AccountLinkerImpl{
		codes:       make(map[string]*linkEntry),
		accountRepo: accountRepo,
	}
}

func (l *AccountLinkerImpl) InitiateLinking(_ context.Context, userID model.GlobalUserID) pluginCore.LinkResult {
	l.mu.Lock()
	defer l.mu.Unlock()

	for code, entry := range l.codes {
		if entry.UserID == userID {
			delete(l.codes, code)
		}
	}

	code, err := generateCode()
	if err != nil {
		return pluginCore.LinkResult{
			Kind:    pluginCore.LinkErrorKind,
			Message: "Failed to generate link code.",
		}
	}

	l.codes[code] = &linkEntry{
		UserID:    userID,
		ExpiresAt: time.Now().Add(codeTTL),
	}

	return pluginCore.LinkResult{
		Kind: pluginCore.LinkCodeGeneratedKind,
		Code: code,
	}
}

func (l *AccountLinkerImpl) CompleteLinking(ctx context.Context, userID model.GlobalUserID, code string) pluginCore.LinkResult {
	l.mu.Lock()
	entry, ok := l.codes[code]
	if !ok {
		l.mu.Unlock()
		return pluginCore.LinkResult{
			Kind:    pluginCore.LinkErrorKind,
			Message: "Invalid link code.",
		}
	}

	if time.Now().After(entry.ExpiresAt) {
		delete(l.codes, code)
		l.mu.Unlock()
		return pluginCore.LinkResult{
			Kind:    pluginCore.LinkErrorKind,
			Message: "Link code has expired. Please generate a new one.",
		}
	}

	sourceUserID := entry.UserID
	delete(l.codes, code)
	l.mu.Unlock()

	if sourceUserID == userID {
		return pluginCore.LinkResult{
			Kind:    pluginCore.LinkErrorKind,
			Message: "Cannot link to yourself. Use the code from a different platform.",
		}
	}

	accounts, err := l.accountRepo.FindByGlobalUserID(ctx, sourceUserID)
	if err != nil {
		return pluginCore.LinkResult{
			Kind:    pluginCore.LinkErrorKind,
			Message: fmt.Sprintf("Failed to find accounts: %v", err),
		}
	}

	for i := range accounts {
		accounts[i].GlobalUserID = userID
		if _, err := l.accountRepo.Save(ctx, &accounts[i]); err != nil {
			return pluginCore.LinkResult{
				Kind:    pluginCore.LinkErrorKind,
				Message: fmt.Sprintf("Failed to link account: %v", err),
			}
		}
	}

	return pluginCore.LinkResult{
		Kind:    pluginCore.LinkLinkedKind,
		Message: fmt.Sprintf("Accounts linked successfully! %d account(s) merged.", len(accounts)),
	}
}

func generateCode() (string, error) {
	b := make([]byte, codeLength)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(codeChars))))
		if err != nil {
			return "", err
		}
		b[i] = codeChars[n.Int64()]
	}
	return string(b), nil
}

var _ pluginCore.AccountLinker = (*AccountLinkerImpl)(nil)