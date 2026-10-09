import { useEffect, useState } from "react"
import { Settings } from "lucide-react"

import { GetGogAccount } from "../../wailsjs/go/main/App"
import { epic } from "../../wailsjs/go/models"

// Shown in place of the library when the GOG filter has nothing to list.
export function GogHint({ onSettings }: { onSettings: () => void }) {
	const [account, setAccount] = useState<epic.GogAccount | null>(null)
	useEffect(() => {
		GetGogAccount().then(setAccount).catch(() => {})
	}, [])
	if (!account) return null

	return (
		<div className="mx-auto max-w-md space-y-4 pt-20 text-center">
			<h3 className="text-base font-semibold tracking-tight">
				{account.loggedIn ? "No GOG games yet" : "Connect your GOG account"}
			</h3>
			<p className="text-sm text-muted-foreground">
				{account.loggedIn
					? "Your GOG library is still loading, or the account has no games. Refresh it from the GOG tab."
					: "Sign in to GOG once and your whole GOG library appears here, ready to install and play through Proton."}
			</p>
			<div className="flex justify-center pt-1">
				<button
					className="inline-flex h-10 items-center justify-center gap-2 rounded-full bg-white px-5 text-xs font-medium text-black transition hover:shadow-[0_0_40px_rgba(255,255,255,0.3)]"
					onClick={onSettings}
				>
					<Settings className="size-3.5" />
					{account.loggedIn ? "Open GOG settings" : "Connect GOG"}
				</button>
			</div>
		</div>
	)
}
