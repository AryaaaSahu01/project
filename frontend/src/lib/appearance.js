import { createContext, useContext } from "react"

export const AppearanceContext = createContext(null)
export const appearanceKey = 'chrono.appearance.v1'
export function normalizeAppearance(input) {
    return {
        mode: ['system', 'light', 'dark'].includes(input?.mode) ? input.mode : 'system',
        palette: ['parchment', 'sage', 'slate' ].includes(input?.palette) ? input.palette : 'parchment',
        cursor: input?.cursor === 'system' ? 'system' : 'pen',
    }
}

export function readAppearance() {
    try { return normalizeAppearance(JSON.parse(localStorage.getItem(appearanceKey))) }
    catch { return normalizeAppearance(null) }
}
export function useAppearance() { return useContext(AppearanceContext) }