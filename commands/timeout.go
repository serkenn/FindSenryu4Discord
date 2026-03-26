package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/u16-io/FindSenryu4Discord/pkg/logger"
	"github.com/u16-io/FindSenryu4Discord/pkg/metrics"
	"github.com/u16-io/FindSenryu4Discord/pkg/permissions"
	"github.com/u16-io/FindSenryu4Discord/service"
)

// canUseTimeout checks if the user has permission to use /timeout.
// Allowed: administrators, bot owners, or users with a timeout-allowed role.
func canUseTimeout(i *discordgo.InteractionCreate) bool {
	userID := getUserID(i)

	// Bot owners always allowed
	if permissions.IsOwner(userID) {
		return true
	}

	if i.Member != nil {
		// Check Discord administrator permission
		if i.Member.Permissions&discordgo.PermissionAdministrator != 0 {
			return true
		}
		// Check timeout-allowed roles
		if service.HasTimeoutRole(i.GuildID, i.Member.Roles) {
			return true
		}
	}

	return false
}

// HandleTimeoutCommand handles the /timeout slash command.
// Temporarily pauses senryu detection for the calling user in the current channel.
// Only administrators, bot owners, or users with a timeout-allowed role can use this.
func HandleTimeoutCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	metrics.RecordCommandExecuted("timeout")

	if i.GuildID == "" {
		respondError(s, i, "このコマンドはサーバー内でのみ使用できます")
		return
	}

	if !canUseTimeout(i) {
		respondError(s, i, "このコマンドは管理者または許可されたロールのみ使用できます")
		return
	}

	userID := getUserID(i)
	channelID := i.ChannelID
	options := i.ApplicationCommandData().Options

	// No argument → show status
	if len(options) == 0 {
		remaining := service.GetTimeoutRemaining(channelID, userID)
		if remaining > 0 {
			respondEphemeral(s, i, fmt.Sprintf("このチャンネルでタイムアウト中です（残り %s）", formatDuration(remaining)))
		} else {
			respondEphemeral(s, i, "このチャンネルではタイムアウトは設定されていません")
		}
		return
	}

	minutes := int(options[0].IntValue())
	if minutes < 1 || minutes > 1440 {
		respondEphemeral(s, i, "1〜1440分（24時間）の範囲で指定してください")
		return
	}

	duration := time.Duration(minutes) * time.Minute
	service.SetTimeout(channelID, userID, duration)

	logger.Info("User set channel timeout", "user_id", userID, "channel_id", channelID, "minutes", minutes)
	respondEphemeral(s, i, fmt.Sprintf("このチャンネルでの川柳検出を %s 一時停止しました ⏸️", formatDuration(duration)))
}

func formatDuration(d time.Duration) string {
	if d >= time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		if mins > 0 {
			return fmt.Sprintf("%d時間%d分", hours, mins)
		}
		return fmt.Sprintf("%d時間", hours)
	}
	return fmt.Sprintf("%d分", int(d.Minutes()))
}

// HandleTimeoutRoleCommand handles the /timeout-role slash command for managing timeout permissions.
func HandleTimeoutRoleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	metrics.RecordCommandExecuted("timeout-role")

	if i.GuildID == "" {
		respondError(s, i, "このコマンドはサーバー内でのみ使用できます")
		return
	}

	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		return
	}

	switch options[0].Name {
	case "add":
		handleTimeoutAddRole(s, i, options[0].Options)
	case "remove":
		handleTimeoutRemoveRole(s, i, options[0].Options)
	case "list":
		handleTimeoutListRoles(s, i)
	}
}

func handleTimeoutAddRole(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	if len(opts) == 0 {
		respondError(s, i, "ロールを指定してください")
		return
	}
	role := opts[0].RoleValue(s, i.GuildID)
	if role == nil {
		respondError(s, i, "無効なロールです")
		return
	}
	if err := service.AddTimeoutRole(i.GuildID, role.ID); err != nil {
		respondError(s, i, "ロールの追加に失敗しました")
		return
	}
	logger.Info("Timeout role added", "guild_id", i.GuildID, "role_id", role.ID, "role_name", role.Name)
	respondEphemeral(s, i, fmt.Sprintf("ロール **%s** にtimeout権限を付与しました ✅", role.Name))
}

func handleTimeoutRemoveRole(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	if len(opts) == 0 {
		respondError(s, i, "ロールを指定してください")
		return
	}
	role := opts[0].RoleValue(s, i.GuildID)
	if role == nil {
		respondError(s, i, "無効なロールです")
		return
	}
	if err := service.RemoveTimeoutRole(i.GuildID, role.ID); err != nil {
		respondError(s, i, "ロールの削除に失敗しました")
		return
	}
	logger.Info("Timeout role removed", "guild_id", i.GuildID, "role_id", role.ID, "role_name", role.Name)
	respondEphemeral(s, i, fmt.Sprintf("ロール **%s** のtimeout権限を解除しました ✅", role.Name))
}

func handleTimeoutListRoles(s *discordgo.Session, i *discordgo.InteractionCreate) {
	roles, err := service.GetTimeoutRoles(i.GuildID)
	if err != nil || len(roles) == 0 {
		respondEphemeral(s, i, "timeout権限が付与されたロールはありません（管理者は常に使用可能）")
		return
	}
	var mentions []string
	for _, r := range roles {
		mentions = append(mentions, fmt.Sprintf("<@&%s>", r))
	}
	respondEphemeral(s, i, fmt.Sprintf("timeout権限ロール: %s", strings.Join(mentions, ", ")))
}
