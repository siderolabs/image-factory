
/**
 * Copy text from the closest span element to the clipboard.
 * Uses the modern Clipboard API (navigator.clipboard).
 * @param {HTMLElement} btn - The button element that triggered the copy action
 * @returns {Promise<void>}
 */
copyToClipboard = async function (btn) {
    try {
        const text = btn.closest('div').querySelector('span').innerText.trim();

        if (navigator.clipboard?.writeText) {
            await navigator.clipboard.writeText(text);
        } else {
            // Fallback for testing, e.g. localhost http origin.
            const textarea = document.createElement('textarea');
            textarea.value = text; document.body.appendChild(textarea);
            textarea.select();
            document.execCommand('copy');
            document.body.removeChild(textarea);
        }

        btn.setAttribute('data-copied', 'true');
        setTimeout(() => btn.removeAttribute('data-copied'), 2000);
    } catch (e) {
        console.error('Failed to copy:', e);
    }
}
