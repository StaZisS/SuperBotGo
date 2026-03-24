package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"SuperBotGo/internal/admin"
	adminapi "SuperBotGo/internal/admin/api"
	"SuperBotGo/internal/authz"
	"SuperBotGo/internal/authz/providers"
	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/channel/discord"
	"SuperBotGo/internal/channel/telegram"
	"SuperBotGo/internal/chat"
	"SuperBotGo/internal/config"
	"SuperBotGo/internal/database"
	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/metrics"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/notification"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/plugin/core"
	"SuperBotGo/internal/pubsub"
	"SuperBotGo/internal/role"
	"SuperBotGo/internal/state"
	"SuperBotGo/internal/state/storage"
	"SuperBotGo/internal/trigger"
	"SuperBotGo/internal/user"
	"SuperBotGo/internal/wasm/adapter"
	"SuperBotGo/internal/wasm/eventbus"
	"SuperBotGo/internal/wasm/hostapi"
	"SuperBotGo/internal/wasm/registry"
	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	var chatRegistry chat.Registry
	adapterRegistry := channel.NewAdapterRegistry()
	pluginManager := plugin.NewManager()

	m := metrics.New()

	wasmCtx := context.Background()

	rt, err := wasmrt.NewRuntime(wasmCtx, wasmrt.Config{
		CacheDir: cfg.Admin.ModulesDir + "/.cache",
	})
	if err != nil {
		logger.Error("failed to create wasm runtime", slog.Any("error", err))
		os.Exit(1)
	}

	rt.SetMetrics(m)

	hostAPI := hostapi.NewHostAPI(hostapi.Dependencies{})
	hostAPI.SetMetrics(m)

	ebMetrics := eventbus.NewMetrics()
	pluginEventBus := eventbus.New(nil, ebMetrics)
	hostAPI.SetEventBus(pluginEventBus)
	logger.Info("plugin event bus initialised with at-least-once delivery")

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

	pluginRegistry := registry.NewPluginRegistry()

	wasmLoader := adapter.NewLoader(rt, hostAPI, wasmSendFunc)
	wasmLoader.SetMetrics(m)
	wasmLoader.SetRegistry(pluginRegistry)

	triggerRegistry := trigger.NewRegistry()
	wasmLoader.SetTriggerRegistry(triggerRegistry)

	triggerRouter := trigger.NewRouter(triggerRegistry, pluginManager)
	cronScheduler := trigger.NewCronScheduler(triggerRouter)
	triggerRegistry.SetCronScheduler(cronScheduler)

	var blobStore adminapi.BlobStore
	switch cfg.Admin.BlobStore {
	case "s3":
		s3Store, s3Err := adminapi.NewS3BlobStore(wasmCtx, adminapi.S3BlobStoreConfig{
			Bucket:    cfg.Admin.S3.Bucket,
			Region:    cfg.Admin.S3.Region,
			Endpoint:  cfg.Admin.S3.Endpoint,
			AccessKey: cfg.Admin.S3.AccessKey,
			SecretKey: cfg.Admin.S3.SecretKey,
			Prefix:    cfg.Admin.S3.Prefix,
		})
		if s3Err != nil {
			logger.Error("failed to create S3 blob store", slog.Any("error", s3Err))
			os.Exit(1)
		}
		blobStore = s3Store
		logger.Info("using S3 blob store", slog.String("bucket", cfg.Admin.S3.Bucket))
	default:
		fsStore, fsErr := adminapi.NewLocalFSBlobStore(cfg.Admin.ModulesDir)
		if fsErr != nil {
			logger.Error("failed to create blob store", slog.Any("error", fsErr))
			os.Exit(1)
		}
		blobStore = fsStore
		logger.Info("using local filesystem blob store", slog.String("dir", cfg.Admin.ModulesDir))
	}

	if cfg.Redis.Addr == "" {
		logger.Error("Redis configuration is required (redis addr must be set)")
		os.Exit(1)
	}
	redisClient, redisErr := database.NewRedisClient(wasmCtx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if redisErr != nil {
		logger.Error("failed to connect to Redis", slog.Any("error", redisErr))
		os.Exit(1)
	}
	cronScheduler.SetRedis(redisClient)
	logger.Info("connected to Redis", slog.String("addr", cfg.Redis.Addr))

	connString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode,
	)
	if cfg.Database.Host == "" || cfg.Database.DBName == "" {
		logger.Error("PostgreSQL configuration is required (database host and name must be set)")
		os.Exit(1)
	}
	pool, dbErr := database.NewPool(wasmCtx, connString)
	if dbErr != nil {
		logger.Error("failed to connect to PostgreSQL", slog.Any("error", dbErr))
		os.Exit(1)
	}
	if migErr := database.RunMigrations(connString); migErr != nil {
		logger.Error("failed to run database migrations", slog.Any("error", migErr))
		os.Exit(1)
	}
	userRepo := user.NewPgUserRepo(pool)
	accountRepo := user.NewPgAccountRepo(pool)
	roleStore := role.NewPgStore(pool)
	pluginStore := adminapi.NewPgPluginStore(pool)
	versionStore := adminapi.NewPgVersionStore(pool)
	cmdPermStore := adminapi.NewPgCommandPermStore(pool)
	adminChatStore := adminapi.NewPgAdminChatStore(pool)
	authzStore := authz.NewPgStore(pool)
	universityProvider := providers.NewUniversityProvider(pool)
	adminBus := pubsub.NewBus(pool, connString, generateInstanceID())
	chatRegistry = chat.NewPgRegistry(pool)
	notifPrefsRepo := notification.NewPgPrefsRepo(pool)
	logger.Info("using PostgreSQL stores")

	userService := user.NewService(userRepo, accountRepo)
	roleManager := role.NewManager(roleStore, logger)

	authorizer := authz.NewAuthorizer(authzStore, logger, universityProvider)
	schemaBuilder := authz.NewRuleSchemaBuilder(authzStore, universityProvider)
	ruleSchemaHandler := adminapi.NewRuleSchemaHandler(schemaBuilder)

	if err := adminapi.AutoloadPlugins(wasmCtx, pluginStore, blobStore, wasmLoader, pluginManager); err != nil {
		logger.Warn("wasm autoload failed", slog.Any("error", err))
	}

	adapter.RegisterWasmPlugins(pluginManager, wasmLoader)

	dialogStore := storage.NewRedisStorage(redisClient)
	logger.Info("using Redis dialog storage")
	stateMgr := state.NewManager(dialogStore)

	adminHandler := adminapi.NewAdminHandler(
		pluginStore,
		blobStore,
		wasmLoader,
		pluginManager,
		rt,
		hostAPI,
		stateMgr,
		cmdPermStore,
		versionStore,
		cfg.Admin.APIKey,
		adminBus,
	)

	adminMux := http.NewServeMux()
	adminHandler.RegisterRoutes(adminMux)
	cmdPermHandler := adminapi.NewCommandPermHandler(cmdPermStore)
	cmdPermHandler.RegisterRoutes(adminMux)
	pluginPermHandler := adminapi.NewPluginPermHandler(pluginStore, wasmLoader, hostAPI, adminBus)
	pluginPermHandler.RegisterRoutes(adminMux)
	chatHandler := adminapi.NewChatHandler(adminChatStore, adapterRegistry)
	chatHandler.RegisterRoutes(adminMux)
	channelStatusHandler := adminapi.NewChannelStatusHandler(adapterRegistry, adminapi.ChannelStatusConfig{
		TelegramConfigured: cfg.Telegram.Token != "",
		DiscordConfigured:  cfg.Discord.Token != "",
	})
	channelStatusHandler.RegisterRoutes(adminMux)
	ruleSchemaHandler.RegisterRoutes(adminMux)
	httpTrigger := trigger.NewHTTPTriggerHandler(triggerRouter, triggerRegistry)
	httpTrigger.SetMetrics(m)
	adminMux.Handle("/api/triggers/http/", httpTrigger)

	adminMux.Handle("GET /metrics", promhttp.Handler())

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
	accountLinker := user.NewAccountLinker(accountRepo)

	allPlugins := []plugin.Plugin{
		core.New(senderAPI, accountLinker, stateMgr, userService, notifPrefsRepo),
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

	cronScheduler.Start()

	updateRouter := plugin.NewUpdateRouter(pluginManager)

	channelMgr := channel.NewChannelManager(
		userService,
		updateRouter,
		stateAdapter,
		pluginManager,
		authorizer,
		adapterRegistry,
		logger,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pluginFetcher := func(fCtx context.Context, id string) (*pubsub.PluginData, error) {
		rec, err := pluginStore.GetPlugin(fCtx, id)
		if err != nil {
			return nil, err
		}
		return &pubsub.PluginData{
			WasmKey:     rec.WasmKey,
			ConfigJSON:  rec.ConfigJSON,
			Permissions: rec.Permissions,
		}, nil
	}
	eventHandler := pubsub.NewAdminEventHandler(
		pluginFetcher, blobStore.Get, wasmLoader, pluginManager, hostAPI, stateMgr,
	)
	go func() {
		if err := adminBus.Subscribe(ctx, eventHandler.Handle); err != nil {
			logger.Error("pubsub subscriber stopped", slog.Any("error", err))
		}
	}()
	logger.Info("pub/sub subscriber started", slog.String("instance", adminBus.InstanceID()))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var commandNames []string
	for _, p := range pluginManager.All() {
		for _, def := range p.Commands() {
			commandNames = append(commandNames, def.Name)
		}
	}

	joinHandler := newChatJoinHandler(chatRegistry, logger)

	if cfg.Telegram.Token != "" {
		logger.Info("starting Telegram bot")
		tgHandler := channel.Chain(channelMgr.OnUpdate, telegram.CallbackNormalizer())
		tgBot, err := telegram.NewBot(cfg.Telegram.Token, tgHandler, joinHandler, logger)
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
		dcBot, err := discord.NewBot(cfg.Discord.Token, channelMgr.OnUpdate, joinHandler, logger)
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

	cronScheduler.Stop()

	if err := wasmLoader.Close(shutdownCtx); err != nil {
		logger.Error("wasm loader close error", slog.Any("error", err))
	}
	if err := rt.Close(shutdownCtx); err != nil {
		logger.Error("wasm runtime close error", slog.Any("error", err))
	}

	logger.Info("SuperBotGo stopped")

	_ = roleManager
}

func generateInstanceID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
