package service

import (
	"github.com/u16-io/FindSenryu4Discord/db"
	"github.com/u16-io/FindSenryu4Discord/model"
	"github.com/u16-io/FindSenryu4Discord/pkg/logger"
	"github.com/u16-io/FindSenryu4Discord/pkg/metrics"
)

// AddTimeoutRole adds a role that is allowed to use /timeout in a guild.
func AddTimeoutRole(guildID, roleID string) error {
	metrics.RecordDatabaseOperation("add_timeout_role")
	setting := model.TimeoutRoleSetting{GuildID: guildID, RoleID: roleID}
	if err := db.DB.Where(setting).FirstOrCreate(&setting).Error; err != nil {
		logger.Error("Failed to add timeout role", "error", err, "guild_id", guildID, "role_id", roleID)
		return err
	}
	return nil
}

// RemoveTimeoutRole removes a role from the timeout-allowed list.
func RemoveTimeoutRole(guildID, roleID string) error {
	metrics.RecordDatabaseOperation("remove_timeout_role")
	if err := db.DB.Where("guild_id = ? AND role_id = ?", guildID, roleID).Delete(&model.TimeoutRoleSetting{}).Error; err != nil {
		logger.Error("Failed to remove timeout role", "error", err, "guild_id", guildID, "role_id", roleID)
		return err
	}
	return nil
}

// GetTimeoutRoles returns all role IDs allowed to use /timeout in a guild.
func GetTimeoutRoles(guildID string) ([]string, error) {
	metrics.RecordDatabaseOperation("get_timeout_roles")
	var settings []model.TimeoutRoleSetting
	if err := db.DB.Where("guild_id = ?", guildID).Find(&settings).Error; err != nil {
		return nil, err
	}
	roles := make([]string, len(settings))
	for i, s := range settings {
		roles[i] = s.RoleID
	}
	return roles, nil
}

// HasTimeoutRole checks if a user has any of the timeout-allowed roles.
func HasTimeoutRole(guildID string, memberRoleIDs []string) bool {
	allowedRoles, err := GetTimeoutRoles(guildID)
	if err != nil || len(allowedRoles) == 0 {
		return false
	}
	allowed := make(map[string]bool, len(allowedRoles))
	for _, r := range allowedRoles {
		allowed[r] = true
	}
	for _, r := range memberRoleIDs {
		if allowed[r] {
			return true
		}
	}
	return false
}
