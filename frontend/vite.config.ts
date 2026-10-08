import fs from "fs"
import path from "path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

// Vite empties dist before a build. main.go embeds the folder, so it has to
// stay non-empty in a fresh checkout for `go test` to compile: put the
// tracked .gitkeep back once the build is done.
const keepDist = {
	name: "keep-dist-gitkeep",
	closeBundle() {
		fs.writeFileSync(path.resolve(__dirname, "dist/.gitkeep"), "")
	},
}

export default defineConfig({
	plugins: [react(), tailwindcss(), keepDist],
	resolve: {
		alias: {
			"@": path.resolve(__dirname, "./src"),
		},
	},
})