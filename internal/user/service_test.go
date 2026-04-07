package user

import (
	"context"
	"errors"
	"testing"

	"SuperBotGo/internal/model"
)

// --- Mock UserRepository ---

type mockUserRepo struct {
	findByIDFn            func(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error)
	findByTsuAccountsIDFn func(ctx context.Context, tsuAccountsID string) (*model.GlobalUser, error)
	saveFn                func(ctx context.Context, user *model.GlobalUser) (*model.GlobalUser, error)
	deleteFn              func(ctx context.Context, id model.GlobalUserID) error
	setTsuAccountsIDFn    func(ctx context.Context, userID model.GlobalUserID, tsuAccountsID string) error
	updateLocalFn         func(ctx context.Context, userID model.GlobalUserID, locale string) error
}

func (m *mockUserRepo) FindByID(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error) {
	return m.findByIDFn(ctx, id)
}

func (m *mockUserRepo) FindByTsuAccountsID(ctx context.Context, tsuAccountsID string) (*model.GlobalUser, error) {
	if m.findByTsuAccountsIDFn != nil {
		return m.findByTsuAccountsIDFn(ctx, tsuAccountsID)
	}
	return nil, nil
}

func (m *mockUserRepo) Save(ctx context.Context, user *model.GlobalUser) (*model.GlobalUser, error) {
	return m.saveFn(ctx, user)
}

func (m *mockUserRepo) Delete(ctx context.Context, id model.GlobalUserID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockUserRepo) SetTsuAccountsID(ctx context.Context, userID model.GlobalUserID, tsuAccountsID string) error {
	if m.setTsuAccountsIDFn != nil {
		return m.setTsuAccountsIDFn(ctx, userID, tsuAccountsID)
	}
	return nil
}

func (m *mockUserRepo) UpdateLocale(ctx context.Context, userID model.GlobalUserID, locale string) error {
	if m.updateLocalFn != nil {
		return m.updateLocalFn(ctx, userID, locale)
	}
	return nil
}

// --- Mock AccountRepository ---

type mockAccountRepo struct {
	findByChannelAndPlatformIDFn func(ctx context.Context, ct model.ChannelType, platformID model.PlatformUserID) (*model.ChannelAccount, error)
	findByGlobalUserIDFn         func(ctx context.Context, globalUserID model.GlobalUserID) ([]model.ChannelAccount, error)
	saveFn                       func(ctx context.Context, account *model.ChannelAccount) (*model.ChannelAccount, error)
	saveCalls                    []*model.ChannelAccount
}

func (m *mockAccountRepo) FindByChannelAndPlatformID(ctx context.Context, ct model.ChannelType, platformID model.PlatformUserID) (*model.ChannelAccount, error) {
	return m.findByChannelAndPlatformIDFn(ctx, ct, platformID)
}

func (m *mockAccountRepo) FindByGlobalUserID(ctx context.Context, globalUserID model.GlobalUserID) ([]model.ChannelAccount, error) {
	if m.findByGlobalUserIDFn != nil {
		return m.findByGlobalUserIDFn(ctx, globalUserID)
	}
	return nil, nil
}

func (m *mockAccountRepo) Save(ctx context.Context, account *model.ChannelAccount) (*model.ChannelAccount, error) {
	m.saveCalls = append(m.saveCalls, account)
	return m.saveFn(ctx, account)
}

// --- Tests for FindOrCreateUser ---

func TestFindOrCreateUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		channelType    model.ChannelType
		platformUserID model.PlatformUserID
		username       []string
		setupUserRepo  func() *mockUserRepo
		setupAcctRepo  func() *mockAccountRepo
		wantErr        bool
		wantUserID     model.GlobalUserID
		wantSaveCalled bool
	}{
		{
			name:           "existing account found returns existing user",
			channelType:    model.ChannelTelegram,
			platformUserID: "tg-123",
			username:       []string{"alice"},
			setupUserRepo: func() *mockUserRepo {
				return &mockUserRepo{
					findByIDFn: func(_ context.Context, id model.GlobalUserID) (*model.GlobalUser, error) {
						return &model.GlobalUser{
							ID:             42,
							PrimaryChannel: model.ChannelTelegram,
						}, nil
					},
				}
			},
			setupAcctRepo: func() *mockAccountRepo {
				return &mockAccountRepo{
					findByChannelAndPlatformIDFn: func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID) (*model.ChannelAccount, error) {
						return &model.ChannelAccount{
							ID:            1,
							ChannelType:   model.ChannelTelegram,
							ChannelUserID: "tg-123",
							GlobalUserID:  42,
							Username:      "alice",
						}, nil
					},
					saveFn: func(_ context.Context, acc *model.ChannelAccount) (*model.ChannelAccount, error) {
						return acc, nil
					},
				}
			},
			wantErr:    false,
			wantUserID: 42,
		},
		{
			name:           "existing account with username change triggers save",
			channelType:    model.ChannelTelegram,
			platformUserID: "tg-123",
			username:       []string{"bob"},
			setupUserRepo: func() *mockUserRepo {
				return &mockUserRepo{
					findByIDFn: func(_ context.Context, _ model.GlobalUserID) (*model.GlobalUser, error) {
						return &model.GlobalUser{
							ID:             42,
							PrimaryChannel: model.ChannelTelegram,
						}, nil
					},
				}
			},
			setupAcctRepo: func() *mockAccountRepo {
				return &mockAccountRepo{
					findByChannelAndPlatformIDFn: func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID) (*model.ChannelAccount, error) {
						return &model.ChannelAccount{
							ID:            1,
							ChannelType:   model.ChannelTelegram,
							ChannelUserID: "tg-123",
							GlobalUserID:  42,
							Username:      "alice",
						}, nil
					},
					saveFn: func(_ context.Context, acc *model.ChannelAccount) (*model.ChannelAccount, error) {
						return acc, nil
					},
				}
			},
			wantErr:        false,
			wantUserID:     42,
			wantSaveCalled: true,
		},
		{
			name:           "account not found creates new user and account",
			channelType:    model.ChannelDiscord,
			platformUserID: "dc-456",
			username:       []string{"carol"},
			setupUserRepo: func() *mockUserRepo {
				return &mockUserRepo{
					saveFn: func(_ context.Context, user *model.GlobalUser) (*model.GlobalUser, error) {
						user.ID = 100
						return user, nil
					},
				}
			},
			setupAcctRepo: func() *mockAccountRepo {
				return &mockAccountRepo{
					findByChannelAndPlatformIDFn: func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID) (*model.ChannelAccount, error) {
						return nil, nil
					},
					saveFn: func(_ context.Context, acc *model.ChannelAccount) (*model.ChannelAccount, error) {
						acc.ID = 200
						return acc, nil
					},
				}
			},
			wantErr:    false,
			wantUserID: 100,
		},
		{
			name:           "accountRepo FindByChannelAndPlatformID error propagates",
			channelType:    model.ChannelTelegram,
			platformUserID: "tg-err",
			setupUserRepo: func() *mockUserRepo {
				return &mockUserRepo{}
			},
			setupAcctRepo: func() *mockAccountRepo {
				return &mockAccountRepo{
					findByChannelAndPlatformIDFn: func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID) (*model.ChannelAccount, error) {
						return nil, errors.New("db connection lost")
					},
					saveFn: func(_ context.Context, acc *model.ChannelAccount) (*model.ChannelAccount, error) {
						return acc, nil
					},
				}
			},
			wantErr: true,
		},
		{
			name:           "userRepo FindByID error propagates",
			channelType:    model.ChannelTelegram,
			platformUserID: "tg-123",
			setupUserRepo: func() *mockUserRepo {
				return &mockUserRepo{
					findByIDFn: func(_ context.Context, _ model.GlobalUserID) (*model.GlobalUser, error) {
						return nil, errors.New("user lookup failed")
					},
				}
			},
			setupAcctRepo: func() *mockAccountRepo {
				return &mockAccountRepo{
					findByChannelAndPlatformIDFn: func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID) (*model.ChannelAccount, error) {
						return &model.ChannelAccount{
							ID:            1,
							ChannelType:   model.ChannelTelegram,
							ChannelUserID: "tg-123",
							GlobalUserID:  42,
							Username:      "alice",
						}, nil
					},
					saveFn: func(_ context.Context, acc *model.ChannelAccount) (*model.ChannelAccount, error) {
						return acc, nil
					},
				}
			},
			wantErr: true,
		},
		{
			name:           "userRepo Save error during creation propagates",
			channelType:    model.ChannelDiscord,
			platformUserID: "dc-999",
			setupUserRepo: func() *mockUserRepo {
				return &mockUserRepo{
					saveFn: func(_ context.Context, _ *model.GlobalUser) (*model.GlobalUser, error) {
						return nil, errors.New("insert failed")
					},
				}
			},
			setupAcctRepo: func() *mockAccountRepo {
				return &mockAccountRepo{
					findByChannelAndPlatformIDFn: func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID) (*model.ChannelAccount, error) {
						return nil, nil
					},
					saveFn: func(_ context.Context, acc *model.ChannelAccount) (*model.ChannelAccount, error) {
						return acc, nil
					},
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			userRepo := tt.setupUserRepo()
			acctRepo := tt.setupAcctRepo()
			svc := NewService(userRepo, acctRepo)

			got, err := svc.FindOrCreateUser(context.Background(), tt.channelType, tt.platformUserID, tt.username...)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ID != tt.wantUserID {
				t.Errorf("user ID = %d, want %d", got.ID, tt.wantUserID)
			}
			if tt.wantSaveCalled && len(acctRepo.saveCalls) == 0 {
				t.Error("expected accountRepo.Save to be called for username update, but it was not")
			}
		})
	}
}

func TestGetUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		userID     model.GlobalUserID
		findResult *model.GlobalUser
		findErr    error
		wantErr    bool
		wantNil    bool
	}{
		{
			name:   "delegates to userRepo FindByID",
			userID: 1,
			findResult: &model.GlobalUser{
				ID:             1,
				PrimaryChannel: model.ChannelTelegram,
			},
			wantErr: false,
		},
		{
			name:    "returns error from repo",
			userID:  2,
			findErr: errors.New("not found"),
			wantErr: true,
		},
		{
			name:    "returns nil when user does not exist",
			userID:  3,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			userRepo := &mockUserRepo{
				findByIDFn: func(_ context.Context, id model.GlobalUserID) (*model.GlobalUser, error) {
					if id != tt.userID {
						t.Errorf("FindByID called with id=%d, want %d", id, tt.userID)
					}
					return tt.findResult, tt.findErr
				},
			}
			acctRepo := &mockAccountRepo{
				findByChannelAndPlatformIDFn: func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID) (*model.ChannelAccount, error) {
					return nil, nil
				},
				saveFn: func(_ context.Context, acc *model.ChannelAccount) (*model.ChannelAccount, error) {
					return acc, nil
				},
			}

			svc := NewService(userRepo, acctRepo)
			got, err := svc.GetUser(context.Background(), tt.userID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil user, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil user, got nil")
			}
			if got.ID != tt.findResult.ID {
				t.Errorf("user ID = %d, want %d", got.ID, tt.findResult.ID)
			}
		})
	}
}
