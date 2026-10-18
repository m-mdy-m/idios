package tui

import (
	_ "embed"
	"encoding/json"
)

// The in-app prose is packaged with the application rather than placed in the user's settings.
// This keeps end-user configuration focused on preferences while allowing maintainers to edit
// product language and help in one reviewable content file.
//
//go:embed content/en.json
var englishContent []byte

type uiContent struct {
	Overrides        map[string]string `json:"overrides"`
	IntroTitle       string            `json:"intro_title"`
	IntroSubtitle    string            `json:"intro_subtitle"`
	IntroPrivacy     string            `json:"intro_privacy"`
	IntroContinue    string            `json:"intro_continue"`
	IntroSeed        string            `json:"intro_seed"`
	IntroHelp        string            `json:"intro_help"`
	IntroSettings    string            `json:"intro_settings"`
	HelpTitle        string            `json:"help_title"`
	HelpIntro        string            `json:"help_intro"`
	HelpRepository   string            `json:"help_repository"`
	HelpIssue        string            `json:"help_issue"`
	HelpDependency   string            `json:"help_dependency"`
	HelpProject      string            `json:"help_project"`
	HelpConcept      string            `json:"help_concept"`
	HelpPR           string            `json:"help_pr"`
	HelpGraph        string            `json:"help_graph"`
	HelpNotes        string            `json:"help_notes"`
	HelpSeed         string            `json:"help_seed"`
	HelpShortcuts    string            `json:"help_shortcuts"`
	SettingsFontNote string            `json:"settings_font_note"`
	NotesEmpty       string            `json:"notes_empty"`
	JobsEmpty        string            `json:"jobs_empty"`
}

func (c uiContent) text(value string) string {
	if translated, ok := c.Overrides[value]; ok {
		return translated
	}
	return value
}

var copyText = func() uiContent {
	var copy uiContent
	if err := json.Unmarshal(englishContent, &copy); err != nil {
		panic("invalid embedded IDIOS content: " + err.Error())
	}
	return copy
}()
