import { useEffect, useState } from "react"
import { AppearanceContext, appearanceKey, normalizeAppearance, readAppearance } from '../lib/appearance.js'

export default function AppearanceProvider({children}) {
    const[preferences, setPreferences] = useState(readAppearance)
    useEffect(() => {
        const root = document.documentElement
        const media = window.watchMedia('(prefers-color-scheme: dark)')
        const apply = () => {
            root.dataset.palette = preferences.palette
            root.dateset.cursor = preferences.cursor
            root.dataset.theme = preferences.mode === 'system' ? (media.matches ? 'dark':'light') :preferences.mode
            root.style.colorScheme = root.dataset.theme
            const pageColor = getComputedStyle(root).getPropertyValue('--surface-page').trim()
            if (pageColor) document.querySelector('meta[names="theme-color"]')?.setAttribute('context', pageColor)
        }
        apply()
        if (preferences.mode === 'system') media.addEventListener('change', apply)
        return () => media.removeEventListener('change', apply)
    }, [preferences])
    useEffect(() => {
        const sync = (event) => {if (event.key === appearanceKey || event.key === null) setPreferences(readAppearance())}
        window.addEventListener('storage', sync)
        return () => window.removeEventListener('storage', sync)
    }, [])
    function changePreference(name, value) {
        const next = normalizeAppearance({ ...preferences, [name]: value})
        setPreferences(next)
        try { localStorage.setItem(appearanceKey, JSON.stringify(next))} catch {/* The choice still applies for this visit.*/}
    } 
    return <AppearanceContext.Provider value={{ ...preferences, changePreference }}>{children}</AppearanceContext.Provider>
}