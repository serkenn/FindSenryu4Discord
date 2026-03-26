package commands

import (
	"fmt"
	"math"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/u16-io/FindSenryu4Discord/pkg/metrics"
	"github.com/u16-io/FindSenryu4Discord/service"
)

// HandleInfoCommand handles the /info slash command.
// Shows the invoking user's current status (timeout, detection, channel mute) as an ephemeral message.
func HandleInfoCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	metrics.RecordCommandExecuted("info")

	if i.GuildID == "" {
		respondError(s, i, "このコマンドはサーバー内でのみ使用できます")
		return
	}

	userID := getUserID(i)
	channelID := i.ChannelID

	// Timeout status
	timeoutStatus := "⏱️ タイムアウト: **なし**"
	remaining := service.GetTimeoutRemaining(channelID, userID)
	if remaining > 0 {
		timeoutStatus = fmt.Sprintf("⏱️ タイムアウト: **有効**（残り %s）", formatDuration(remaining))
	}

	// Detection opt-out status
	detectStatus := "🔍 川柳検出: **有効**"
	if service.IsDetectionOptedOut(i.GuildID, userID) {
		detectStatus = "🔍 川柳検出: **無効**（オプトアウト中）"
	}

	// Channel mute status
	channelStatus := "🔇 チャンネルミュート: **なし**"
	if service.IsMute(channelID) {
		channelStatus = "🔇 チャンネルミュート: **有効**（このチャンネルは検出停止中）"
	}

	embed := &discordgo.MessageEmbed{
		Title: "📋 あなたの現在のステータス",
		Color: 0x3498DB,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "タイムアウト",
				Value: timeoutStatus,
			},
			{
				Name:  "検出設定",
				Value: detectStatus,
			},
			{
				Name:  "チャンネル状態",
				Value: channelStatus,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "この情報はあなただけに表示されています",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}

// formatDuration formats a duration into a human-readable Japanese string.
func formatDuration(d time.Duration) string {
	totalSeconds := int(math.Ceil(d.Seconds()))
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d時間%d分", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%d分%d秒", minutes, seconds)
	}
	return fmt.Sprintf("%d秒", seconds)
}
