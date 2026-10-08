package main

import (
	"fmt"
	"time"

	"shelf/internal/library"
)

func main() {
	games := library.New(library.NewSteam(nil)).Scan()
	for _, g := range games {
		last := "never"
		if g.LastPlayed > 0 {
			last = time.Unix(g.LastPlayed, 0).Format("2006-01-02")
		}
		fmt.Printf("%-45s %7.1fh  %-10s installed=%v\n",
			g.Name, float64(g.PlaytimeMinutes)/60, last, g.Installed)
	}
	fmt.Printf("\n%d games\n", len(games))
}
