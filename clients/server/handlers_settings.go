package server

import (
	"idios/internal/config"
	"idios/internal/reminders"
	"net/http"
	"strings"
	"time"
)

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"config": cfg, "config_path": s.opts.ConfigPath, "config_dir": config.ConfigDir(), "db_path": s.opts.DBPath})
}

func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Config config.AppConfig `json:"config"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Config.Version == 0 {
		input.Config.Version = 1
	}
	if input.Config.Server.Port < 1 || input.Config.Server.Port > 65535 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "server.port must be between 1 and 65535"})
		return
	}
	if input.Config.Server.Host != "127.0.0.1" && input.Config.Server.Host != "localhost" && input.Config.Server.Host != "::1" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "server.host must be a loopback address"})
		return
	}
	if input.Config.Server.RefreshIntervalSeconds < 10 || input.Config.Server.RefreshIntervalSeconds > 3600 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "server.refresh_interval_seconds must be between 10 and 3600"})
		return
	}
	if input.Config.Notifications.PollIntervalMinutes < 1 || input.Config.Notifications.PollIntervalMinutes > 1440 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "notifications.poll_interval_minutes must be between 1 and 1440"})
		return
	}
	for name, value := range map[string]string{"morning_time": input.Config.Notifications.PlanReminders.MorningTime, "midday_time": input.Config.Notifications.PlanReminders.MiddayTime, "evening_time": input.Config.Notifications.PlanReminders.EveningTime} {
		parsed, err := time.Parse("15:04", value)
		if err != nil || parsed.Format("15:04") != value {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "notifications.plan_reminders." + name + " must use 24-hour HH:MM"})
			return
		}
	}
	planTimes := []string{input.Config.Notifications.PlanReminders.MorningTime, input.Config.Notifications.PlanReminders.MiddayTime, input.Config.Notifications.PlanReminders.EveningTime}
	parsedPlanTimes := make([]time.Time, 0, len(planTimes))
	for _, value := range planTimes {
		parsed, _ := time.Parse("15:04", value) // Format validation above has already succeeded.
		parsedPlanTimes = append(parsedPlanTimes, parsed)
	}
	if !parsedPlanTimes[0].Before(parsedPlanTimes[1]) || !parsedPlanTimes[1].Before(parsedPlanTimes[2]) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "notifications.plan_reminders times must be ordered morning < midday < evening"})
		return
	}
	minReminder, maxReminder := reminders.NormalizeRange(input.Config.Notifications.QuoteReminders)
	if input.Config.Notifications.QuoteReminders.Enabled && (input.Config.Notifications.QuoteReminders.MinIntervalMinutes < 1 || input.Config.Notifications.QuoteReminders.MaxIntervalMinutes < input.Config.Notifications.QuoteReminders.MinIntervalMinutes) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "notifications.quote_reminders interval range must be positive and max must be greater than or equal to min"})
		return
	}
	input.Config.Notifications.QuoteReminders.MinIntervalMinutes = minReminder
	input.Config.Notifications.QuoteReminders.MaxIntervalMinutes = maxReminder
	if input.Config.Notifications.QuoteReminders.Enabled && strings.TrimSpace(input.Config.Notifications.QuoteReminders.NextAt) == "" {
		reminders.EnsureSchedule(&input.Config, time.Now())
	}
	if message := validateWebThemeConfig(input.Config.Web); message != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": message})
		return
	}
	s.mu.Lock()
	previous := s.cfg
	previousReminder := previous.Notifications.QuoteReminders
	nextReminder := &input.Config.Notifications.QuoteReminders
	if nextReminder.Enabled && previousReminder.Enabled && nextReminder.MinIntervalMinutes == previousReminder.MinIntervalMinutes && nextReminder.MaxIntervalMinutes == previousReminder.MaxIntervalMinutes {
		nextReminder.NextAt = previousReminder.NextAt
		nextReminder.LastQuoteID = previousReminder.LastQuoteID
	}
	if err := config.SaveAppConfig(input.Config); err != nil {
		s.mu.Unlock()
		writeError(w, err)
		return
	}
	resolved, err := config.LoadAppConfig()
	if err != nil {
		s.mu.Unlock()
		writeError(w, err)
		return
	}
	s.cfg = resolved
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "saved",
		"config_path":      s.opts.ConfigPath,
		"restart_required": resolved.Server.Port != previous.Server.Port || resolved.Server.Host != previous.Server.Host,
	})
}
