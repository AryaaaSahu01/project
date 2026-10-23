export function downloadText(filename, contents) {
    let url
    let link

    try {
        url = URL.createObjectURL(
            new Blob([contents], {type:'text/plain;charset=utf-8'}),
        )

        link = document.createElement('a')
        link.href = url
        link.download = filename

        document.body.appendChild(link)
        link.click()

        return true
    } catch {
        return false
    } finally {
        link?.remove()

        if (url) {
            window.setTimeout(() => URL.revokeObjectURL(url), 1000)
        }
    }
}