import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import { startAppearance } from '@/lib/appearance'

// Before the first paint, so the interface never flashes the wrong colour or size.
startAppearance()

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
		<React.StrictMode>
				<App/>
		</React.StrictMode>
)
