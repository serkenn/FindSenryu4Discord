package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unicode"
	"time"

	"github.com/u16-io/FindSenryu4Discord/commands"
	"github.com/u16-io/FindSenryu4Discord/config"
	"github.com/u16-io/FindSenryu4Discord/db"
	"github.com/u16-io/FindSenryu4Discord/model"
	"github.com/u16-io/FindSenryu4Discord/pkg/adminnotify"
	"github.com/u16-io/FindSenryu4Discord/pkg/backup"
	"github.com/u16-io/FindSenryu4Discord/pkg/health"
	"github.com/u16-io/FindSenryu4Discord/pkg/logger"
	"github.com/u16-io/FindSenryu4Discord/pkg/metrics"
	"github.com/u16-io/FindSenryu4Discord/pkg/permissions"
	"github.com/u16-io/FindSenryu4Discord/pkg/senryuimg"
	"github.com/u16-io/FindSenryu4Discord/pkg/webgui"
	"github.com/u16-io/FindSenryu4Discord/service"

	"github.com/ikawaha/kagome-dict/uni"
	"github.com/0x307e/go-haiku"
	"github.com/bwmarrin/discordgo"
)

var (
	startTime       time.Time
	adminNotifier   *adminnotify.Manager
	botReady        atomic.Bool
	guildCacheTimer atomic.Pointer[time.Timer]
	allSessions     []*discordgo.Session
	expectedShards  atomic.Int32
	connectedShards atomic.Int32

	minTimeoutMinutes float64 = 1
	maxTimeoutMinutes float64 = 1440

	// lastDetectedUser tracks the most recently detected poem author per channel.
	// key = channelID, value = {userID, time}
	lastDetectedMu   sync.RWMutex
	lastDetectedUser = make(map[string]detectedInfo)

	// OptOutPromptPrefix is the custom ID prefix for opt-out prompt buttons.
	OptOutPromptPrefix = "optout_prompt_"

	userCommands = []*discordgo.ApplicationCommand{
		{
			Name:        "help",
			Description: "俳句・川柳・短歌のルールとコマンド一覧を表示します",
		},
		{
			Name:        "mute",
			Description: "このチャンネルでの川柳検出をミュートします（管理者/Bot管理者のみ）",
		},
		{
			Name:        "unmute",
			Description: "このチャンネルでの川柳検出のミュートを解除します（管理者/Bot管理者のみ）",
		},
		{
			Name:        "rank",
			Description: "ギルド内で詠んだ回数が多い人のランキングを表示します",
		},
		{
			Name:        "channel",
			Description: "チャンネルタイプ別の川柳検出設定を変更します（管理者/Bot管理者のみ）",
		},
		{
			Name:        "doctor",
			Description: "このチャンネルでBotが正常に動作するか診断します",
		},
		{
			Name:        "detect",
			Description: "自分の川柳検出のオン/オフを切り替えます",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "on",
					Description: "川柳検出を有効にします",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "off",
					Description: "川柳検出を無効にします",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "status",
					Description: "現在の川柳検出設定を表示します",
				},
			},
		},
		{
			Name:        "blacklist",
			Description: "自分の川柳検出をトグルします（ブラックリスト）",
		},
		{
			Name:        "timeout",
			Description: "川柳検出の一時停止（管理者または許可ロールのみ）",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "minutes",
					Description: "一時停止する分数（1〜1440）",
					Required:    false,
					MinValue:    &minTimeoutMinutes,
					MaxValue:    maxTimeoutMinutes,
				},
			},
		},
		{
			Name:        "timeout-role",
			Description: "timeout権限ロールを管理します（管理者/Bot管理者のみ）",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "add",
					Description: "timeout権限を付与するロールを追加",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionRole,
							Name:        "role",
							Description: "timeout権限を付与するロール",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remove",
					Description: "timeout権限ロールを削除",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionRole,
							Name:        "role",
							Description: "timeout権限を解除するロール",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "timeout権限のあるロール一覧を表示",
				},
			},
		},
		{
			Name:        "compose",
			Description: "上の句・中の句・下の句を指定して川柳を作成します",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "kamigo",
					Description: "上の句",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "nakasichi",
					Description: "中の句",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "simogo",
					Description: "下の句",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "詠み手として指定するユーザー（省略時は自分）",
					Required:    false,
				},
			},
		},
	}

	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"help":      commands.HandleHelpCommand,
		"mute":      handleMuteCommand,
		"unmute":    handleUnmuteCommand,
		"rank":      handleRankCommand,
		"channel":   commands.HandleChannelCommand,
		"doctor":    commands.HandleDoctorCommand,
		"detect":    commands.HandleDetectCommand,
		"admin":     commands.HandleAdminCommand,
		"contact":   commands.HandleContactCommand,
		"blacklist": commands.HandleBlacklistCommand,
		"timeout":      commands.HandleTimeoutCommand,
		"timeout-role": commands.HandleTimeoutRoleCommand,
		"compose":      commands.HandleComposeCommand,
	}
)

func main() {
	startTime = time.Now()

	// Initialize haiku dictionary
	haiku.UseDict(uni.Dict())

	// Initialize fallback mora tokenizer
	initMoraTokenizer()

	// Load configuration
	conf, err := config.Load("config.toml")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger.Init(logger.Config{
		Level:  conf.Log.Level,
		Format: conf.Log.Format,
	})

	logger.Info("Starting FindSenryu4Discord",
		"log_level", conf.Log.Level,
		"db_driver", conf.Database.Driver,
	)

	// Initialize database
	if err := db.Init(); err != nil {
		logger.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}

	// Set font path for senryu image generation
	senryuimg.SetFontPath(conf.Web.FontPath)
	if conf.Web.FallbackFontPath != "" {
		senryuimg.SetFallbackFontPath(conf.Web.FallbackFontPath)
	}

	// Start health check server
	healthServer, err := health.StartServer()
	if err != nil {
		logger.Error("Failed to start health server", "error", err)
	}

	// Start WebGUI server
	webServer, err := webgui.StartServer()
	if err != nil {
		logger.Error("Failed to start WebGUI server", "error", err)
	}

	// Initialize backup manager
	var backupManager *backup.Manager
	if conf.Database.Driver == "sqlite3" && conf.Backup.Enabled {
		backupManager = backup.NewManager(conf.Backup, conf.Database.Path)
		backupManager.Start()
		commands.SetBackupManager(backupManager)
	}
	commands.SetStartTime(startTime)

	// Get recommended shard count from Discord
	tmpSession, err := discordgo.New("Bot " + conf.Discord.Token)
	if err != nil {
		logger.Error("Failed to create Discord session", "error", err)
		os.Exit(1)
	}
	gatewayBot, err := tmpSession.GatewayBot()
	if err != nil {
		logger.Error("Failed to get gateway bot info", "error", err)
		os.Exit(1)
	}
	shardCount := gatewayBot.Shards
	if shardCount < 1 {
		shardCount = 1
	}
	logger.Info("Discord gateway info", "recommended_shards", gatewayBot.Shards, "using_shards", shardCount)

	// Gateway Intents
	intents := discordgo.IntentGuilds |
		discordgo.IntentGuildMessages |
		discordgo.IntentMessageContent

	// Create and open sessions for each shard
	expectedShards.Store(int32(shardCount))
	allSessions = make([]*discordgo.Session, shardCount)
	for i := 0; i < shardCount; i++ {
		s, err := discordgo.New("Bot " + conf.Discord.Token)
		if err != nil {
			logger.Error("Failed to create Discord session", "error", err, "shard", i)
			os.Exit(1)
		}
		s.ShardID = i
		s.ShardCount = shardCount
		s.Identify.Intents = intents

		s.AddHandler(messageCreate)
		s.AddHandler(interactionCreate)
		s.AddHandler(guildCreate)
		s.AddHandler(guildDelete)
		s.AddHandler(onConnect)

		if err := s.Open(); err != nil {
			logger.Error("Failed to open Discord connection", "error", err, "shard", i)
			os.Exit(1)
		}
		logger.Info("Shard connected", "shard_id", i, "shard_count", shardCount)
		allSessions[i] = s

		// Rate limit: wait between shard connections (Discord recommends ~5s)
		if i < shardCount-1 {
			time.Sleep(5 * time.Second)
		}
	}

	// Share all sessions with commands package for cross-shard guild counting
	commands.SetAllSessions(allSessions)

	// Use shard 0 as the primary session for command registration
	dg := allSessions[0]

	// Conditionally add /contact command
	if conf.Admin.ContactChannelID != "" {
		userCommands = append(userCommands, &discordgo.ApplicationCommand{
			Name:        "contact",
			Description: "Bot管理者にお問い合わせを送信します",
		})
	}

	// Register user commands (global)
	logger.Info("Registering user slash commands...")
	registeredUserCommands := make([]*discordgo.ApplicationCommand, len(userCommands))
	for i, cmd := range userCommands {
		rcmd, err := dg.ApplicationCommandCreate(dg.State.User.ID, "", cmd)
		if err != nil {
			logger.Error("Failed to register command", "command", cmd.Name, "error", err)
		} else {
			registeredUserCommands[i] = rcmd
			logger.Info("Registered command", "command", cmd.Name)
		}
	}

	// Register admin commands (guild-specific)
	adminGuildID := permissions.GetAdminGuildID()
	var registeredAdminCommands []*discordgo.ApplicationCommand
	if adminGuildID != "" {
		logger.Info("Registering admin slash commands...", "guild_id", adminGuildID)
		for _, cmd := range commands.AdminCommands() {
			rcmd, err := dg.ApplicationCommandCreate(dg.State.User.ID, adminGuildID, cmd)
			if err != nil {
				logger.Error("Failed to register admin command", "command", cmd.Name, "error", err)
			} else {
				registeredAdminCommands = append(registeredAdminCommands, rcmd)
				logger.Info("Registered admin command", "command", cmd.Name, "guild_id", adminGuildID)
			}
		}
	}

	// Update game status
	dg.UpdateGameStatus(1, conf.Discord.Playing)

	// Update database stats in metrics
	dbStats := db.GetStats()
	metrics.SetDatabaseStats(dbStats.SenryuCount, dbStats.MutedChannelCount)

	// Initialize admin notification manager
	if conf.Admin.LogChannelID != "" || conf.Admin.ReportChannelID != "" {
		adminNotifier = adminnotify.NewManager(dg, conf.Admin.LogChannelID, conf.Admin.ReportChannelID)
		adminNotifier.SetAllSessions(allSessions)
		adminNotifier.Start()
		adminNotifier.NotifyStarted()
	}
	// Start periodic cleanup (every 10 minutes)
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			service.CleanupExpiredTimeouts()
			service.CleanupExpiredCombos()
		}
	}()

	botReady.Store(true)

	// Mark as ready
	if healthServer != nil {
		healthServer.SetReady(true)
	}

	logger.Info("Bot is now running. Press CTRL-C to exit.")

	// Wait for termination signal
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Graceful shutdown
	logger.Info("Shutting down...")

	// Mark as not ready
	if healthServer != nil {
		healthServer.SetReady(false)
	}

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop admin notification manager
	if adminNotifier != nil {
		adminNotifier.NotifyStopping()
		adminNotifier.Stop(ctx)
	}

	// Stop backup manager
	if backupManager != nil {
		backupManager.Stop(ctx)
	}

	// Stop WebGUI server
	if webServer != nil {
		if err := webServer.Stop(ctx); err != nil {
			logger.Error("Failed to stop WebGUI server", "error", err)
		}
	}

	// Stop health server
	if healthServer != nil {
		if err := healthServer.Stop(ctx); err != nil {
			logger.Error("Failed to stop health server", "error", err)
		}
	}

	// Remove slash commands
	logger.Info("Removing user slash commands...")
	for _, cmd := range registeredUserCommands {
		if cmd != nil {
			if err := dg.ApplicationCommandDelete(dg.State.User.ID, "", cmd.ID); err != nil {
				logger.Error("Failed to delete command", "command", cmd.Name, "error", err)
			}
		}
	}

	// Remove admin commands
	if adminGuildID != "" {
		logger.Info("Removing admin slash commands...")
		for _, cmd := range registeredAdminCommands {
			if cmd != nil {
				if err := dg.ApplicationCommandDelete(dg.State.User.ID, adminGuildID, cmd.ID); err != nil {
					logger.Error("Failed to delete admin command", "command", cmd.Name, "error", err)
				}
			}
		}
	}

	// Close all Discord shard connections
	for _, s := range allSessions {
		if err := s.Close(); err != nil {
			logger.Error("Failed to close Discord connection", "error", err, "shard", s.ShardID)
		}
	}

	// Close database
	if err := db.Close(); err != nil {
		logger.Error("Failed to close database", "error", err)
	}

	logger.Info("Shutdown complete")
}

func onConnect(s *discordgo.Session, _ *discordgo.Connect) {
	n := connectedShards.Add(1)
	logger.Info("Gateway connected, caching guilds...", "shard", s.ShardID, "connected_shards", n, "expected_shards", expectedShards.Load())
	botReady.Store(false)
	// Reset debounce timer on new shard connection to prevent premature ready
	if t := guildCacheTimer.Load(); t != nil {
		t.Stop()
	}
}

func countAllGuilds() int {
	total := 0
	for _, s := range allSessions {
		if s != nil {
			total += len(s.State.Guilds)
		}
	}
	return total
}

func guildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	metrics.SetConnectedGuilds(countAllGuilds())
	if !botReady.Load() {
		logger.Debug("Guild cache", "name", g.Name, "id", g.ID)
		// Debounce: reset timer on each GUILD_CREATE during cache burst.
		// When no more events arrive within 5s, mark as ready.
		if t := guildCacheTimer.Load(); t != nil {
			t.Stop()
		}
		t := time.AfterFunc(5*time.Second, func() {
			if connectedShards.Load() < expectedShards.Load() {
				// Not all shards connected yet; wait for remaining shards
				logger.Info("Guild cache paused, waiting for remaining shards",
					"guilds", countAllGuilds(),
					"connected_shards", connectedShards.Load(),
					"expected_shards", expectedShards.Load(),
				)
				return
			}
			total := countAllGuilds()
			logger.Info("Guild cache complete, bot is ready", "guilds", total, "shards", expectedShards.Load())
			metrics.SetConnectedGuilds(total)
			botReady.Store(true)
		})
		guildCacheTimer.Store(t)
		return
	}
	logger.Info("Joined guild", "name", g.Name, "id", g.ID)
	if adminNotifier != nil {
		adminNotifier.NotifyGuildJoin(g.Guild)
	}
}

func guildDelete(s *discordgo.Session, g *discordgo.GuildDelete) {
	logger.Info("Left guild", "id", g.ID)
	metrics.SetConnectedGuilds(countAllGuilds())

	// Clean up guild data
	senryuCount, err := service.DeleteSenryuByServer(g.ID)
	if err != nil {
		logger.Error("Failed to clean up guild data", "error", err, "guild_id", g.ID, "type", "senryus")
	}
	optOutCount, err := service.DeleteOptOutByServer(g.ID)
	if err != nil {
		logger.Error("Failed to clean up guild data", "error", err, "guild_id", g.ID, "type", "opt-outs")
	}
	channelConfigCount, err := service.DeleteChannelConfigByGuild(g.ID)
	if err != nil {
		logger.Error("Failed to clean up guild data", "error", err, "guild_id", g.ID, "type", "channel-config")
	}

	logger.Info("Guild data cleaned up",
		"guild_id", g.ID,
		"senryus", senryuCount,
		"opt_outs", optOutCount,
		"channel_configs", channelConfigCount,
	)

	if botReady.Load() && adminNotifier != nil {
		adminNotifier.NotifyGuildLeave(g, senryuCount, optOutCount)
	}
}

func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	case discordgo.InteractionMessageComponent:
		handleComponentInteraction(s, i)
	case discordgo.InteractionModalSubmit:
		handleModalSubmitInteraction(s, i)
	}
}

func handleModalSubmitInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.ModalSubmitData().CustomID
	switch {
	case customID == commands.ContactModalCustomID:
		commands.HandleContactModalSubmit(s, i)
	case strings.HasPrefix(customID, commands.ReplyModalPrefix):
		commands.HandleContactReplyModalSubmit(s, i)
	}
}

func handleComponentInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID

	switch {
	case strings.HasPrefix(customID, commands.ContactReplyPrefix):
		commands.HandleContactReplyButton(s, i)
	case strings.HasPrefix(customID, commands.ChannelTogglePrefix):
		commands.HandleChannelToggle(s, i)
	case strings.HasPrefix(customID, OptOutPromptPrefix):
		handleOptOutPromptButton(s, i, customID)
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot {
		return
	}

	metrics.RecordMessageProcessed()

	ch, err := s.State.Channel(m.ChannelID)
	if err != nil {
		ch, err = s.Channel(m.ChannelID)
		if err != nil {
			logger.Warn("Failed to get channel", "error", err, "channel_id", m.ChannelID)
			metrics.RecordError("discord_api")
			return
		}
	}

	// DM channels are not supported
	switch ch.Type {
	case discordgo.ChannelTypeDM, discordgo.ChannelTypeGroupDM:
		s.ChannelMessageSend(m.ChannelID, "個チャはダメです")
		return
	}

	// Check if this channel type is enabled for the guild
	if !service.IsChannelTypeEnabled(m.GuildID, ch.Type) {
		return
	}

	// Check for abusive replies to the bot's messages
	if handleAbusiveReply(s, m) {
		return
	}

	// Check if a recently-detected user is sending abusive follow-up
	if checkPostDetectionAbuse(s, m) {
		return
	}

	if handleYomeYomuna(m, s) {
		return
	}

	if !service.IsMute(m.ChannelID) && !isParentChannelMuted(ch) {
		if m.Author.ID != s.State.User.ID {
			if service.IsDetectionOptedOut(m.GuildID, m.Author.ID) {
				return
			}
			if service.IsTimedOut(m.ChannelID, m.Author.ID) {
				return
			}
			if containsDiscordTokens(m.Content) {
				return
			}
			content := m.Content
			spoiler := containsSpoiler(content)
			if spoiler {
				content = stripSpoilerMarkers(content)
			}
			// 1. Check free-form haiku whitelist first
			if match := matchJiyuritsu(content); match != nil {
				handleJiyuritsuMatch(s, m, match, spoiler)
				return
			}

			// Normalize full-width characters for better detection
			normalizedContent := normalizeForDetection(content)

			// Strip spaces/punctuation for length check
			contentRunes := []rune(strings.ReplaceAll(strings.ReplaceAll(normalizedContent, " ", ""), "　", ""))

			// --- False-positive prevention filters ---
			detConf := config.GetConf().Detection

			// a) Check Japanese character ratio
			if japaneseRatio(normalizedContent) < detConf.MinJapaneseRatio {
				return
			}

			// b) Check for mid-text sentence punctuation (会話文フィルタ)
			if hasMiddleSentencePunctuation(normalizedContent) {
				return
			}

			// c) Check for known false-positive phrases
			if isFalsePositivePhrase(normalizedContent) {
				return
			}

			// 2. Try tanka detection (5-7-5-7-7) — longer pattern first
			// Max ~50 chars to prevent false positives from long messages
			if len(contentRunes) >= detConf.MinTankaRunes && len(contentRunes) <= 80 {
				t := findHaikuSafe(normalizedContent, []int{5, 7, 5, 7, 7})
				if len(t) != 0 {
					parts := strings.Split(t[0], " ")
					if len(parts) == 5 {
						handlePoemDetected(s, m, parts, model.PoemTypeTanka, spoiler)
						return
					}
				}
				// Fallback: try kagome-based mora counting for tanka
				if fbParts := fallbackHaikuDetect(normalizedContent, []int{5, 7, 5, 7, 7}); len(fbParts) == 5 {
					handlePoemDetected(s, m, fbParts, model.PoemTypeTanka, spoiler)
					return
				}
			}

			// 3. Try 五言律詩 detection (5×8 = 40 kanji)
			if goGenPhrases := detectGoGenRisshi(normalizedContent); goGenPhrases != nil {
				handleGoGenRisshiDetected(s, m, goGenPhrases, spoiler)
				return
			}

			// 4. Try senryu/haiku detection (5-7-5)
			// Min/max char limits to prevent false positives
			if len(contentRunes) >= detConf.MinSenryuRunes && len(contentRunes) <= 50 {
				var detected []string
				h := findHaikuSafe(normalizedContent, []int{5, 7, 5})
				if len(h) != 0 {
					parts := strings.Split(h[0], " ")
					if len(parts) == 3 {
						detected = parts
					}
				}
				if detected == nil {
					// Fallback: try kagome-based mora counting
					if fbParts := fallbackHaikuDetect(normalizedContent, []int{5, 7, 5}); len(fbParts) == 3 {
						detected = fbParts
					}
				}
				if detected != nil {
					// Check for kigo to distinguish haiku vs senryu
					fullText := strings.Join(detected, "")
					kigoResult := detectKigo(fullText)
					if kigoResult != nil {
						handleHaikuDetected(s, m, detected, kigoResult, spoiler)
					} else {
						handlePoemDetected(s, m, detected, model.PoemTypeSenryu, spoiler)
					}
				}
			}
		}
	}
}

var medals = []string{"🥇", "🥈", "🥉", "🎖️", "🎖️"}

// canMuteUnmute checks if the user has permission to mute/unmute.
// Allowed: Discord administrators or bot owner_ids.
func canMuteUnmute(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	var userID string
	if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	} else if i.User != nil {
		userID = i.User.ID
	}

	// Bot owners always allowed
	if permissions.IsOwner(userID) {
		return true
	}

	// Check Discord administrator permission
	if i.Member != nil {
		perms := i.Member.Permissions
		if perms&discordgo.PermissionAdministrator != 0 {
			return true
		}
	}

	return false
}

func handleMuteCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	metrics.RecordCommandExecuted("mute")

	if !canMuteUnmute(s, i) {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "このコマンドは管理者のみ使用できます 🚫",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if err := service.ToMute(i.ChannelID, i.GuildID); err != nil {
		logger.Error("Failed to mute channel", "error", err, "channel_id", i.ChannelID)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "ミュートに失敗しました ❌",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	} else {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "このチャンネルでの川柳検出をミュートしました ✅",
			},
		})
	}
}

func handleUnmuteCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	metrics.RecordCommandExecuted("unmute")

	if !canMuteUnmute(s, i) {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "このコマンドは管理者のみ使用できます 🚫",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if err := service.ToUnMute(i.ChannelID); err != nil {
		logger.Error("Failed to unmute channel", "error", err, "channel_id", i.ChannelID)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "ミュート解除に失敗しました ❌",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	} else {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "このチャンネルでの川柳検出のミュートを解除しました ✅",
			},
		})
	}
}

func handleRankCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	metrics.RecordCommandExecuted("rank")

	ranks, err := service.GetRanking(i.GuildID)
	if err != nil {
		logger.Error("Failed to get ranking", "error", err, "guild_id", i.GuildID)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "ランキングの取得に失敗しました",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	guild, err := s.State.Guild(i.GuildID)
	if err != nil {
		guild, err = s.Guild(i.GuildID)
		if err != nil {
			logger.Warn("Failed to get guild for rank embed", "error", err, "guild_id", i.GuildID)
		}
	}

	embed := discordgo.MessageEmbed{
		Type:      discordgo.EmbedTypeRich,
		Title:     "サーバー内ランキング",
		Timestamp: time.Now().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{},
	}
	if guild != nil {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text:    guild.Name,
			IconURL: guild.IconURL(""),
		}
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: guild.IconURL(""),
		}
	}

	for _, rank := range ranks {
		member, err := s.GuildMember(i.GuildID, rank.AuthorId)
		if err != nil {
			continue
		}
		displayName := member.Nick
		if displayName == "" {
			displayName = member.User.GlobalName
		}
		if displayName == "" {
			displayName = member.User.Username
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s 第%d位: %d回", medals[rank.Rank-1], rank.Rank, rank.Count),
			Value:  displayName,
			Inline: true,
		})
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{&embed},
		},
	})
}

func handleYomeYomuna(m *discordgo.MessageCreate, s *discordgo.Session) bool {
	switch m.Content {
	case "詠め":
		senryus, err := service.GetThreeRandomSenryus(m.GuildID)
		if err != nil {
			logger.Error("Failed to get random senryus", "error", err)
			s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
			return true
		}
		if len(senryus) == 0 {
			if _, err := s.ChannelMessageSend(m.ChannelID, "まだ誰も詠んでいません。あなたが先に詠んでください。"); err != nil {
				logger.Warn("Failed to send message", "error", err, "channel_id", m.ChannelID)
			}
		} else {
			if _, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("ここで一句\n「%s」\n詠み手: %s",
				strings.Join([]string{
					senryus[0].Kamigo,
					senryus[1].Nakasichi,
					senryus[2].Simogo,
				}, " "), strings.Join(getWriters(senryus, m.GuildID, s), ", "))); err != nil {
				logger.Warn("Failed to send senryu message", "error", err, "channel_id", m.ChannelID)
			}
		}
		return true
	}
	return false
}

// abusiveWords contains words/phrases that trigger auto-blacklisting when
// directed at the bot in a reply.
var abusiveWords = []string{
	"死ね", "しね", "シネ",
	"くたばれ", "消えろ", "きえろ",
	"うざい", "ウザい", "うぜえ", "ウゼえ", "うぜー", "ウゼー",
	"邪魔", "じゃま",
	"きもい", "キモい", "きめえ", "キメえ",
	"ゴミ", "ごみ", "カス", "かす",
	"クソ", "くそ", "糞",
	"黙れ", "だまれ", "黙って",
	"殺す", "ころす",
	"失せろ", "うせろ",
	"氏ね", "タヒね",
}

// containsAbusiveWord checks if the text contains any abusive word.
func containsAbusiveWord(text string) bool {
	lower := strings.ToLower(text)
	for _, word := range abusiveWords {
		if strings.Contains(lower, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

// isBotMention checks if the message mentions the bot by name or by @mention.
func isBotMention(s *discordgo.Session, content string) bool {
	botUser := s.State.User
	if botUser == nil {
		return false
	}
	// Check for @mention
	if strings.Contains(content, "<@"+botUser.ID+">") || strings.Contains(content, "<@!"+botUser.ID+">") {
		return true
	}
	// Check for bot username/display name in text
	lower := strings.ToLower(content)
	if botUser.Username != "" && strings.Contains(lower, strings.ToLower(botUser.Username)) {
		return true
	}
	if botUser.GlobalName != "" && strings.Contains(lower, strings.ToLower(botUser.GlobalName)) {
		return true
	}
	return false
}

// handleAbusiveReply detects abusive replies/messages directed at the bot and auto-blacklists the user.
// Triggers on: (1) replies to bot messages containing abuse, or (2) messages mentioning the bot by name with abuse.
// Returns true if an abusive message was handled.
func handleAbusiveReply(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	isReplyToBot := false
	if m.MessageReference != nil && m.MessageReference.MessageID != "" {
		refMsg, err := s.ChannelMessage(m.ChannelID, m.MessageReference.MessageID)
		if err == nil && refMsg.Author != nil && refMsg.Author.ID == s.State.User.ID {
			isReplyToBot = true
		}
	}

	mentionsBot := isBotMention(s, m.Content)

	// Only check if the message is directed at the bot
	if !isReplyToBot && !mentionsBot {
		return false
	}

	if !containsAbusiveWord(m.Content) {
		return false
	}

	// Auto-blacklist the user
	if err := service.OptOutDetection(m.GuildID, m.Author.ID); err != nil {
		logger.Error("Failed to auto-blacklist abusive user", "error", err, "user_id", m.Author.ID)
		return false
	}

	logger.Info("Auto-blacklisted user for abusive reply",
		"user_id", m.Author.ID,
		"guild_id", m.GuildID,
		"content", m.Content,
	)

	// Notify the user
	replyText := fmt.Sprintf("<@%s> 暴言が検出されたため、このサーバーでの川柳検出を自動的にオフにしました。\n再度有効にするには `/detect on` を使用してください。", m.Author.ID)
	s.ChannelMessageSendReply(m.ChannelID, replyText, m.Reference())

	// Notify admin
	if adminNotifier != nil {
		displayName := getDisplayName(s, m.GuildID, m.Author)
		adminNotifier.NotifyLog(fmt.Sprintf("⚠️ 暴言による自動ブラックリスト\nユーザー: %s (`%s`)\nサーバー: `%s`\n内容: %s",
			displayName, m.Author.ID, m.GuildID, m.Content))
	}

	return true
}

// detectedInfo holds info about the last user whose poem was detected in a channel.
type detectedInfo struct {
	userID string
	at     time.Time
}

// recordDetectedUser records that a user's poem was detected in a channel.
func recordDetectedUser(channelID, userID string) {
	lastDetectedMu.Lock()
	defer lastDetectedMu.Unlock()
	lastDetectedUser[channelID] = detectedInfo{userID: userID, at: time.Now()}
}

// checkPostDetectionAbuse checks if the message author was just detected and
// their follow-up message contains abusive language. If so, sends an ephemeral-style
// opt-out prompt visible only to the user.
// Returns true if a prompt was sent (caller should stop processing).
func checkPostDetectionAbuse(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	lastDetectedMu.RLock()
	info, ok := lastDetectedUser[m.ChannelID]
	lastDetectedMu.RUnlock()

	if !ok || info.userID != m.Author.ID {
		return false
	}
	// Only within 2 minutes of detection
	if time.Since(info.at) > 2*time.Minute {
		return false
	}

	if !containsAbusiveWord(m.Content) {
		return false
	}

	// Clear the tracking so we don't prompt again
	lastDetectedMu.Lock()
	delete(lastDetectedUser, m.ChannelID)
	lastDetectedMu.Unlock()

	// Send opt-out prompt with button (auto-delete after 30s)
	msg, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content: fmt.Sprintf("<@%s> あなたの発言を川柳として切り抜かないように設定しますか？", m.Author.ID),
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "はい、オフにする",
						Style:    discordgo.DangerButton,
						CustomID: OptOutPromptPrefix + "yes_" + m.Author.ID,
					},
					discordgo.Button{
						Label:    "いいえ、そのまま",
						Style:    discordgo.SecondaryButton,
						CustomID: OptOutPromptPrefix + "no_" + m.Author.ID,
					},
				},
			},
		},
	})
	if err != nil {
		logger.Warn("Failed to send opt-out prompt", "error", err)
		return false
	}

	// Auto-delete after 30 seconds
	time.AfterFunc(30*time.Second, func() {
		_ = s.ChannelMessageDelete(m.ChannelID, msg.ID)
	})

	return true
}

// handleOptOutPromptButton handles the opt-out prompt button interaction.
func handleOptOutPromptButton(s *discordgo.Session, i *discordgo.InteractionCreate, customID string) {
	// Extract action and target user ID: "optout_prompt_yes_123456" or "optout_prompt_no_123456"
	rest := strings.TrimPrefix(customID, OptOutPromptPrefix)
	parts := strings.SplitN(rest, "_", 2)
	if len(parts) != 2 {
		return
	}
	action, targetUserID := parts[0], parts[1]

	// Only the target user can click the button
	var clickerID string
	if i.Member != nil && i.Member.User != nil {
		clickerID = i.Member.User.ID
	} else if i.User != nil {
		clickerID = i.User.ID
	}
	if clickerID != targetUserID {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "このボタンはあなた宛てではありません。",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	switch action {
	case "yes":
		if err := service.OptOutDetection(i.GuildID, targetUserID); err != nil {
			logger.Error("Failed to opt-out via prompt", "error", err, "user_id", targetUserID)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "設定に失敗しました。あとで `/detect off` をお試しください。",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "川柳検出をオフにしました。再度有効にするには `/detect on` を使用してください。",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	case "no":
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "そのままにします。引き続き川柳を検出します！",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	// Delete the prompt message
	if i.Message != nil {
		_ = s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
	}
}

// isParentChannelMuted checks if the parent channel of a thread is muted.
func isParentChannelMuted(ch *discordgo.Channel) bool {
	if ch.ParentID == "" {
		return false
	}
	return service.IsMute(ch.ParentID)
}

func sliceUnique(target []string) (unique []string) {
	m := map[string]bool{}
	for _, v := range target {
		if !m[v] {
			m[v] = true
			unique = append(unique, v)
		}
	}
	return unique
}

// containsDiscordTokens reports whether s contains Discord-specific tokens
// (mentions, channels, roles, custom emoji, URLs) that should exclude
// the message from haiku detection.
var reDiscordTokens = regexp.MustCompile(
	`<@!?\d+>` + // user mentions
		`|<#\d+>` + // channel mentions
		`|<@&\d+>` + // role mentions
		`|<a?:\w+:\d+>` + // custom emoji
		`|https?://\S+`, // URLs
)

func containsDiscordTokens(s string) bool {
	return reDiscordTokens.MatchString(s)
}

// --- False-positive prevention filters ---

// isJapaneseChar returns true for hiragana, katakana, or kanji.
func isJapaneseChar(r rune) bool {
	return unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Han)
}

// japaneseRatio returns the ratio of Japanese characters (hiragana+katakana+kanji)
// to total non-whitespace runes.
func japaneseRatio(s string) float64 {
	var total, jp int
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		total++
		if isJapaneseChar(r) {
			jp++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(jp) / float64(total)
}

// hasMiddleSentencePunctuation returns true if sentence-ending punctuation
// (。！？!?) appears anywhere except the very end of the text.
// Poems typically don't have sentence breaks in the middle.
func hasMiddleSentencePunctuation(s string) bool {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) < 2 {
		return false
	}
	// Check all characters except the last one
	for _, r := range runes[:len(runes)-1] {
		switch r {
		case '。', '！', '？', '!', '?':
			return true
		}
	}
	return false
}

// falsePositivePhrases contains normalized substrings that commonly trigger
// false positive detections. Checked after normalizeForMatch.
var falsePositivePhrases = []string{
	"おはようございます",
	"おやすみなさい",
	"ありがとうございます",
	"よろしくおねがいします",
	"おつかれさまです",
	"おつかれさまでした",
	"いただきます",
	"ごちそうさまでした",
	"いってきます",
	"ただいま",
	"おかえりなさい",
	"それはいいですね",
	"わかりました",
	"なるほどですね",
	"すみませんでした",
	"ごめんなさい",
	"こんにちは",
	"こんばんは",
	"さようなら",
}

// isFalsePositivePhrase checks if the normalized content matches a known
// false-positive phrase.
func isFalsePositivePhrase(content string) bool {
	normalized := normalizeForMatch(content)
	for _, phrase := range falsePositivePhrases {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	return false
}

// findHaikuSafe wraps haiku.Find with recover to prevent panics from crashing the bot.
func findHaikuSafe(content string, rule []int) (result []string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Warn("Recovered from panic in haiku.Find", "error", r, "content_len", len(content))
			result = nil
		}
	}()
	return haiku.Find(content, rule)
}

// isKanji returns true if the rune is a CJK unified ideograph.
func isKanji(r rune) bool {
	return unicode.Is(unicode.Han, r)
}

// detectGoGenRisshi checks if the content is a 五言律詩 (5-character × 8-line Chinese regulated verse).
// Returns the 8 phrases if detected, nil otherwise.
func detectGoGenRisshi(content string) []string {
	// Remove all whitespace and punctuation
	var runes []rune
	for _, r := range content {
		if isKanji(r) {
			runes = append(runes, r)
		}
	}

	// 五言律詩 = exactly 40 kanji characters, split into 8 lines of 5
	if len(runes) != 40 {
		return nil
	}

	// Verify the original content is mostly kanji (allow some punctuation/whitespace)
	totalRunes := []rune(content)
	kanjiRatio := float64(len(runes)) / float64(len(totalRunes))
	if kanjiRatio < 0.7 {
		return nil
	}

	phrases := make([]string, 8)
	for i := 0; i < 8; i++ {
		phrases[i] = string(runes[i*5 : (i+1)*5])
	}
	return phrases
}

// normalizeForDetection preprocesses text for better haiku detection.
// Converts full-width numbers/letters to half-width, normalizes punctuation.
func normalizeForDetection(s string) string {
	var result []rune
	for _, r := range s {
		switch {
		// Full-width digits → half-width
		case r >= '０' && r <= '９':
			result = append(result, r-'０'+'0')
		// Full-width upper letters → half-width
		case r >= 'Ａ' && r <= 'Ｚ':
			result = append(result, r-'Ａ'+'A')
		// Full-width lower letters → half-width
		case r >= 'ａ' && r <= 'ｚ':
			result = append(result, r-'ａ'+'a')
		default:
			result = append(result, r)
		}
	}
	return string(result)
}

var reSpoiler = regexp.MustCompile(`\|\|.+?\|\|`)

func containsSpoiler(s string) bool {
	return reSpoiler.MatchString(s)
}

func stripSpoilerMarkers(s string) string {
	return strings.ReplaceAll(s, "||", "")
}

// handlePoemDetected handles a detected senryu or tanka and sends a reply with image.
func handlePoemDetected(s *discordgo.Session, m *discordgo.MessageCreate, parts []string, poemType string, spoiler bool) {
	rec := model.Senryu{
		ServerID:  m.GuildID,
		AuthorID:  m.Author.ID,
		Kamigo:    parts[0],
		Nakasichi: parts[1],
		Simogo:    parts[2],
		Type:      poemType,
		Spoiler:   &spoiler,
	}
	if poemType == model.PoemTypeTanka && len(parts) >= 5 {
		rec.Shiku = parts[3]
		rec.Goku = parts[4]
	}

	created, err := service.CreateSenryu(rec)
	if err != nil {
		logger.Error("Failed to create poem", "error", err, "type", poemType)
		metrics.RecordError("database")
		s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
		return
	}

	// Record combo
	combo := service.RecordCombo(m.GuildID, m.Author.ID)
	comboText := service.GetComboText(combo)

	fullText := strings.Join(parts, " ")
	typeName := "川柳"
	if poemType == model.PoemTypeTanka {
		typeName = "短歌"
	}
	replyText := fmt.Sprintf("%sを検出しました！\n「%s」", typeName, fullText)
	if spoiler {
		replyText = fmt.Sprintf("%sを検出しました！\n||「%s」||", typeName, fullText)
	}
	if comboText != "" {
		replyText += "\n" + comboText
	}

	msg := &discordgo.MessageSend{
		Content:   replyText,
		Reference: m.Reference(),
	}

	authorName := getDisplayName(s, m.GuildID, m.Author)
	avatarURL := m.Author.AvatarURL("128")

	var bgData []byte
	if bg, bgErr := service.GetBackground(m.GuildID); bgErr == nil && bg != nil {
		if data, readErr := os.ReadFile(bg.FilePath); readErr == nil {
			bgData = data
		}
	}

	renderOpts := senryuimg.RenderOptions{
		AuthorName: authorName,
		AvatarURL:  avatarURL,
		Background: bgData,
	}
	if len(parts) >= 1 {
		renderOpts.Kamigo = parts[0]
	}
	if len(parts) >= 2 {
		renderOpts.Nakasichi = parts[1]
	}
	if len(parts) >= 3 {
		renderOpts.Simogo = parts[2]
	}
	if len(parts) >= 4 {
		renderOpts.Shiku = parts[3]
	}
	if len(parts) >= 5 {
		renderOpts.Goku = parts[4]
	}

	imgData, imgErr := senryuimg.RenderSenryu(renderOpts)
	if imgErr != nil {
		logger.Warn("Failed to render poem image, sending text only", "error", imgErr)
	} else {
		msg.Files = []*discordgo.File{{
			Name:        poemType + ".webp",
			ContentType: "image/webp",
			Reader:      bytes.NewReader(imgData),
		}}
	}

	if _, err := s.ChannelMessageSendComplex(m.ChannelID, msg); err != nil {
		logger.Warn("Failed to send poem reply", "error", err, "channel_id", m.ChannelID)
		s.MessageReactionAdd(m.ChannelID, m.ID, "⚠️")
		if delErr := service.DeleteSenryu(int(created.ID), m.GuildID); delErr != nil {
			logger.Error("Failed to rollback poem after reply failure", "error", delErr, "senryu_id", created.ID)
		} else {
			logger.Info("Rolled back poem after reply failure", "senryu_id", created.ID, "channel_id", m.ChannelID)
		}
	}

	// Track detected user for post-detection abuse check
	recordDetectedUser(m.ChannelID, m.Author.ID)

	// Auto-cooldown: prevent rapid re-detection of the same user
	cooldown := config.GetConf().Detection.AutoCooldownSeconds
	if cooldown > 0 {
		service.SetTimeout(m.ChannelID, m.Author.ID, time.Duration(cooldown)*time.Second)
	}
}

// handleHaikuDetected handles a detected 5-7-5 poem that contains a kigo (季語).
func handleHaikuDetected(s *discordgo.Session, m *discordgo.MessageCreate, parts []string, kigo *KigoResult, spoiler bool) {
	rec := model.Senryu{
		ServerID:  m.GuildID,
		AuthorID:  m.Author.ID,
		Kamigo:    parts[0],
		Nakasichi: parts[1],
		Simogo:    parts[2],
		Type:      model.PoemTypeHaiku,
		Spoiler:   &spoiler,
	}

	created, err := service.CreateSenryu(rec)
	if err != nil {
		logger.Error("Failed to create haiku", "error", err)
		metrics.RecordError("database")
		s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
		return
	}

	// Record combo
	combo := service.RecordCombo(m.GuildID, m.Author.ID)
	comboText := service.GetComboText(combo)

	fullText := strings.Join(parts, " ")
	seasonInfo := fmt.Sprintf("%s %s（季語: %s）", kigo.Season.SeasonEmoji(), kigo.Season.SeasonName(), kigo.Word)
	replyText := fmt.Sprintf("俳句を検出しました！\n「%s」\n%s", fullText, seasonInfo)
	if spoiler {
		replyText = fmt.Sprintf("俳句を検出しました！\n||「%s」||\n%s", fullText, seasonInfo)
	}
	if comboText != "" {
		replyText += "\n" + comboText
	}

	msg := &discordgo.MessageSend{
		Content:   replyText,
		Reference: m.Reference(),
	}

	authorName := getDisplayName(s, m.GuildID, m.Author)
	avatarURL := m.Author.AvatarURL("128")

	var bgData []byte
	if bg, bgErr := service.GetBackground(m.GuildID); bgErr == nil && bg != nil {
		if data, readErr := os.ReadFile(bg.FilePath); readErr == nil {
			bgData = data
		}
	}

	renderOpts := senryuimg.RenderOptions{
		AuthorName: authorName,
		AvatarURL:  avatarURL,
		Background: bgData,
		Kamigo:     parts[0],
		Nakasichi:  parts[1],
		Simogo:     parts[2],
	}

	imgData, imgErr := senryuimg.RenderSenryu(renderOpts)
	if imgErr != nil {
		logger.Warn("Failed to render haiku image, sending text only", "error", imgErr)
	} else {
		msg.Files = []*discordgo.File{{
			Name:        "haiku.webp",
			ContentType: "image/webp",
			Reader:      bytes.NewReader(imgData),
		}}
	}

	if _, err := s.ChannelMessageSendComplex(m.ChannelID, msg); err != nil {
		logger.Warn("Failed to send haiku reply", "error", err, "channel_id", m.ChannelID)
		s.MessageReactionAdd(m.ChannelID, m.ID, "⚠️")
		if delErr := service.DeleteSenryu(int(created.ID), m.GuildID); delErr != nil {
			logger.Error("Failed to rollback haiku after reply failure", "error", delErr, "senryu_id", created.ID)
		}
	}

	// Track detected user for post-detection abuse check
	recordDetectedUser(m.ChannelID, m.Author.ID)

	// Auto-cooldown
	cooldown := config.GetConf().Detection.AutoCooldownSeconds
	if cooldown > 0 {
		service.SetTimeout(m.ChannelID, m.Author.ID, time.Duration(cooldown)*time.Second)
	}
}

// handleJiyuritsuMatch handles a free-form haiku whitelist match.
func handleJiyuritsuMatch(s *discordgo.Session, m *discordgo.MessageCreate, match *jiyuritsuEntry, spoiler bool) {
	rec := model.Senryu{
		ServerID:  m.GuildID,
		AuthorID:  m.Author.ID,
		Kamigo:    match.Text,
		Type:      model.PoemTypeJiyuritsu,
		Spoiler:   &spoiler,
	}

	created, err := service.CreateSenryu(rec)
	if err != nil {
		logger.Error("Failed to create jiyuritsu record", "error", err)
		metrics.RecordError("database")
		s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
		return
	}

	// Record combo
	combo := service.RecordCombo(m.GuildID, m.Author.ID)
	comboText := service.GetComboText(combo)

	replyText := fmt.Sprintf("自由律俳句を検出しました！\n「%s」\n— %s", match.Text, match.Author)
	if spoiler {
		replyText = fmt.Sprintf("自由律俳句を検出しました！\n||「%s」||\n— %s", match.Text, match.Author)
	}
	if comboText != "" {
		replyText += "\n" + comboText
	}

	msg := &discordgo.MessageSend{
		Content:   replyText,
		Reference: m.Reference(),
	}

	authorName := getDisplayName(s, m.GuildID, m.Author)
	avatarURL := m.Author.AvatarURL("128")

	var bgData []byte
	if bg, bgErr := service.GetBackground(m.GuildID); bgErr == nil && bg != nil {
		if data, readErr := os.ReadFile(bg.FilePath); readErr == nil {
			bgData = data
		}
	}

	imgData, imgErr := senryuimg.RenderSenryu(senryuimg.RenderOptions{
		Kamigo:     match.Text,
		AuthorName: authorName,
		AvatarURL:  avatarURL,
		Background: bgData,
	})
	if imgErr != nil {
		logger.Warn("Failed to render jiyuritsu image, sending text only", "error", imgErr)
	} else {
		msg.Files = []*discordgo.File{{
			Name:        "jiyuritsu.webp",
			ContentType: "image/webp",
			Reader:      bytes.NewReader(imgData),
		}}
	}

	if _, sendErr := s.ChannelMessageSendComplex(m.ChannelID, msg); sendErr != nil {
		logger.Warn("Failed to send jiyuritsu reply", "error", sendErr, "channel_id", m.ChannelID)
		s.MessageReactionAdd(m.ChannelID, m.ID, "⚠️")
		if delErr := service.DeleteSenryu(int(created.ID), m.GuildID); delErr != nil {
			logger.Error("Failed to rollback jiyuritsu after reply failure", "error", delErr, "senryu_id", created.ID)
		}
	}

	// Track detected user for post-detection abuse check
	recordDetectedUser(m.ChannelID, m.Author.ID)

	// Auto-cooldown
	cooldown := config.GetConf().Detection.AutoCooldownSeconds
	if cooldown > 0 {
		service.SetTimeout(m.ChannelID, m.Author.ID, time.Duration(cooldown)*time.Second)
	}
}

// handleGoGenRisshiDetected handles a detected 五言律詩.
func handleGoGenRisshiDetected(s *discordgo.Session, m *discordgo.MessageCreate, phrases []string, spoiler bool) {
	// Store first 3 phrases in Kamigo/Nakasichi/Simogo, rest in Shiku/Goku (concatenated)
	rec := model.Senryu{
		ServerID:  m.GuildID,
		AuthorID:  m.Author.ID,
		Kamigo:    phrases[0] + " " + phrases[1],
		Nakasichi: phrases[2] + " " + phrases[3],
		Simogo:    phrases[4] + " " + phrases[5],
		Shiku:     phrases[6],
		Goku:      phrases[7],
		Type:      model.PoemTypeGoGenRisshi,
		Spoiler:   &spoiler,
	}

	created, err := service.CreateSenryu(rec)
	if err != nil {
		logger.Error("Failed to create gogenrisshi record", "error", err)
		metrics.RecordError("database")
		s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
		return
	}

	combo := service.RecordCombo(m.GuildID, m.Author.ID)
	comboText := service.GetComboText(combo)

	fullText := strings.Join(phrases, "\n")
	replyText := fmt.Sprintf("五言律詩を検出しました！\n```\n%s\n```", fullText)
	if spoiler {
		replyText = fmt.Sprintf("五言律詩を検出しました！\n||```\n%s\n```||", fullText)
	}
	if comboText != "" {
		replyText += "\n" + comboText
	}

	msg := &discordgo.MessageSend{
		Content:   replyText,
		Reference: m.Reference(),
	}

	if _, err := s.ChannelMessageSendComplex(m.ChannelID, msg); err != nil {
		logger.Warn("Failed to send gogenrisshi reply", "error", err, "channel_id", m.ChannelID)
		s.MessageReactionAdd(m.ChannelID, m.ID, "⚠️")
		if delErr := service.DeleteSenryu(int(created.ID), m.GuildID); delErr != nil {
			logger.Error("Failed to rollback gogenrisshi after reply failure", "error", delErr, "senryu_id", created.ID)
		}
	}

	// Track detected user for post-detection abuse check
	recordDetectedUser(m.ChannelID, m.Author.ID)

	// Auto-cooldown
	cooldown := config.GetConf().Detection.AutoCooldownSeconds
	if cooldown > 0 {
		service.SetTimeout(m.ChannelID, m.Author.ID, time.Duration(cooldown)*time.Second)
	}
}

// getDisplayName returns the best display name for a user in a guild.
// Priority: server nickname > global display name > username
func getDisplayName(s *discordgo.Session, guildID string, user *discordgo.User) string {
	member, err := s.GuildMember(guildID, user.ID)
	if err == nil && member.Nick != "" {
		return member.Nick
	}
	if user.GlobalName != "" {
		return user.GlobalName
	}
	return user.Username
}

func getWriters(senryus []model.Senryu, guildID string, session *discordgo.Session) []string {
	var writers []string
	for _, senryu := range senryus {
		member, err := session.GuildMember(guildID, senryu.AuthorID)
		if err != nil {
			continue
		}
		if member.Nick != "" {
			writers = append(writers, member.Nick)
		} else {
			writers = append(writers, member.User.Username)
		}
	}
	return sliceUnique(writers)
}
