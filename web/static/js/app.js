// 0xGQLForge — Core application JavaScript

document.addEventListener('DOMContentLoaded', function() {
    initThemeUI();
    highlightNavLink();
});

// Highlight the active navbar link based on the current path.
function highlightNavLink() {
    const path = window.location.pathname;
    document.querySelectorAll('.navbar-link').forEach(a => {
        const href = a.getAttribute('href');
        if (href && (path === href || (href !== '/' && path.startsWith(href)))) {
            a.classList.add('navbar-link-active');
        }
    });
}

// ── Theme toggle ───────────────────────────────────────────────────────────
function initThemeUI() {
    const theme = document.documentElement.getAttribute('data-theme') || 'light';
    _applyThemeUI(theme);
}

function toggleTheme() {
    const current = document.documentElement.getAttribute('data-theme') || 'light';
    const next = current === 'light' ? 'dark' : 'light';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('theme', next);
    _applyThemeUI(next);
}

function _applyThemeUI(theme) {
    const icon  = document.getElementById('theme-icon');
    const label = document.getElementById('theme-label');
    if (!icon || !label) return;
    if (theme === 'light') {
        icon.textContent  = '☾';
        label.textContent = 'Dark Mode';
    } else {
        icon.textContent  = '☀';
        label.textContent = 'Light Mode';
    }
}
