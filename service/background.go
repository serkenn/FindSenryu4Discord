package service

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/u16-io/FindSenryu4Discord/db"
	"github.com/u16-io/FindSenryu4Discord/model"
	"github.com/u16-io/FindSenryu4Discord/pkg/logger"
	"github.com/u16-io/FindSenryu4Discord/pkg/metrics"
)

// UpsertBackground creates or updates a background image record for a guild.
func UpsertBackground(guildID, filePath string) error {
	metrics.RecordDatabaseOperation("upsert_background")

	bg := model.BackgroundImage{
		GuildID:   guildID,
		FilePath:  filePath,
		UpdatedAt: time.Now(),
	}

	// Try to find existing
	var existing model.BackgroundImage
	if err := db.DB.Where("guild_id = ?", guildID).First(&existing).Error; err == nil {
		// Update
		if err := db.DB.Model(&existing).Updates(map[string]interface{}{
			"file_path":  filePath,
			"updated_at": bg.UpdatedAt,
		}).Error; err != nil {
			metrics.RecordError("database")
			logger.Error("Failed to update background", "error", err, "guild_id", guildID)
			return errors.Wrap(err, "failed to update background")
		}
		return nil
	}

	// Create
	if err := db.DB.Create(&bg).Error; err != nil {
		metrics.RecordError("database")
		logger.Error("Failed to create background", "error", err, "guild_id", guildID)
		return errors.Wrap(err, "failed to create background")
	}

	return nil
}

// GetBackground returns the background image record for a guild.
func GetBackground(guildID string) (*model.BackgroundImage, error) {
	metrics.RecordDatabaseOperation("get_background")

	var bg model.BackgroundImage
	if err := db.DB.Where("guild_id = ?", guildID).First(&bg).Error; err != nil {
		return nil, err
	}
	return &bg, nil
}

// GetSenryuList returns paginated senryus for a server.
func GetSenryuList(serverID string, page, pageSize int) ([]model.Senryu, int64, error) {
	metrics.RecordDatabaseOperation("get_senryu_list")

	var total int64
	query := db.DB.Model(&model.Senryu{})
	if serverID != "" {
		query = query.Where("server_id = ?", serverID)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "failed to count senryus")
	}

	var senryus []model.Senryu
	offset := (page - 1) * pageSize
	q := db.DB.Order("id DESC").Offset(offset).Limit(pageSize)
	if serverID != "" {
		q = q.Where("server_id = ?", serverID)
	}
	if err := q.Find(&senryus).Error; err != nil {
		return nil, 0, errors.Wrap(err, "failed to get senryus")
	}

	return senryus, total, nil
}

// GetSenryuByIDGlobal returns a senryu by ID without server restriction (for web API).
func GetSenryuByIDGlobal(id int) (*model.Senryu, error) {
	metrics.RecordDatabaseOperation("get_senryu_by_id_global")

	var s model.Senryu
	if err := db.DB.Where("id = ?", id).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}
