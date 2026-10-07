import { library } from "../../wailsjs/go/models"

// Steam banners come from the local cover server; other stores supply a URL.
export function heroSrc(game: library.Game): string {
	return game.hero || `${game.cover}?kind=hero`
}
