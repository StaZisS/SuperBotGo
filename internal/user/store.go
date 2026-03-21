package user

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"SuperBotGo/internal/model"
)

type UserRepository interface {
	FindByID(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error)
	Save(ctx context.Context, user *model.GlobalUser) (*model.GlobalUser, error)
	UpdateLocale(ctx context.Context, userID model.GlobalUserID, locale string) error
}

type AccountRepository interface {
	FindByChannelAndPlatformID(ctx context.Context, ct model.ChannelType, platformID model.PlatformUserID) (*model.ChannelAccount, error)
	FindByGlobalUserID(ctx context.Context, globalUserID model.GlobalUserID) ([]model.ChannelAccount, error)
	Save(ctx context.Context, account *model.ChannelAccount) (*model.ChannelAccount, error)
}

type PlaceholderUserRepo struct {
	mu    sync.RWMutex
	users map[model.GlobalUserID]*model.GlobalUser
	seq   atomic.Int64
}

func NewPlaceholderUserRepo() *PlaceholderUserRepo {
	return &PlaceholderUserRepo{
		users: make(map[model.GlobalUserID]*model.GlobalUser),
	}
}

func (r *PlaceholderUserRepo) FindByID(_ context.Context, id model.GlobalUserID) (*model.GlobalUser, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return nil, nil
	}
	copy := *u
	return &copy, nil
}

func (r *PlaceholderUserRepo) Save(_ context.Context, user *model.GlobalUser) (*model.GlobalUser, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if user.ID == 0 {
		user.ID = model.GlobalUserID(r.seq.Add(1))
	}
	stored := *user
	r.users[user.ID] = &stored
	result := stored
	return &result, nil
}

func (r *PlaceholderUserRepo) UpdateLocale(_ context.Context, userID model.GlobalUserID, locale string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[userID]
	if !ok {
		return fmt.Errorf("user %d not found", userID)
	}
	u.Locale = locale
	return nil
}

var _ UserRepository = (*PlaceholderUserRepo)(nil)

type PlaceholderAccountRepo struct {
	mu       sync.RWMutex
	accounts []*model.ChannelAccount
	seq      atomic.Int64
}

func NewPlaceholderAccountRepo() *PlaceholderAccountRepo {
	return &PlaceholderAccountRepo{}
}

func (r *PlaceholderAccountRepo) FindByChannelAndPlatformID(_ context.Context, ct model.ChannelType, platformID model.PlatformUserID) (*model.ChannelAccount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, acc := range r.accounts {
		if acc.ChannelType == ct && acc.ChannelUserID == platformID {
			copy := *acc
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *PlaceholderAccountRepo) FindByGlobalUserID(_ context.Context, globalUserID model.GlobalUserID) ([]model.ChannelAccount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []model.ChannelAccount
	for _, acc := range r.accounts {
		if acc.GlobalUserID == globalUserID {
			result = append(result, *acc)
		}
	}
	return result, nil
}

func (r *PlaceholderAccountRepo) Save(_ context.Context, account *model.ChannelAccount) (*model.ChannelAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if account.ID == 0 {
		account.ID = r.seq.Add(1)
		stored := *account
		r.accounts = append(r.accounts, &stored)
	} else {
		for i, acc := range r.accounts {
			if acc.ID == account.ID {
				stored := *account
				r.accounts[i] = &stored
				break
			}
		}
	}
	result := *account
	return &result, nil
}

var _ AccountRepository = (*PlaceholderAccountRepo)(nil)
