/**
 * Resolve the stored theme choice (light, dark or system) onto <html>.
 * Loaded in <head> so the theme is set before first paint.
 */
(function () {
    const root = document.documentElement;
    const media = matchMedia('(prefers-color-scheme: dark)');

    const apply = function () {
        let choice = 'system';

        try {
            choice = localStorage.getItem('theme');
        } catch (e) {
            // Storage is unavailable in some privacy modes.
        }

        if (choice !== 'light' && choice !== 'dark') {
            choice = 'system';
        }

        const theme = choice === 'system' ? (media.matches ? 'dark' : 'light') : choice;

        root.dataset.choice = choice;
        root.dataset.theme = theme;
        root.style.colorScheme = theme;

        document.querySelectorAll('[data-theme-choice]').forEach(function (item) {
            const pressed = item.dataset.themeChoice === choice;

            item.setAttribute('aria-pressed', pressed);

            if (pressed) {
                const summary = item.closest('details').querySelector('summary');

                summary.setAttribute('aria-label', summary.dataset.label + ': ' + item.textContent.trim());
            }
        });
    };

    document.addEventListener('click', function (e) {
        const item = e.target.closest('[data-theme-choice]');

        if (item) {
            try {
                localStorage.setItem('theme', item.dataset.themeChoice);
            } catch (err) {
                console.error('Failed to store theme:', err);
            }

            apply();
        }

        document.querySelectorAll('details[data-theme-menu][open]').forEach(function (menu) {
            if (item || !menu.contains(e.target)) {
                menu.removeAttribute('open');
            }
        });
    });

    document.addEventListener('keydown', function (e) {
        const menu = document.querySelector('details[data-theme-menu][open]');

        if (e.key === 'Escape' && menu) {
            menu.removeAttribute('open');
            menu.querySelector('summary').focus();
        }
    });

    media.addEventListener('change', apply);
    document.addEventListener('DOMContentLoaded', apply);
    apply();
})();
