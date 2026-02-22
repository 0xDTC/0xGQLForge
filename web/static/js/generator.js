// 0xGQLForge — Query Generator UI

// ── Active selection state ────────────────────────────────────────────────────
let _activeOpEl = null;

function selectOp(schemaId, opName, kind, el) {
    if (_activeOpEl) _activeOpEl.classList.remove('active');
    _activeOpEl = el;
    el.classList.add('active');
    generateQuery(schemaId, opName, kind);
}

// ── Sidebar search / filter ───────────────────────────────────────────────────
function filterOps(query) {
    const q = query.trim().toLowerCase();
    document.querySelectorAll('.op-item').forEach(item => {
        const match = !q || item.dataset.op.toLowerCase().includes(q);
        item.style.display = match ? '' : 'none';
    });
    document.querySelectorAll('.op-group').forEach(group => {
        const anyVisible = [...group.querySelectorAll('.op-item')].some(i => i.style.display !== 'none');
        group.style.display = anyVisible ? '' : 'none';
    });
}

// ── Toast notification ────────────────────────────────────────────────────────
function showGenToast(msg, isError) {
    let t = document.getElementById('gen-toast');
    if (!t) {
        t = document.createElement('div');
        t.id = 'gen-toast';
        t.style.cssText = 'position:fixed;bottom:2rem;right:2rem;padding:.75rem 1.25rem;border-radius:8px;font-size:.85rem;z-index:200;transition:opacity .3s;pointer-events:none;';
        document.body.appendChild(t);
    }
    t.style.background = isError ? '#dc2626' : '#22c55e';
    t.style.color = '#fff';
    t.style.opacity = '1';
    t.textContent = msg;
    clearTimeout(t._timer);
    t._timer = setTimeout(() => { t.style.opacity = '0'; }, 2200);
}

function copyText(text, label) {
    navigator.clipboard.writeText(text)
        .then(() => showGenToast('Copied ' + (label || '') + ' to clipboard', false))
        .catch(() => showGenToast('Clipboard access denied', true));
}

// ── Copy store: avoids embedding raw text in onclick HTML attributes ──────────
// Embedding JSON.stringify() output inside onclick="..." breaks the HTML
// attribute parser (JSON uses double-quotes, same as the attribute delimiters).
// Instead we store raw text here and reference it by key via data attributes.
const _copyStore = {};

function _bindCopyButtons(container) {
    container.querySelectorAll('.gen-copy-btn').forEach(btn => {
        btn.addEventListener('click', function () {
            const entry = _copyStore[this.dataset.copyKey];
            if (entry) copyText(entry.text, entry.label);
        });
    });
}

// ── Main generate function ────────────────────────────────────────────────────
function generateQuery(schemaId, opName, kind) {
    const depthEl  = document.getElementById('max-depth');
    const maxDepth = depthEl ? (parseInt(depthEl.value) || 3) : 3;
    const result   = document.getElementById('generator-result') || document.getElementById('detail-panel');
    if (!result) return;

    result.className = '';
    result.innerHTML = `
        <div class="gen-loading">
            <div class="gen-spinner"></div>
            <span>Generating <strong>${escHtml(opName)}</strong>…</span>
        </div>`;

    fetch('/api/generate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ schemaId, operation: opName, kind, maxDepth }),
    })
    .then(r => r.json())
    .then(data => {
        result.className = '';
        if (data.error) {
            result.innerHTML = `<div class="parse-result error">${escHtml(data.error)}</div>`;
            return;
        }
        result.innerHTML = buildResultHTML(opName, kind, data);
        _bindCopyButtons(result);
    })
    .catch(err => {
        result.className = '';
        result.innerHTML = `<div class="parse-result error">Network error: ${escHtml(err.message)}</div>`;
    });
}

// ── Result HTML builder ───────────────────────────────────────────────────────
function buildResultHTML(opName, kind, data) {
    let html = `<div class="gen-result">`;

    // Header: op name + complexity badge
    html += `<div class="gen-result-header">
        <div class="gen-result-title">
            <span class="type-badge ${escHtml(kind)}">${escHtml(kind[0].toUpperCase())}</span>
            <h2>${escHtml(opName)}</h2>
        </div>`;

    if (data.complexity) {
        const risk  = escHtml(data.complexity.risk || 'low');
        const score = Math.round(data.complexity.score || 0);
        html += `<div class="gen-complexity">
            <span class="badge badge-${risk}">${risk}</span>
            <span class="gen-complexity-meta">score ${score} &middot; ${data.complexity.fieldCount} fields</span>
        </div>`;
    }
    html += `</div>`;

    // Arguments table
    const args = data.operation && data.operation.args;
    if (args && args.length > 0) {
        html += `<div class="gen-section">
            <div class="gen-section-hd"><span>Arguments</span></div>
            <table class="table">
                <thead><tr><th>Name</th><th>Type</th><th>Required</th><th>Default</th></tr></thead>
                <tbody>`;
        args.forEach(arg => {
            const sig = formatTypeRef(arg.type);
            const req = arg.type && arg.type.kind === 'NON_NULL';
            const def = arg.defaultValue != null
                ? `<code>${escHtml(String(arg.defaultValue))}</code>`
                : `<span style="color:var(--text-muted)">—</span>`;
            html += `<tr>
                <td><code>${escHtml(arg.name)}</code></td>
                <td><code class="type-sig">${escHtml(sig)}</code></td>
                <td>${req ? '<span class="required">Required</span>' : '<span style="color:var(--text-muted)">Optional</span>'}</td>
                <td>${def}</td>
            </tr>`;
        });
        html += `</tbody></table></div>`;
    }

    // Generated query
    const qText = data.query || '';
    html += genSection('Generated Query', escHtml(qText), qText, 'query');

    // Variables
    if (data.variables && Object.keys(data.variables).length > 0) {
        const vText = JSON.stringify(data.variables, null, 2);
        html += genSection('Variables', escHtml(vText), vText, 'variables');
    }

    // cURL
    const body     = JSON.stringify({ query: data.query || '', variables: data.variables || {} });
    const curlText = `curl -X POST \\\n  -H 'Content-Type: application/json' \\\n  -d '${body.replace(/'/g, "'\\''")}' \\\n  <TARGET_URL>/graphql`;
    html += genSection('cURL Command', escHtml(curlText), curlText, 'cURL');

    html += `</div>`;
    return html;
}

// Builds one code section with a safe copy button (no inline JS data).
function genSection(title, displayHtml, rawText, copyLabel) {
    const key = 'copy_' + Math.random().toString(36).slice(2);
    _copyStore[key] = { text: rawText, label: copyLabel };
    return `<div class="gen-section">
        <div class="gen-section-hd">
            <span>${escHtml(title)}</span>
            <button class="btn btn-sm gen-copy-btn" data-copy-key="${key}">Copy</button>
        </div>
        <pre class="code-block">${displayHtml}</pre>
    </div>`;
}

// ── Helpers ───────────────────────────────────────────────────────────────────
function formatTypeRef(ref) {
    if (!ref) return 'Unknown';
    if (ref.kind === 'NON_NULL' && ref.ofType) return formatTypeRef(ref.ofType) + '!';
    if (ref.kind === 'LIST'     && ref.ofType) return '[' + formatTypeRef(ref.ofType) + ']';
    return ref.name || ref.kind || 'Unknown';
}

function escHtml(str) {
    const d = document.createElement('div');
    d.textContent = String(str);
    return d.innerHTML;
}
