// 0xGQLForge — Core application JavaScript

document.addEventListener('DOMContentLoaded', function() {
    loadSchemaNav();
    initThemeUI();
});

function loadSchemaNav() {
    fetch('/api/schemas')
        .then(r => r.json())
        .then(schemas => {
            const nav = document.getElementById('schema-nav');
            if (!nav || !schemas || schemas.length === 0) return;
            nav.innerHTML = '';
            schemas.forEach(s => {
                const li = document.createElement('li');
                const a = document.createElement('a');
                a.href = '/schema/' + s.id;
                a.className = 'nav-link';
                a.textContent = s.name;
                li.appendChild(a);
                nav.appendChild(li);
            });
        })
        .catch(() => {});
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
