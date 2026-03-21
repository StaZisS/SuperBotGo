package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"SuperBotGo/internal/admin"
	adminapi "SuperBotGo/internal/admin/api"
	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/channel/discord"
	"SuperBotGo/internal/channel/telegram"
	"SuperBotGo/internal/chat"
	"SuperBotGo/internal/config"
	"SuperBotGo/internal/database"
	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/plugin/broadcast"
	pluginLink "SuperBotGo/internal/plugin/link"
	"SuperBotGo/internal/plugin/project"
	"SuperBotGo/internal/plugin/settings"
	"SuperBotGo/internal/role"
	"SuperBotGo/internal/state"
	"SuperBotGo/internal/trigger"
	"SuperBotGo/internal/user"
	"SuperBotGo/internal/wasm/adapter"
	"SuperBotGo/internal/wasm/hostapi"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	if err := i18n.Init(cfg.DefaultLocale); err != nil {
		logger.Warn("i18n initialization failed, continuing with fallback keys", slog.Any("error", err))
	}

	chatRegistry := chat.NewPlaceholderRegistry()
	adapterRegistry := channel.NewAdapterRegistry()
	pluginManager := plugin.NewManager()

	wasmCtx := context.Background()

	rt, err := wasmrt.NewRuntime(wasmCtx, wasmrt.Config{
		CacheDir: cfg.Admin.ModulesDir + "/.cache",
	})
	if err != nil {
		logger.Error("failed to create wasm runtime", slog.Any("error", err))
		os.Exit(1)
	}

	hostAPI := hostapi.NewHostAPI(hostapi.Dependencies{})

	if err := hostAPI.RegisterHostModule(wasmCtx, rt); err != nil {
		logger.Error("failed to register wasm host module", slog.Any("error", err))
		os.Exit(1)
	}

	var senderAPI *plugin.SenderAPI
	wasmSendFunc := adapter.SendFunc(func(ctx context.Context, channelType model.ChannelType, chatID string, text string) error {
		if senderAPI == nil {
			return fmt.Errorf("sender API not initialized")
		}
		msg := model.Message{
			Blocks: []model.ContentBlock{
				model.TextBlock{Text: text, Style: model.StylePlain},
			},
		}
		return senderAPI.ReplyToChat(ctx, channelType, chatID, msg)
	})

	wasmLoader := adapter.NewLoader(rt, hostAPI, wasmSendFunc)

	triggerRegistry := trigger.NewRegistry()
	wasmLoader.SetTriggerRegistry(triggerRegistry)

	blobStore, err := adminapi.NewLocalFSBlobStore(cfg.Admin.ModulesDir)
	if err != nil {
		logger.Error("failed to create blob store", slog.Any("error", err))
		os.Exit(1)
	}

	var userRepo user.UserRepository
	var accountRepo user.AccountRepository
	var roleStore role.Store
	var pluginStore adminapi.PluginStore
	var cmdPermStore adminapi.CommandPermStore
	var cmdAccessChecker *adminapi.CommandAccessChecker
	var ruleSchemaHandler *adminapi.RuleSchemaHandler

	connString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode,
	)
	if cfg.Database.Host != "" && cfg.Database.DBName != "" {
		pool, dbErr := database.NewPool(wasmCtx, connString)
		if dbErr != nil {
			logger.Warn("PostgreSQL not available, falling back to in-memory stores", slog.Any("error", dbErr))
		} else {
			if migErr := database.RunMigrations(connString); migErr != nil {
				logger.Warn("failed to run migrations, falling back to in-memory stores", slog.Any("error", migErr))
				pool.Close()
				pool = nil
			}
			if pool != nil {
				userRepo = user.NewPgUserRepo(pool)
				accountRepo = user.NewPgAccountRepo(pool)
				roleStore = role.NewPgStore(pool)
				pluginStore = adminapi.NewPgPluginStore(pool)
				cmdPermStore = adminapi.NewPgCommandPermStore(pool)
				cmdAccessChecker = adminapi.NewCommandAccessChecker(pool)
				ruleSchemaHandler = adminapi.NewRuleSchemaHandler(pool)
				logger.Info("using PostgreSQL stores")
			}
		}
	}

	if userRepo == nil {
		userRepo = user.NewPlaceholderUserRepo()
		accountRepo = user.NewPlaceholderAccountRepo()
		logger.Warn("using in-memory user/account stores (data will be lost on restart)")
	}
	if roleStore == nil {
		roleStore = role.NewPlaceholderStore()
		logger.Warn("using in-memory role store (data will be lost on restart)")
	}
	if pluginStore == nil {
		fileStore, fsErr := adminapi.NewFilePluginStore(cfg.Admin.ModulesDir)
		if fsErr != nil {
			logger.Error("failed to create file plugin store", slog.Any("error", fsErr))
			os.Exit(1)
		}
		pluginStore = fileStore
		logger.Info("using file-based plugin store")
	}

	userService := user.NewService(userRepo, accountRepo)
	roleManager := role.NewManager(roleStore, logger)

	if err := adminapi.AutoloadPlugins(wasmCtx, pluginStore, blobStore, wasmLoader, pluginManager); err != nil {
		logger.Warn("wasm autoload failed", slog.Any("error", err))
	}

	adapter.RegisterWasmPlugins(pluginManager, wasmLoader)

	dialogStorage := &inMemoryDialogStorage{}
	stateMgr := state.NewManager(dialogStorage)

	adminHandler := adminapi.NewAdminHandler(
		pluginStore,
		blobStore,
		wasmLoader,
		pluginManager,
		rt,
		hostAPI,
		stateMgr,
		cmdPermStore,
		cfg.Admin.APIKey,
	)

	adminMux := http.NewServeMux()
	adminHandler.RegisterRoutes(adminMux)
	cmdPermHandler := adminapi.NewCommandPermHandler(cmdPermStore)
	cmdPermHandler.RegisterRoutes(adminMux)
	pluginPermHandler := adminapi.NewPluginPermHandler(pluginStore, wasmLoader, hostAPI)
	pluginPermHandler.RegisterRoutes(adminMux)
	if ruleSchemaHandler == nil {
		ruleSchemaHandler = adminapi.NewRuleSchemaHandler(nil)
	}
	ruleSchemaHandler.RegisterRoutes(adminMux)
	triggerRouter := trigger.NewRouter(triggerRegistry, pluginManager)
	httpTrigger := trigger.NewHTTPTriggerHandler(triggerRouter, triggerRegistry)
	adminMux.Handle("/api/triggers/http/", httpTrigger)

	admin.RegisterStaticRoutes(adminMux)

	adminServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Admin.Port),
		Handler:      adminMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("starting Admin API HTTP server", slog.Int("port", cfg.Admin.Port))
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("admin API server error", slog.Any("error", err))
		}
	}()

	senderAPI = plugin.NewSenderAPI(adapterRegistry, userService, chatRegistry)

	projectStore := &placeholderProjectStore{}
	chatStore := &placeholderChatStore{}
	accountLinker := &placeholderAccountLinker{}

	allPlugins := []plugin.Plugin{
		broadcast.New(senderAPI, projectStore),
		pluginLink.New(senderAPI, accountLinker),
		project.New(senderAPI, projectStore, chatStore),
		settings.New(senderAPI, userService),
	}

	for _, p := range allPlugins {
		for _, def := range p.Commands() {
			stateMgr.RegisterCommand(def)
		}
	}

	for _, wp := range pluginManager.All() {
		for _, def := range wp.Commands() {
			stateMgr.RegisterCommand(def)
		}
	}

	stateAdapter := channel.NewStateManagerAdapter(stateMgr)

	pluginManager.Load(allPlugins)

	updateRouter := plugin.NewUpdateRouter(pluginManager, roleManager)

	channelMgr := channel.NewChannelManager(
		userService,
		updateRouter,
		stateAdapter,
		pluginManager,
		roleManager,
		adapterRegistry,
		logger,
	)
	if cmdAccessChecker != nil {
		channelMgr.SetCommandAccessChecker(cmdAccessChecker)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var commandNames []string
	for _, p := range pluginManager.All() {
		for _, def := range p.Commands() {
			commandNames = append(commandNames, def.Name)
		}
	}

	if cfg.Telegram.Token != "" {
		logger.Info("starting Telegram bot")
		tgBot, err := telegram.NewBot(cfg.Telegram.Token, channelMgr, logger)
		if err != nil {
			logger.Error("failed to create Telegram bot", slog.Any("error", err))
		} else {
			tgBot.RegisterCommands(commandNames)
			channelMgr.RegisterAdapter(tgBot.Adapter())
			go func() {
				if err := tgBot.Start(ctx); err != nil {
					logger.Error("Telegram bot stopped with error", slog.Any("error", err))
				}
			}()
		}
	} else {
		logger.Warn("Telegram token not configured, Telegram bot will not start")
	}

	if cfg.Discord.Token != "" {
		logger.Info("starting Discord bot")
		dcBot, err := discord.NewBot(cfg.Discord.Token, channelMgr, logger)
		if err != nil {
			logger.Error("failed to create Discord bot", slog.Any("error", err))
		} else {
			channelMgr.RegisterAdapter(dcBot.Adapter())
			go func() {
				if err := dcBot.Start(ctx); err != nil {
					logger.Error("Discord bot stopped with error", slog.Any("error", err))
				}
			}()
		}
	} else {
		logger.Warn("Discord token not configured, Discord bot will not start")
	}

	logger.Info("SuperBotGo started, waiting for shutdown signal")

	<-sigCh
	logger.Info("shutdown signal received, stopping...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := adminServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("admin API server shutdown error", slog.Any("error", err))
	} else {
		logger.Info("admin API server stopped")
	}

	if err := wasmLoader.Close(shutdownCtx); err != nil {
		logger.Error("wasm loader close error", slog.Any("error", err))
	}
	if err := rt.Close(shutdownCtx); err != nil {
		logger.Error("wasm runtime close error", slog.Any("error", err))
	}

	logger.Info("SuperBotGo stopped")
}
