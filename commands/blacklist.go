package commands

import (
	"github.com/bwmarrin/discordgo"
	"github.com/u16-io/FindSenryu4Discord/pkg/logger"
	"github.com/u16-io/FindSenryu4Discord/pkg/metrics"
	"github.com/u16-io/FindSenryu4Discord/service"
)

// HandleBlacklistCommand handles the /blacklist slash command.
// Toggles the user's detection opt-out state.
func HandleBlacklistCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	metrics.RecordCommandExecuted("blacklist")

	if i.GuildID == "" {
		respondError(s, i, "このコマンドはサーバー内でのみ使用できます")
		return
	}

	userID := getUserID(i)

	if service.IsDetectionOptedOut(i.GuildID, userID) {
		// Currently opted out → opt back in
		if err := service.OptInDetection(i.GuildID, userID); err != nil {
			logger.Error("Failed to opt in detection via blacklist", "error", err, "user_id", userID, "guild_id", i.GuildID)
			respondEphemeral(s, i, "ブラックリスト解除に失敗しました")
			return
		}
		respondEphemeral(s, i, "ブラックリストを解除しました。川柳検出が **有効** になりました ✅")
	} else {
		// Currently opted in → opt out
		if err := service.OptOutDetection(i.GuildID, userID); err != nil {
			logger.Error("Failed to opt out detection via blacklist", "error", err, "user_id", userID, "guild_id", i.GuildID)
			respondEphemeral(s, i, "ブラックリスト登録に失敗しました")
			return
		}
		respondEphemeral(s, i, "ブラックリストに登録しました。川柳検出が **無効** になりました 🚫")
	}
}
