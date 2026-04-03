package user

import (
	"context"
	"testing"

	"SuperBotGo/internal/model"
	pluginCore "SuperBotGo/internal/plugin/core"
)

func newTestLinker(acctRepo AccountRepository) *AccountLinkerImpl {
	linker := NewAccountLinker(acctRepo)
	t := linker // ensure cleanup goroutine stops
	_ = t
	return linker
}

func TestInitiateLinking_CodeLength(t *testing.T) {
	t.Parallel()

	acctRepo := &mockAccountRepo{
		findByChannelAndPlatformIDFn: func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID) (*model.ChannelAccount, error) {
			return nil, nil
		},
		saveFn: func(_ context.Context, acc *model.ChannelAccount) (*model.ChannelAccount, error) {
			return acc, nil
		},
	}

	linker := NewAccountLinker(acctRepo)
	defer linker.Stop()

	result := linker.InitiateLinking(context.Background(), 1)

	if result.Kind != pluginCore.LinkCodeGenerated {
		t.Fatalf("expected LinkCodeGenerated, got %d", result.Kind)
	}
	if len(result.Code) != codeLength {
		t.Errorf("code length = %d, want %d", len(result.Code), codeLength)
	}
}

func TestInitiateLinking_ReplacesPreviousCode(t *testing.T) {
	t.Parallel()

	acctRepo := &mockAccountRepo{
		findByChannelAndPlatformIDFn: func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID) (*model.ChannelAccount, error) {
			return nil, nil
		},
		saveFn: func(_ context.Context, acc *model.ChannelAccount) (*model.ChannelAccount, error) {
			return acc, nil
		},
	}

	linker := NewAccountLinker(acctRepo)
	defer linker.Stop()

	ctx := context.Background()
	var userID model.GlobalUserID = 10

	first := linker.InitiateLinking(ctx, userID)
	if first.Kind != pluginCore.LinkCodeGenerated {
		t.Fatalf("first call: expected LinkCodeGenerated, got %d", first.Kind)
	}

	second := linker.InitiateLinking(ctx, userID)
	if second.Kind != pluginCore.LinkCodeGenerated {
		t.Fatalf("second call: expected LinkCodeGenerated, got %d", second.Kind)
	}

	if first.Code == second.Code {
		t.Error("expected second code to differ from first (old code should be replaced)")
	}

	// The old code should no longer be valid.
	linker.mu.Lock()
	_, oldExists := linker.codes[first.Code]
	linker.mu.Unlock()

	if oldExists {
		t.Error("old code should have been removed after re-initiation")
	}
}

func TestCompleteLinking_ValidCode(t *testing.T) {
	t.Parallel()

	accounts := []model.ChannelAccount{
		{
			ID:            1,
			ChannelType:   model.ChannelTelegram,
			ChannelUserID: "tg-source",
			GlobalUserID:  10,
		},
	}

	acctRepo := &mockAccountRepo{
		findByChannelAndPlatformIDFn: func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID) (*model.ChannelAccount, error) {
			return nil, nil
		},
		findByGlobalUserIDFn: func(_ context.Context, _ model.GlobalUserID) ([]model.ChannelAccount, error) {
			return accounts, nil
		},
		saveFn: func(_ context.Context, acc *model.ChannelAccount) (*model.ChannelAccount, error) {
			return acc, nil
		},
	}

	linker := NewAccountLinker(acctRepo)
	defer linker.Stop()

	ctx := context.Background()
	var sourceUser model.GlobalUserID = 10
	var targetUser model.GlobalUserID = 20

	initResult := linker.InitiateLinking(ctx, sourceUser)
	if initResult.Kind != pluginCore.LinkCodeGenerated {
		t.Fatalf("expected LinkCodeGenerated, got %d", initResult.Kind)
	}

	completeResult := linker.CompleteLinking(ctx, targetUser, initResult.Code)

	if completeResult.Kind != pluginCore.LinkLinked {
		t.Fatalf("expected LinkLinked, got %d (message: %s)", completeResult.Kind, completeResult.Message)
	}
}

func TestCompleteLinking_InvalidCode(t *testing.T) {
	t.Parallel()

	acctRepo := &mockAccountRepo{
		findByChannelAndPlatformIDFn: func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID) (*model.ChannelAccount, error) {
			return nil, nil
		},
		saveFn: func(_ context.Context, acc *model.ChannelAccount) (*model.ChannelAccount, error) {
			return acc, nil
		},
	}

	linker := NewAccountLinker(acctRepo)
	defer linker.Stop()

	result := linker.CompleteLinking(context.Background(), 20, "BADCODE")

	if result.Kind != pluginCore.LinkError {
		t.Fatalf("expected LinkError, got %d", result.Kind)
	}
	if result.Message != "Invalid link code." {
		t.Errorf("message = %q, want %q", result.Message, "Invalid link code.")
	}
}

func TestCompleteLinking_SelfLink(t *testing.T) {
	t.Parallel()

	acctRepo := &mockAccountRepo{
		findByChannelAndPlatformIDFn: func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID) (*model.ChannelAccount, error) {
			return nil, nil
		},
		saveFn: func(_ context.Context, acc *model.ChannelAccount) (*model.ChannelAccount, error) {
			return acc, nil
		},
	}

	linker := NewAccountLinker(acctRepo)
	defer linker.Stop()

	ctx := context.Background()
	var sameUser model.GlobalUserID = 10

	initResult := linker.InitiateLinking(ctx, sameUser)
	if initResult.Kind != pluginCore.LinkCodeGenerated {
		t.Fatalf("expected LinkCodeGenerated, got %d", initResult.Kind)
	}

	result := linker.CompleteLinking(ctx, sameUser, initResult.Code)

	if result.Kind != pluginCore.LinkError {
		t.Fatalf("expected LinkError for self-link, got %d", result.Kind)
	}
	expected := "Cannot link to yourself. Use the code from a different platform."
	if result.Message != expected {
		t.Errorf("message = %q, want %q", result.Message, expected)
	}
}

func TestCompleteLinking_CodeConsumed(t *testing.T) {
	t.Parallel()

	accounts := []model.ChannelAccount{
		{
			ID:            1,
			ChannelType:   model.ChannelDiscord,
			ChannelUserID: "dc-source",
			GlobalUserID:  10,
		},
	}

	acctRepo := &mockAccountRepo{
		findByChannelAndPlatformIDFn: func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID) (*model.ChannelAccount, error) {
			return nil, nil
		},
		findByGlobalUserIDFn: func(_ context.Context, _ model.GlobalUserID) ([]model.ChannelAccount, error) {
			return accounts, nil
		},
		saveFn: func(_ context.Context, acc *model.ChannelAccount) (*model.ChannelAccount, error) {
			return acc, nil
		},
	}

	linker := NewAccountLinker(acctRepo)
	defer linker.Stop()

	ctx := context.Background()
	initResult := linker.InitiateLinking(ctx, 10)

	// First completion should succeed.
	first := linker.CompleteLinking(ctx, 20, initResult.Code)
	if first.Kind != pluginCore.LinkLinked {
		t.Fatalf("first completion: expected LinkLinked, got %d", first.Kind)
	}

	// Second attempt with same code should fail (code already consumed).
	second := linker.CompleteLinking(ctx, 30, initResult.Code)
	if second.Kind != pluginCore.LinkError {
		t.Fatalf("second completion: expected LinkError, got %d", second.Kind)
	}
}

func TestStop_NoPanic(t *testing.T) {
	t.Parallel()

	acctRepo := &mockAccountRepo{
		findByChannelAndPlatformIDFn: func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID) (*model.ChannelAccount, error) {
			return nil, nil
		},
		saveFn: func(_ context.Context, acc *model.ChannelAccount) (*model.ChannelAccount, error) {
			return acc, nil
		},
	}

	linker := NewAccountLinker(acctRepo)
	linker.Stop() // should not panic
}
