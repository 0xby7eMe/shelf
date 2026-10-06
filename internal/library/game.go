package library

type Source string

const (
	SourceSteam Source = "steam"
)

type Game struct {
	ID              string `json:"id"`         
	Source          Source `json:"source"`
	ExternalID      string `json:"externalId"` 
	Name            string `json:"name"`
	Installed       bool   `json:"installed"`
	PlaytimeMinutes int    `json:"playtimeMinutes"`
	LastPlayed      int64  `json:"lastPlayed"` 
	InstallPath     string `json:"installPath,omitempty"`
	Cover           string `json:"cover,omitempty"` 
}