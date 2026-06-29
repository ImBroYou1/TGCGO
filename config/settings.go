package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Settings struct {
	Token          string   `json:"token"`
	AllowedChatIDs []int64  `json:"allowed_chat_ids"`
	PasswordHash   string   `json:"password_hash"`
	UsePassword    bool     `json:"use_password"`
	Language       string   `json:"language"`
	MaxAttempts    int      `json:"max_attempts"`
	BlockMinutes   int      `json:"block_minutes"`
	ServerName     string   `json:"server_name"`
	DisabledCmds   []string `json:"disabled_cmds"`
	ConsoleEnabled bool     `json:"console_enabled"`
	ConsoleBlacklist []string `json:"console_blacklist"`
}

type LoginAttempt struct {
	mu           sync.Mutex
	Count        int
	BlockedUntil time.Time
}

var (
	cfg           Settings
	mu            sync.RWMutex
	filePath      = "data/settings.json"
	loginAttempts sync.Map
)

func Load() Settings {
	mu.RLock()
	defer mu.RUnlock()
	return cfg
}

func HashPassword(pass string) string {
	if pass == "" || pass == "none" || pass == "None" {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(pass))
	return hex.EncodeToString(h.Sum(nil))
}

func Init() {
	mu.Lock()
	defer mu.Unlock()

	// Default settings
	cfg.Language = "en"
	cfg.MaxAttempts = 3
	cfg.BlockMinutes = 5
	cfg.ServerName = "Server"
	cfg.UsePassword = true
	cfg.AllowedChatIDs = []int64{}
	cfg.DisabledCmds = []string{}

	// 1. Try to load from settings.json
	data, err := os.ReadFile(filePath)
	if err == nil {
		var saved Settings
		if json.Unmarshal(data, &saved) == nil {
			cfg = saved
			// Validate defaults
			if cfg.Language == "" { cfg.Language = "en" }
			if cfg.MaxAttempts <= 0 { cfg.MaxAttempts = 3 }
			if cfg.BlockMinutes <= 0 { cfg.BlockMinutes = 5 }
			if cfg.ServerName == "" { cfg.ServerName = "Server" }
			if cfg.AllowedChatIDs == nil { cfg.AllowedChatIDs = []int64{} }
			if cfg.DisabledCmds == nil { cfg.DisabledCmds = []string{} }
			return
		}
	}

	// 2. Fallback to .env if settings.json not found or corrupted
	cfg.Token = os.Getenv("BOT_TOKEN")
	cfg.ServerName = os.Getenv("SERVER_NAME")
	if cfg.ServerName == "" {
		cfg.ServerName = "Server"
	}

	envChatIDStr := os.Getenv("ALLOWED_CHAT_ID")
	if envChatIDStr != "" {
		envChatID, _ := strconv.ParseInt(strings.TrimSpace(envChatIDStr), 10, 64)
		if envChatID != 0 {
			cfg.AllowedChatIDs = append(cfg.AllowedChatIDs, envChatID)
		}
	}

	envPass := os.Getenv("ADMIN_PASSWORD")
	if envPass == "" || envPass == "none" || envPass == "None" {
		cfg.PasswordHash = ""
		cfg.UsePassword = false
	} else {
		cfg.PasswordHash = HashPassword(envPass)
		cfg.UsePassword = true
	}

	// Save the loaded config as settings.json
	saveLocked(cfg)
}

func Save(s Settings) error {
	mu.Lock()
	defer mu.Unlock()
	return saveLocked(s)
}

func saveLocked(s Settings) error {
	cfg = s
	os.MkdirAll("data", 0755)
	data, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(filePath, data, 0644)
}

func UpdateLanguage(lang string) {
	s := Load()
	s.Language = lang
	Save(s)
}

func UpdatePassword(pass string) {
	s := Load()
	if pass == "" || pass == "none" || pass == "None" {
		s.PasswordHash = ""
		s.UsePassword = false
	} else {
		s.PasswordHash = HashPassword(pass)
		s.UsePassword = true
	}
	Save(s)
}

func UpdateUsePassword(val bool) {
	s := Load()
	s.UsePassword = val
	Save(s)
}

func UpdateToken(token string) {
	s := Load()
	s.Token = token
	Save(s)
}

func UpdateAttemptsAndBlock(attempts, blockMinutes int) {
	s := Load()
	if attempts > 0 {
		s.MaxAttempts = attempts
	}
	if blockMinutes > 0 {
		s.BlockMinutes = blockMinutes
	}
	Save(s)
}

// User access control
func IsChatIDAllowed(id int64) bool {
	mu.RLock()
	defer mu.RUnlock()
	for _, cid := range cfg.AllowedChatIDs {
		if cid == id {
			return true
		}
	}
	return false
}

func AddAllowedChatID(id int64) {
	s := Load()
	// Check if already allowed
	for _, cid := range s.AllowedChatIDs {
		if cid == id {
			return
		}
	}
	s.AllowedChatIDs = append(s.AllowedChatIDs, id)
	Save(s)
}

func RemoveAllowedChatID(id int64) {
	s := Load()
	var newList []int64
	for _, cid := range s.AllowedChatIDs {
		if cid != id {
			newList = append(newList, cid)
		}
	}
	s.AllowedChatIDs = newList
	Save(s)
}

func VerifyPassword(plainText string) bool {
    mu.RLock()
    defer mu.RUnlock()
    // If password protection is disabled, any password is accepted.
    if !cfg.UsePassword {
        return true
    }
    // If protection is enabled but no password hash is stored, reject.
    if cfg.PasswordHash == "" {
        return false
    }
    return HashPassword(plainText) == cfg.PasswordHash
}

// Security features toggles
func IsCmdDisabled(cmd string) bool {
	mu.RLock()
	defer mu.RUnlock()
	for _, c := range cfg.DisabledCmds {
		if c == cmd {
			return true
		}
	}
	return false
}

func ToggleCmd(cmd string) {
	s := Load()
	found := false
	var newList []string
	for _, c := range s.DisabledCmds {
		if c == cmd {
			found = true
		} else {
			newList = append(newList, c)
		}
	}
	if !found {
		newList = append(newList, cmd)
	}
	s.DisabledCmds = newList
	Save(s)
}

func CheckLoginAttempt(chatID int64) (bool, string) {
	val, _ := loginAttempts.LoadOrStore(chatID, &LoginAttempt{})
	attempt := val.(*LoginAttempt)
	attempt.mu.Lock()
	defer attempt.mu.Unlock()
	c := Load()
	if time.Now().Before(attempt.BlockedUntil) {
		return false, fmt.Sprintf("🚫 %.0f min", attempt.BlockedUntil.Sub(time.Now()).Minutes())
	}
	if attempt.Count >= c.MaxAttempts {
		attempt.BlockedUntil = time.Now().Add(time.Duration(c.BlockMinutes) * time.Minute)
		attempt.Count = 0
		return false, fmt.Sprintf("🚫 %d min", c.BlockMinutes)
	}
	return true, ""
}

func RecordFailedAttempt(chatID int64) {
	val, _ := loginAttempts.LoadOrStore(chatID, &LoginAttempt{})
	attempt := val.(*LoginAttempt)
	attempt.mu.Lock()
	attempt.Count++
	attempt.mu.Unlock()
}

func ResetAttempts(chatID int64) {
	loginAttempts.Delete(chatID)
}

func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}
