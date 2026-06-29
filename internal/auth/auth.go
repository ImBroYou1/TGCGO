package auth

import (
	"fmt"
	"sync"

	"TGCGO/config"
)

var authenticated sync.Map

func IsAuthenticated(chatID int64) bool {
	c := config.Load()
	if !c.UsePassword {
		return config.IsChatIDAllowed(chatID)
	}
	if !config.IsChatIDAllowed(chatID) {
		return false
	}
	val, ok := authenticated.Load(chatID)
	return ok && val.(bool)
}

func Authenticate(chatID int64, password string) (bool, string) {
	allowed, _ := config.CheckLoginAttempt(chatID)
	if !allowed {
		return false, fmt.Sprintf(config.T("auth_blocked"), config.Load().BlockMinutes)
	}

	cfg := config.Load()
	if cfg.UsePassword && password == "" {
		config.RecordFailedAttempt(chatID)
		return false, config.T("auth_failed")
	}
	if config.VerifyPassword(password) {
		authenticated.Store(chatID, true)
		config.AddAllowedChatID(chatID)
		config.ResetAttempts(chatID)
		return true, ""
	}

	config.RecordFailedAttempt(chatID)
	return false, config.T("auth_failed")
}

func Logout(chatID int64) {
	authenticated.Delete(chatID)
}
