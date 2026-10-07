package library

type Source string

const (
	SourceSteam Source = "steam"
	SourceEpic  Source = "epic"
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
	SizeBytes       int64  `json:"sizeBytes,omitempty"` // space the install takes on disk
	ThirdParty      string `json:"thirdParty,omitempty"` // store that must install the game, e.g. "Ubisoft Connect"
	Version         string `json:"version,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable,omitempty"`
	CloudSaves      bool   `json:"cloudSaves,omitempty"` // the game supports cloud saves
	Hero            string `json:"hero,omitempty"`  // wide banner; empty means derive it from Cover
}