package user

import (
	"context"

	"SuperBotGo/internal/model"
)

type UserRepository interface {
	FindByID(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error)
	FindByTsuAccountsID(ctx context.Context, tsuAccountsID string) (*model.GlobalUser, error)
	Save(ctx context.Context, user *model.GlobalUser) (*model.GlobalUser, error)
	Delete(ctx context.Context, id model.GlobalUserID) error
	SetTsuAccountsID(ctx context.Context, userID model.GlobalUserID, tsuAccountsID string) error
	UpdateLocale(ctx context.Context, userID model.GlobalUserID, locale string) error
}

type AccountRepository interface {
	FindByChannelAndPlatformID(ctx context.Context, ct model.ChannelType, platformID model.PlatformUserID) (*model.ChannelAccount, error)
	FindByGlobalUserID(ctx context.Context, globalUserID model.GlobalUserID) ([]model.ChannelAccount, error)
	Save(ctx context.Context, account *model.ChannelAccount) (*model.ChannelAccount, error)
}
