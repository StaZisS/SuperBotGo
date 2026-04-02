package user

import (
	"context"

	"SuperBotGo/internal/locale"
	"SuperBotGo/internal/model"
)

type Service struct {
	userRepo    UserRepository
	accountRepo AccountRepository
}

func NewService(userRepo UserRepository, accountRepo AccountRepository) *Service {
	return &Service{
		userRepo:    userRepo,
		accountRepo: accountRepo,
	}
}

func (s *Service) FindOrCreateUser(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID, username ...string) (*model.GlobalUser, error) {
	uname := ""
	if len(username) > 0 {
		uname = username[0]
	}

	account, err := s.accountRepo.FindByChannelAndPlatformID(ctx, channelType, platformUserID)
	if err != nil {
		return nil, err
	}

	if account != nil {
		// Update username if changed
		if uname != "" && account.Username != uname {
			account.Username = uname
			_, _ = s.accountRepo.Save(ctx, account)
		}

		user, err := s.userRepo.FindByID(ctx, account.GlobalUserID)
		if err != nil {
			return nil, err
		}
		if user != nil {
			return user, nil
		}
	}

	newUser := &model.GlobalUser{
		PrimaryChannel: channelType,
		Locale:         locale.Default(),
	}

	savedUser, err := s.userRepo.Save(ctx, newUser)
	if err != nil {
		return nil, err
	}

	newAccount := &model.ChannelAccount{
		ChannelType:   channelType,
		ChannelUserID: platformUserID,
		GlobalUserID:  savedUser.ID,
		Username:      uname,
	}

	savedAccount, err := s.accountRepo.Save(ctx, newAccount)
	if err != nil {
		return nil, err
	}

	savedUser.Accounts = []model.ChannelAccount{*savedAccount}
	return savedUser, nil
}

func (s *Service) GetUser(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error) {
	return s.userRepo.FindByID(ctx, id)
}

func (s *Service) UpdateLocale(ctx context.Context, userID model.GlobalUserID, locale string) error {
	return s.userRepo.UpdateLocale(ctx, userID, locale)
}
