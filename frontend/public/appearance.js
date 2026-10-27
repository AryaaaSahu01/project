(() => {
    let preferences = {}
    try {preferences = JSON.parse(localStorage.getItem('chrono.appearance.v1')) || {} } catch {/*Storage can be unavailable*/}
    const mode = ['light', 'dark', 'system'].includes(preferences.mode) ? preferences.mode : 'system'
    const root = document.documentElement
    root.dataset.palette = ['parchment', 'sage', 'slate'].includes(preferences.palette) ? preferences.palette : 'parchment'
    root.dataset.theme = mode === 'system' ? (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark':'light') :mode
    root.dataset.cursor = preferences.cursor === 'system' ? 'system' : 'pen'
    root.style.colorScheme = root.dataset.theme
}) ()