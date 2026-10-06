package library

import (
	"log"
	"sort"
	"strings"
	"sync"
)

type Provider interface {
	Source() Source
	Scan() ([]Game, error)
}

type Library struct {
	providers []Provider
}

func New(providers ...Provider) *Library {
	return &Library{providers: providers}
}

func (l *Library) Scan() []Game {
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		all = []Game{}
	)

	for _, p := range l.providers {
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()
			games, err := p.Scan()
			if err != nil {
				log.Printf("library: %s scan failed: %v", p.Source(), err)
				return
			}
			mu.Lock()
			all = append(all, games...)
			mu.Unlock()
		}(p)
	}
	wg.Wait()

	sort.Slice(all, func(i, j int) bool {
		return strings.ToLower(all[i].Name) < strings.ToLower(all[j].Name)
	})
	return all
}