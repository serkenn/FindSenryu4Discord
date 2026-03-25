package commands

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/u16-io/FindSenryu4Discord/pkg/logger"
	"github.com/u16-io/FindSenryu4Discord/pkg/metrics"
	"github.com/u16-io/FindSenryu4Discord/service"
)

// HandleTimeoutCommand handles the /timeout slash command.
// Temporarily pauses senryu detection for the calling user in the current channel.
func HandleTimeoutCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	metrics.RecordCommandExecuted("timeout")

	if i.GuildID == "" {
		respondError(s, i, "このコマンドはサーバー内でのみ使用できます")
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
