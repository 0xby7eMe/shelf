#!/usr/bin/env node
import fs from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"

const [name, modulePath] = process.argv.slice(2)

if (!name || !/^[a-z0-9]+(-[a-z0-9]+)*$/.test(name)) {
    console.error(
        "Usage: node scripts/rename.mjs <app-name> [go-module-path]\n" +
        "  app-name     lowercase kebab-case, e.g. my-app\n" +
        "  module-path  e.g. github.com/you/my-app (defaults to app-name)"
    )
    process.exit(1)
}

const goModule = modulePath ?? name
const title = name
    .split("-")
    .map((w) => w[0].toUpperCase() + w.slice(1))
    .join(" ")
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..")

function edit(file, transform) {
    const p = path.join(root, file)
    if (!fs.existsSync(p)) {
        console.warn(`  skipped    ${file} (not found)`)
        return
    }
    const before = fs.readFileSync(p, "utf8")
    const after = transform(before)
    if (before === after) {
        console.warn(`  unchanged  ${file}`)
        return
    }
    fs.writeFileSync(p, after)
    console.log(`  updated    ${file}`)
}

const json = (fn) => (text) => {
    const data = JSON.parse(text)
    fn(data)
    return JSON.stringify(data, null, 2) + "\n"
}

console.log(`Renaming to "${name}" (title: "${title}", module: "${goModule}")`)

edit("go.mod", (t) => t.replace(/^module .*/m, `module ${goModule}`))

edit(
    "wails.json",
    json((d) => {
        d.name = name
        d.outputfilename = name
    })
)

edit(
    "frontend/package.json",
    json((d) => {
        d.name = name
    })
)

edit(
    "frontend/package-lock.json",
    json((d) => {
        d.name = name
        if (d.packages?.[""]) d.packages[""].name = name
    })
)

edit("main.go", (t) => t.replace(/(Title:\s+)"[^"]*"/, `$1"${title}"`))

edit("frontend/index.html", (t) =>
    t.replace(/<title>.*<\/title>/, `<title>${title}</title>`)
)

edit("README.md", (t) => t.replace(/^# .*/m, `# ${title}`))

console.log("\nDone. Next:\n  go mod tidy\n  make dev")