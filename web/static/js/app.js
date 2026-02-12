// GraphScope — Core application JavaScript

document.addEventListener('DOMContentLoaded', function() {
    loadSchemaNav();
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
