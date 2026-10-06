import { useEffect, useState } from "react"
import { Moon, Palette, Server, Sun, Zap } from "lucide-react"

import { Greet } from "../wailsjs/go/main/App"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"

const features = [
    {
        icon: Server,
        title: "Go backend",
        description: "Call Go functions straight from the frontend via generated bindings.",
    },
    {
        icon: Zap,
        title: "React + Vite",
        description: "Instant hot reload with TypeScript and Tailwind CSS v4.",
    },
    {
        icon: Palette,
        title: "shadcn/ui",
        description: "Accessible, themeable components you own and can edit.",
    },
]

function App() {
    const [dark, setDark] = useState(
        () => window.matchMedia("(prefers-color-scheme: dark)").matches
    )
    const [name, setName] = useState("")
    const [result, setResult] = useState("")

    useEffect(() => {
        document.documentElement.classList.toggle("dark", dark)
    }, [dark])

    async function greet() {
        setResult(await Greet(name || "stranger"))
    }

    return (
        <div className="min-h-screen bg-background text-foreground">
            <div className="mx-auto flex min-h-screen max-w-3xl flex-col px-6 py-8">
                <header className="flex items-center justify-between">
                    <Badge variant="secondary">Wails + shadcn</Badge>
                    <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => setDark((d) => !d)}
                        aria-label="Toggle theme"
                    >
                        {dark ? <Sun className="size-4" /> : <Moon className="size-4" />}
                    </Button>
                </header>

                <main className="flex flex-1 flex-col justify-center gap-10 py-10">
                    <div className="space-y-3 text-center">
                        <h1 className="text-4xl font-bold tracking-tight sm:text-5xl">
                            Build desktop apps faster
                        </h1>
                        <p className="mx-auto max-w-md text-muted-foreground">
                            A clean starting point for Go desktop apps with a modern React
                            frontend. Edit{" "}
                            <code className="rounded bg-muted px-1.5 py-0.5 text-sm">
                                frontend/src/App.tsx
                            </code>{" "}
                            to get going.
                        </p>
                    </div>

                    <div className="grid gap-4 sm:grid-cols-3">
                        {features.map(({ icon: Icon, title, description }) => (
                        <Card key={title}>
                            <CardHeader>
                                <Icon className="mb-2 size-5 text-muted-foreground" />
                                <CardTitle className="text-base">{title}</CardTitle>
                                <CardDescription>{description}</CardDescription>
                            </CardHeader>
                        </Card>
                        ))}
                    </div>

                    <Card>
                        <CardHeader>
                            <CardTitle className="text-base">Try a Go binding</CardTitle>
                            <CardDescription>
                                This calls <code>Greet()</code> in <code>app.go</code>.
                            </CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-3">
                            <div className="flex gap-2">
                                <Input
                                placeholder="Your name"
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                onKeyDown={(e) => e.key === "Enter" && greet()}
                                />
                                <Button onClick={greet}>Greet</Button>
                            </div>
                            {result && (
                                <p className="rounded-md bg-muted px-3 py-2 text-sm">{result}</p>
                            )}
                        </CardContent>
                    </Card>
                </main>
            </div>
        </div>
    )
}

export default App