package bot

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"TGCGO/config"
	"TGCGO/internal/auth"
	"TGCGO/internal/commands"
	"TGCGO/internal/console"
	"TGCGO/internal/disks"
	"TGCGO/internal/network"
	"TGCGO/internal/processes"
	"TGCGO/internal/services"
	"TGCGO/internal/system"

	tele "gopkg.in/telebot.v3"
)

type Bot struct {
	bot      *tele.Bot
	sessions map[int64]*Session
	sessMu   sync.RWMutex
}

type Session struct {
	Type    string
	Step    int
	Name    string
	Desc    string
	CmdID   string
	Device  string
	Path    string
	Options string
	Data    interface{}
}

func New(token string) (*Bot, error) {
	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, err
	}

	bot := &Bot{bot: b, sessions: make(map[int64]*Session)}

	// Register new command handlers
	b.Handle("/help", func(c tele.Context) error {
		return bot.helpHandler(c)
	})
	// Removed /status command registration as per user request
	// /console now universal; registration kept
	b.Handle("/console", func(c tele.Context) error {
		return bot.consoleHandler(c)
	})
	b.Handle("/convert", func(c tele.Context) error {
		return bot.convertHandler(c)
	})

	b.Handle("/start", func(c tele.Context) error {
		if auth.IsAuthenticated(c.Chat().ID) {
			return bot.mainMenu(c)
		}
		return c.Send(config.T("auth_prompt"))
	})

	b.Handle(tele.OnText, func(c tele.Context) error {
		if !auth.IsAuthenticated(c.Chat().ID) {
			ok, msg := auth.Authenticate(c.Chat().ID, c.Text())
			if ok {
				c.Send(config.T("auth_success"))
				return bot.mainMenu(c)
			}
			if msg != "" {
				return c.Send(msg)
			}
			return nil
		}
		if s, ok := bot.getSession(c.Chat().ID); ok {
			return bot.sessionInput(c, s)
		}
		return nil
	})

	b.Handle(tele.OnCallback, func(c tele.Context) error {
		if !auth.IsAuthenticated(c.Chat().ID) {
			return c.Respond(&tele.CallbackResponse{Text: "🔒 /start", ShowAlert: true})
		}
		return bot.handleCallback(c)
	})

	return bot, nil
}

func (b *Bot) Start() { go b.bot.Start() }
func (b *Bot) Stop()  { b.bot.Stop() }

func (b *Bot) getSession(chatID int64) (*Session, bool) {
	b.sessMu.RLock()
	defer b.sessMu.RUnlock()
	s, ok := b.sessions[chatID]
	return s, ok
}

func (b *Bot) setSession(chatID int64, s *Session) {
	b.sessMu.Lock()
	defer b.sessMu.Unlock()
	b.sessions[chatID] = s
}

func (b *Bot) deleteSession(chatID int64) {
	b.sessMu.Lock()
	defer b.sessMu.Unlock()
	delete(b.sessions, chatID)
}

func (b *Bot) handleCallback(c tele.Context) error {
	data := strings.TrimSpace(c.Data())

	// Security Lock checks
	if (data == "tab_disks" || data == "disk_mount" || data == "disk_umount" || strings.HasPrefix(data, "disk_umount_")) && config.IsCmdDisabled("disks") {
		return c.Respond(&tele.CallbackResponse{Text: config.T("feature_disabled"), ShowAlert: true})
	}
	if (data == "tab_services" || data == "srv_add" || data == "srv_delete_menu" || strings.HasPrefix(data, "srv_delete_") || strings.HasPrefix(data, "srv_")) && config.IsCmdDisabled("services") {
		return c.Respond(&tele.CallbackResponse{Text: config.T("feature_disabled"), ShowAlert: true})
	}
	if (data == "tab_console" || data == "console_input" || data == "console_terminal" || strings.HasPrefix(data, "console_quick_")) && config.IsCmdDisabled("console") {
		return c.Respond(&tele.CallbackResponse{Text: config.T("feature_disabled"), ShowAlert: true})
	}
	if (data == "tab_power" || data == "power_reboot" || data == "power_reboot_confirm" || data == "power_shutdown" || data == "power_shutdown_confirm") && config.IsCmdDisabled("power") {
		return c.Respond(&tele.CallbackResponse{Text: config.T("feature_disabled"), ShowAlert: true})
	}
	if (data == "tab_commands" || data == "cmd_create" || data == "cmd_list" || data == "cmd_edit_menu" || strings.HasPrefix(data, "cmd_edit_") || data == "cmd_delete_menu" || strings.HasPrefix(data, "cmd_exec_") || strings.HasPrefix(data, "cmd_delete_")) && config.IsCmdDisabled("commands") {
		return c.Respond(&tele.CallbackResponse{Text: config.T("feature_disabled"), ShowAlert: true})
	}

	switch {
	case data == "cancel":
		// Delete any active session and inform the user
		b.deleteSession(c.Chat().ID)
		return c.Send(config.T("cancel"))
	case data == "main_menu":
		return b.mainMenu(c)
	case data == "tab_system":
		return b.systemTab(c)
	case data == "tab_disks":
		return b.disksTab(c)
	case data == "disk_mount":
		b.setSession(c.Chat().ID, &Session{Type: "mount_disk", Step: 1})
		return c.Send(config.T("enter_device"))
	case data == "disk_umount":
		return b.umountMenu(c)
	case strings.HasPrefix(data, "disk_umount_"):
		r := disks.Umount(strings.TrimPrefix(data, "disk_umount_"))
		return c.Send(fmt.Sprintf("```\n%s\n```", r), &tele.SendOptions{ParseMode: ""})
	case data == "tab_network":
		return b.networkTab(c)
	case data == "net_ports":
		return b.portsMenu(c)
	case data == "net_connections":
		return c.EditOrSend(network.GetConnections(), &tele.SendOptions{ReplyMarkup: backRow(), ParseMode: tele.ModeMarkdown})
	case data == "net_listening":
		return c.EditOrSend(network.GetListeningPorts(), &tele.SendOptions{ReplyMarkup: backRow(), ParseMode: tele.ModeMarkdown})
	case data == "net_port_custom":
		b.setSession(c.Chat().ID, &Session{Type: "check_port"})
		return c.Send(config.T("enter_port"))
	case strings.HasPrefix(data, "net_check_"):
		port := strings.TrimPrefix(data, "net_check_")
		result := network.CheckPort(port)
		return c.Send(fmt.Sprintf("%s", result))
	case data == "tab_processes":
		return b.processesTab(c)
	case data == "tab_services":
		return b.servicesTab(c)
	case data == "srv_add":
		b.setSession(c.Chat().ID, &Session{Type: "add_service"})
		return c.Send(config.T("enter_service"))
	case data == "srv_delete_menu":
		return b.deleteServices(c)
	case strings.HasPrefix(data, "srv_delete_"):
		services.Remove(strings.TrimPrefix(data, "srv_delete_"))
		return c.Send(config.T("deleted"))
	case strings.HasPrefix(data, "srv_"):
		p := strings.Split(data, "_")
		return c.Send(services.Manage(strings.Join(p[2:], "_"), p[1]), &tele.SendOptions{ParseMode: ""})
	case data == "tab_console":
		return b.consoleTab(c)
	case data == "console_input":
		b.setSession(c.Chat().ID, &Session{Type: "console"})
		return c.Send(config.T("enter_console"))
	case data == "console_terminal":
		b.setSession(c.Chat().ID, &Session{Type: "terminal"})
		return c.Send("💻 *Terminal opened*\nEnter command (/exit to close):", &tele.SendOptions{ParseMode: tele.ModeMarkdown})
	case strings.HasPrefix(data, "console_quick_"):
		cmd := strings.TrimPrefix(data, "console_quick_")
		result := console.RunBash(cmd)
		return c.Send(fmt.Sprintf("```\n%s\n```", result), &tele.SendOptions{ParseMode: ""})
	case data == "tab_power":
		return b.powerTab(c)
	case data == "power_reboot":
		return b.confirm(c, "power_reboot_confirm", "confirm_reboot")
	case data == "power_reboot_confirm":
		c.Send(config.T("rebooting"))
		system.Run("sudo reboot")
		return nil
	case data == "power_shutdown":
		return b.confirm(c, "power_shutdown_confirm", "confirm_shutdown")
	case data == "power_shutdown_confirm":
		c.Send(config.T("shutting_down"))
		system.Run("sudo poweroff")
		return nil
	case data == "tab_commands":
		return b.commandsTab(c)
	case data == "cmd_create":
		b.setSession(c.Chat().ID, &Session{Type: "create_command", Step: 1})
		return c.Send(config.T("enter_name"))
	case data == "cmd_list":
		return b.cmdList(c)
	case data == "cmd_edit_menu":
		return b.cmdEdit(c)
	case strings.HasPrefix(data, "cmd_edit_"):
		return b.cmdEditStart(c, strings.TrimPrefix(data, "cmd_edit_"))
	case data == "cmd_delete_menu":
		return b.cmdDelete(c)
	case strings.HasPrefix(data, "cmd_exec_"):
		cid := strings.TrimPrefix(data, "cmd_exec_")
		cmds := commands.Load()
		for _, cmd := range cmds {
			if cmd.ID == cid {
				result := console.RunBash(cmd.Bash)
				return c.Send(fmt.Sprintf("✅ *%s*\n```\n%s\n```", cmd.Name, result), &tele.SendOptions{ParseMode: ""})
			}
		}
	case strings.HasPrefix(data, "cmd_delete_"):
		cid := strings.TrimPrefix(data, "cmd_delete_")
		cmds := commands.Load()
		var n []commands.Command
		for _, cmd := range cmds {
			if cmd.ID != cid {
				n = append(n, cmd)
			}
		}
		commands.Save(n)
		return c.Send(config.T("deleted"))
	case data == "tab_settings":
		return b.settingsTab(c)
	case data == "set_lang_en":
		config.UpdateLanguage("en")
		return c.Send("✅ English")
	case data == "set_lang_uk":
		config.UpdateLanguage("uk")
		return c.Send("✅ Українська")
	case data == "set_lang_ru":
		config.UpdateLanguage("ru")
		return c.Send("✅ Русский")
	case data == "set_password_on":
		config.UpdateUsePassword(true)
		return c.Send(config.T("enabled"))
	case data == "set_password_off":
		config.UpdateUsePassword(false)
		return c.Send(config.T("disabled"))
	case data == "set_security":
		if config.Load().UsePassword && config.Load().PasswordHash != "" {
			b.setSession(c.Chat().ID, &Session{Type: "security_auth"})
			return c.Send(config.T("enter_sec_password"))
		}
		return b.securityMenu(c)
	case data == "set_token":
		b.setSession(c.Chat().ID, &Session{Type: "set_token"})
		return c.Send(config.T("enter_new_token"))
	case data == "set_password":
		b.setSession(c.Chat().ID, &Session{Type: "set_password"})
		return c.Send(config.T("enter_new_pass"))
	case data == "set_limits":
		b.setSession(c.Chat().ID, &Session{Type: "set_limits"})
		return c.Send(config.T("enter_limits"))
	case data == "set_allowed_ids":
		return b.userManagerMenu(c)
	case data == "user_add_btn":
		b.setSession(c.Chat().ID, &Session{Type: "user_add"})
		return c.Send(config.T("enter_allowed_id"))
	case strings.HasPrefix(data, "user_remove_"):
		cidStr := strings.TrimPrefix(data, "user_remove_")
		cid, _ := strconv.ParseInt(cidStr, 10, 64)
		config.RemoveAllowedChatID(cid)
		return b.userManagerMenu(c)
	case strings.HasPrefix(data, "sec_toggle_"):
		cmd := strings.TrimPrefix(data, "sec_toggle_")
		config.ToggleCmd(cmd)
		return b.securityMenu(c)
	case data == "auth_logout":
		auth.Logout(c.Chat().ID)
		return c.EditOrSend(config.T("session_ended"))
	case data == "tab_convert":
		return b.convertHandler(c)
	}
	return c.Respond(&tele.CallbackResponse{})
}

func backRow() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	// Back button
	backBtn := m.Data(config.T("back"), "main_menu")
	// Cancel button
	cancelBtn := m.Data("❌ Cancel", "cancel")
	m.Inline(m.Row(backBtn, cancelBtn))
	return m
}

func (b *Bot) sessionInput(c tele.Context, s *Session) error {
	if c.Text() == "/cancel" {
		b.deleteSession(c.Chat().ID)
		return c.Send(config.T("cancel"))
	}

	// Session input security check
	if (s.Type == "mount_disk" && config.IsCmdDisabled("disks")) ||
		((s.Type == "console" || s.Type == "terminal") && config.IsCmdDisabled("console")) ||
		(s.Type == "add_service" && config.IsCmdDisabled("services")) ||
		((s.Type == "create_command" || s.Type == "edit_command") && config.IsCmdDisabled("commands")) {
		b.deleteSession(c.Chat().ID)
		return c.Send(config.T("feature_disabled"))
	}

	switch s.Type {
	case "console":
		b.deleteSession(c.Chat().ID)
		result := console.RunBash(c.Text())
		return c.Send(fmt.Sprintf("```\n%s\n```", result), &tele.SendOptions{ParseMode: ""})
	case "terminal":
		if c.Text() == "/exit" {
			b.deleteSession(c.Chat().ID)
			return c.Send("❌ Terminal closed")
		}
		result := console.RunBash(c.Text())
		return c.Send(fmt.Sprintf("```\n%s\n```", result), &tele.SendOptions{ParseMode: ""})
	case "create_command":
		return b.createCmd(c, s, c.Text())
	case "edit_command":
		return b.editCmd(c, s, c.Text())
	case "mount_disk":
		return b.mountDisk(c, s, c.Text())
	case "check_port":
		b.deleteSession(c.Chat().ID)
		result := network.CheckPort(c.Text())
		return c.Send(result)
	case "add_service":
		services.AddService(c.Text())
		b.deleteSession(c.Chat().ID)
		return c.Send(fmt.Sprintf("%s `%s`", config.T("added"), c.Text()), &tele.SendOptions{ParseMode: ""})
	case "set_token":
		b.deleteSession(c.Chat().ID)
		config.UpdateToken(c.Text())
		c.Send(config.T("token_updated"))
		time.Sleep(1 * time.Second)
		os.Exit(0)
		return nil
	case "set_password":
		b.deleteSession(c.Chat().ID)
		config.UpdatePassword(c.Text())
		return c.Send(config.T("updated"))
	case "set_limits":
		b.deleteSession(c.Chat().ID)
		parts := strings.Fields(c.Text())
		if len(parts) == 2 {
			attempts, _ := strconv.Atoi(parts[0])
			block, _ := strconv.Atoi(parts[1])
			if attempts > 0 && block > 0 {
				config.UpdateAttemptsAndBlock(attempts, block)
				return c.Send(config.T("updated"))
			}
		}
		return c.Send("❌ Invalid format. Use: `<attempts> <block_minutes>`")
	case "user_add":
		b.deleteSession(c.Chat().ID)
		cid, err := strconv.ParseInt(strings.TrimSpace(c.Text()), 10, 64)
		if err == nil && cid != 0 {
			config.AddAllowedChatID(cid)
			return c.Send(config.T("added"))
		}
		return c.Send("❌ Invalid Chat ID")
	case "security_auth":
		if config.VerifyPassword(c.Text()) {
			b.deleteSession(c.Chat().ID)
			return b.securityMenu(c)
		}
		config.RecordFailedAttempt(c.Chat().ID)
		allowed, _ := config.CheckLoginAttempt(c.Chat().ID)
		if !allowed {
			b.deleteSession(c.Chat().ID)
			return c.Send(fmt.Sprintf(config.T("auth_blocked"), config.Load().BlockMinutes))
		}
		return c.Send(config.T("auth_failed"))
	}
	return nil
}

func (b *Bot) helpHandler(c tele.Context) error {
	msg := "*Bot Commands Overview*\n" +
		"`/help` – Show this help message\n" +
		"`/console` – Run any shell command (e.g., `fastfetch`, `ls`, etc.)\n" +
		"`/convert` – Media converter (images, video, audio)\n" +
		"`/start` – Open main menu (requires authentication)\n" +
		"`/settings` – Bot configuration\n"
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (b *Bot) consoleHandler(c tele.Context) error {
	args := strings.TrimSpace(c.Message().Payload)
	if args == "" {
		return c.Send("Usage: `/console <command>`", &tele.SendOptions{ParseMode: tele.ModeMarkdown})
	}
	// Run the provided command safely via console helper
	out := console.RunBash(args)
	return c.Send(fmt.Sprintf("```\n%s\n```", out), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (b *Bot) convertHandler(c tele.Context) error {
	return c.Send("Media conversion feature is under development. Use `/convert image <format>` etc.", &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (b *Bot) createCmd(c tele.Context, s *Session, text string) error {
	switch s.Step {
	case 1:
		s.Name = text
		s.Step = 2
		return c.Send(config.T("enter_desc"))
	case 2:
		if text != "-" {
			s.Desc = text
		}
		s.Step = 3
		return c.Send(config.T("enter_bash"))
	case 3:
		cmds := commands.Load()
		cmds = append(cmds, commands.Command{ID: fmt.Sprintf("%d", time.Now().Unix()), Name: s.Name, Description: s.Desc, Bash: text})
		commands.Save(cmds)
		b.deleteSession(c.Chat().ID)
		return c.Send(fmt.Sprintf("%s `%s`", config.T("created"), s.Name), &tele.SendOptions{ParseMode: ""})
	}
	return nil
}

func (b *Bot) editCmd(c tele.Context, s *Session, text string) error {
	switch s.Step {
	case 1:
		s.Name = text
		s.Step = 2
		return c.Send(config.T("enter_desc"))
	case 2:
		if text != "-" {
			s.Desc = text
		}
		s.Step = 3
		return c.Send(config.T("enter_bash"))
	case 3:
		cmds := commands.Load()
		for i := range cmds {
			if cmds[i].ID == s.CmdID {
				cmds[i].Name = s.Name
				cmds[i].Description = s.Desc
				cmds[i].Bash = text
			}
		}
		commands.Save(cmds)
		b.deleteSession(c.Chat().ID)
		return c.Send(config.T("updated"))
	}
	return nil
}

func (b *Bot) mountDisk(c tele.Context, s *Session, text string) error {
	switch s.Step {
	case 1:
		s.Device = text
		s.Step = 2
		return c.Send(config.T("enter_path"))
	case 2:
		s.Path = text
		s.Step = 3
		return c.Send(config.T("enter_options"))
	case 3:
		if text == "" {
			text = "rw,nofail"
		}
		r := disks.Mount(s.Device, s.Path, text)
		b.deleteSession(c.Chat().ID)
		return c.Send(fmt.Sprintf("```\n%s\n```", r), &tele.SendOptions{ParseMode: ""})
	}
	return nil
}

func (b *Bot) mainMenu(c tele.Context) error {
	m := &tele.ReplyMarkup{}

	// Helper to format button label with lock icon if disabled
	lbl := func(key, cmd string) string {
		name := config.T(key)
		if config.IsCmdDisabled(cmd) {
			return "🔒 " + name
		}
		return name
	}

	m.Inline(
		m.Row(m.Data(config.T("system"), "tab_system"), m.Data(lbl("disks", "disks"), "tab_disks")),
		m.Row(m.Data(config.T("network"), "tab_network"), m.Data(config.T("processes"), "tab_processes")),
		m.Row(m.Data(lbl("services", "services"), "tab_services"), m.Data(lbl("console", "console"), "tab_console")),
		m.Row(m.Data("🔄 Convert", "tab_convert"), m.Data(lbl("commands", "commands"), "tab_commands")),
		m.Row(m.Data(lbl("power", "power"), "tab_power"), m.Data(config.T("settings"), "tab_settings")),
		m.Row(m.Data(config.T("logout"), "auth_logout"), m.Data(config.T("help"), "help_menu")))

	return c.Send(fmt.Sprintf(config.T("main_menu"), config.Load().ServerName), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) systemTab(c tele.Context) error {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data(config.T("refresh"), "tab_system"), m.Data(config.T("back"), "main_menu")))
	return c.EditOrSend(system.GetInfo(), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) disksTab(c tele.Context) error {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(m.Data(config.T("mount"), "disk_mount"), m.Data(config.T("unmount"), "disk_umount")),
		m.Row(m.Data(config.T("refresh"), "tab_disks"), m.Data(config.T("back"), "main_menu")),
	)
	return c.EditOrSend(disks.GetInfo(), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) umountMenu(c tele.Context) error {
	mnt := disks.GetMounted()
	if len(mnt) == 0 {
		return c.Send(config.T("no_mounts"))
	}
	m := &tele.ReplyMarkup{}
	var r []tele.Row
	for _, x := range mnt {
		r = append(r, m.Row(m.Data("❌ "+x.Mount, "disk_umount_"+x.Device)))
	}
	r = append(r, m.Row(m.Data(config.T("back"), "tab_disks")))
	m.Inline(r...)
	return c.Send(config.T("unmount_menu"), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) networkTab(c tele.Context) error {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(m.Data(config.T("ports"), "net_ports"), m.Data(config.T("listening"), "net_listening")),
		m.Row(m.Data(config.T("connections"), "net_connections")),
		m.Row(m.Data(config.T("refresh"), "tab_network"), m.Data(config.T("back"), "main_menu")),
	)
	return c.EditOrSend(network.GetInfo(), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) portsMenu(c tele.Context) error {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(m.Data("22", "net_check_22"), m.Data("80", "net_check_80")),
		m.Row(m.Data("443", "net_check_443"), m.Data("445", "net_check_445")),
		m.Row(m.Data(config.T("custom_port"), "net_port_custom"), m.Data(config.T("back"), "tab_network")),
	)
	return c.EditOrSend(config.T("ports_menu"), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) processesTab(c tele.Context) error {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data(config.T("refresh"), "tab_processes"), m.Data(config.T("back"), "main_menu")))
	return c.EditOrSend(processes.GetTopProcesses(), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) servicesTab(c tele.Context) error {
	m := &tele.ReplyMarkup{}
	var r []tele.Row
	for _, s := range services.Load() {
		r = append(r, m.Row(m.Data(s, "x"), m.Data("▶️", "srv_start_"+s), m.Data("⏹️", "srv_stop_"+s), m.Data("🔄", "srv_restart_"+s)))
	}
	r = append(r, m.Row(m.Data(config.T("add_service_btn"), "srv_add"), m.Data(config.T("del_service_btn"), "srv_delete_menu")))
	r = append(r, m.Row(m.Data(config.T("refresh"), "tab_services"), m.Data(config.T("back"), "main_menu")))
	m.Inline(r...)
	return c.EditOrSend(services.GetInfo(), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) deleteServices(c tele.Context) error {
	svc := services.Load()
	if len(svc) == 0 {
		return c.Send(config.T("no_services"))
	}
	m := &tele.ReplyMarkup{}
	var r []tele.Row
	for _, s := range svc {
		r = append(r, m.Row(m.Data("❌ "+s, "srv_delete_"+s)))
	}
	r = append(r, m.Row(m.Data(config.T("back"), "tab_services")))
	m.Inline(r...)
	return c.Send(config.T("delete_menu"), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) consoleTab(c tele.Context) error {
	q := console.GetQuickCommands()
	m := &tele.ReplyMarkup{}
	var r []tele.Row
	for i := 0; i < len(q); i += 2 {
		if i+1 < len(q) {
			r = append(r, m.Row(m.Data(q[i].Name, "console_quick_"+q[i].Cmd), m.Data(q[i+1].Name, "console_quick_"+q[i+1].Cmd)))
		} else {
			r = append(r, m.Row(m.Data(q[i].Name, "console_quick_"+q[i].Cmd)))
		}
	}
	r = append(r,
		m.Row(m.Data(config.T("enter_input"), "console_input")),
		m.Row(m.Data("💻 Terminal", "console_terminal")),
		m.Row(m.Data(config.T("back"), "main_menu")),
	)
	m.Inline(r...)
	return c.EditOrSend(config.T("console_menu"), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) powerTab(c tele.Context) error {
	m := &tele.ReplyMarkup{}
	m.Inline(
		m.Row(m.Data(config.T("reboot"), "power_reboot")),
		m.Row(m.Data(config.T("shutdown"), "power_shutdown")),
		m.Row(m.Data(config.T("back"), "main_menu")),
	)
	return c.EditOrSend(config.T("power_menu"), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) confirm(c tele.Context, cb, msgKey string) error {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data(config.T("yes"), cb), m.Data(config.T("no"), "tab_power")))
	return c.EditOrSend(config.T(msgKey), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) commandsTab(c tele.Context) error {
	cmds := commands.Load()
	m := &tele.ReplyMarkup{}
	var r []tele.Row
	r = append(r, m.Row(m.Data(config.T("create"), "cmd_create")))
	if len(cmds) > 0 {
		r = append(r, m.Row(m.Data(config.T("execute"), "cmd_list"), m.Data(config.T("edit"), "cmd_edit_menu"), m.Data(config.T("delete"), "cmd_delete_menu")))
	}
	r = append(r, m.Row(m.Data(config.T("back"), "main_menu")))
	m.Inline(r...)
	return c.EditOrSend(fmt.Sprintf("🛠️ *%s* (%d)", config.T("commands"), len(cmds)), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) cmdList(c tele.Context) error {
	cmds := commands.Load()
	if len(cmds) == 0 {
		return c.Send(config.T("no_commands"))
	}
	m := &tele.ReplyMarkup{}
	var r []tele.Row
	for _, c := range cmds {
		r = append(r, m.Row(m.Data(c.Name, "cmd_exec_"+c.ID)))
	}
	r = append(r, m.Row(m.Data(config.T("back"), "tab_commands")))
	m.Inline(r...)
	return c.EditOrSend(config.T("exec_menu"), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) cmdEdit(c tele.Context) error {
	cmds := commands.Load()
	if len(cmds) == 0 {
		return c.Send(config.T("no_commands"))
	}
	m := &tele.ReplyMarkup{}
	var r []tele.Row
	for _, c := range cmds {
		r = append(r, m.Row(m.Data("✏️ "+c.Name, "cmd_edit_"+c.ID)))
	}
	r = append(r, m.Row(m.Data(config.T("back"), "tab_commands")))
	m.Inline(r...)
	return c.EditOrSend(config.T("edit_menu"), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) cmdEditStart(c tele.Context, id string) error {
	cmds := commands.Load()
	for _, cmd := range cmds {
		if cmd.ID == id {
			b.setSession(c.Chat().ID, &Session{Type: "edit_command", CmdID: id, Step: 1})
			return c.Send(fmt.Sprintf("✏️ `%s`\n`%s`\n%s", cmd.Name, cmd.Bash, config.T("enter_name")), &tele.SendOptions{ParseMode: ""})
		}
	}
	return nil
}

func (b *Bot) cmdDelete(c tele.Context) error {
	cmds := commands.Load()
	if len(cmds) == 0 {
		return c.Send(config.T("no_commands"))
	}
	m := &tele.ReplyMarkup{}
	var r []tele.Row
	for _, c := range cmds {
		r = append(r, m.Row(m.Data("❌ "+c.Name, "cmd_delete_"+c.ID)))
	}
	r = append(r, m.Row(m.Data(config.T("back"), "tab_commands")))
	m.Inline(r...)
	return c.EditOrSend(config.T("delete_menu"), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) settingsTab(c tele.Context) error {
	cfg := config.Load()
	m := &tele.ReplyMarkup{}
	var rows []tele.Row

	rows = append(rows, m.Row(
		m.Data("🇬🇧 EN", "set_lang_en"),
		m.Data("🇺🇦 UK", "set_lang_uk"),
		m.Data("🇷🇺 RU", "set_lang_ru"),
	))

	passStatus := config.T("disabled")
	if cfg.UsePassword {
		passStatus = config.T("enabled")
	}

	if cfg.UsePassword {
		rows = append(rows, m.Row(m.Data(config.T("disable_password"), "set_password_off")))
	} else {
		rows = append(rows, m.Row(m.Data(config.T("enable_password"), "set_password_on")))
	}

	rows = append(rows, m.Row(
		m.Data(config.T("set_token"), "set_token"),
		m.Data(config.T("set_allowed_ids"), "set_allowed_ids"),
	))

	rows = append(rows, m.Row(
		m.Data(config.T("set_password"), "set_password"),
		m.Data(config.T("set_limits"), "set_limits"),
	))

	rows = append(rows, m.Row(
		m.Data(config.T("security"), "set_security"),
	))

	rows = append(rows, m.Row(m.Data(config.T("back"), "main_menu")))
	m.Inline(rows...)

	text := fmt.Sprintf(config.T("settings_menu"), cfg.Language, passStatus, cfg.MaxAttempts, cfg.BlockMinutes)
	return c.EditOrSend(text, &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) securityMenu(c tele.Context) error {
	m := &tele.ReplyMarkup{}
	var rows []tele.Row

	getStatusIcon := func(cmd string) string {
		if config.IsCmdDisabled(cmd) {
			return "🔴"
		}
		return "🟢"
	}

	rows = append(rows,
		m.Row(m.Data(fmt.Sprintf("%s Power", getStatusIcon("power")), "sec_toggle_power")),
		m.Row(m.Data(fmt.Sprintf("%s Console", getStatusIcon("console")), "sec_toggle_console")),
		m.Row(m.Data(fmt.Sprintf("%s Services", getStatusIcon("services")), "sec_toggle_services")),
		m.Row(m.Data(fmt.Sprintf("%s Disks", getStatusIcon("disks")), "sec_toggle_disks")),
		m.Row(m.Data(fmt.Sprintf("%s Commands", getStatusIcon("commands")), "sec_toggle_commands")),
		m.Row(m.Data(config.T("back"), "tab_settings")),
	)
	m.Inline(rows...)
	return c.EditOrSend(config.T("security_menu"), &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}

func (b *Bot) userManagerMenu(c tele.Context) error {
	cfg := config.Load()
	m := &tele.ReplyMarkup{}
	var rows []tele.Row

	for _, cid := range cfg.AllowedChatIDs {
		rows = append(rows, m.Row(
			m.Data(fmt.Sprintf("%d", cid), "dummy"),
			m.Data("❌", fmt.Sprintf("user_remove_%d", cid)),
		))
	}
	rows = append(rows, m.Row(m.Data("➕ Add Chat ID", "user_add_btn")))
	rows = append(rows, m.Row(m.Data(config.T("back"), "tab_settings")))
	m.Inline(rows...)

	return c.EditOrSend("👥 *Allowed Users*", &tele.SendOptions{ReplyMarkup: m, ParseMode: tele.ModeMarkdown})
}
