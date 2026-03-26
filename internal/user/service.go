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

func (s *Service) FindOrCreateUser(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID) (*model.GlobalUser, error) {

	account, err := s.accountRepo.FindByChannelAndPlatformID(ctx, channelType, platformUserID)
	if err != nil {
		return nil, err
	}

	if account != nil {

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
