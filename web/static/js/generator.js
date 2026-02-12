// 0xGQLForge — Query Generator UI

function generateQuery(schemaId, opName, kind) {
    const depthEl = document.getElementById('max-depth');
    const maxDepth = depthEl ? (parseInt(depthEl.value) || 3) : 3;
    const resultDiv = document.getElementById('generator-result') || document.getElementById('detail-panel');

    resultDiv.innerHTML = '<p>Generating query...</p>';

    fetch('/api/generate', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
            schemaId: schemaId,
            operation: opName,
            kind: kind,
            maxDepth: maxDepth
        })
    })
    .then(r => r.json())
    .then(data => {
        if (data.error) {
            resultDiv.innerHTML = '<div class="parse-result error">' + data.error + '</div>';
            return;
        }

        let html = '<div class="detail-header">';
        html += '<h2><span class="type-badge ' + kind + '">' + kind.charAt(0).toUpperCase() + '</span> ' + opName + '</h2>';
        html += '</div>';

        // Complexity badge
        if (data.complexity) {
            html += '<div style="margin-bottom:1rem;">';
            html += 'Complexity: <span class="badge badge-' + data.complexity.risk + '">' + data.complexity.risk + '</span>';
            html += ' (score: ' + data.complexity.score.toFixed(0) + ', fields: ' + data.complexity.fieldCount + ')';
            html += '</div>';
        }

        // Operation details
        if (data.operation && data.operation.args && data.operation.args.length > 0) {
            html += '<div class="detail-section">';
            html += '<h3>Arguments</h3>';
            html += '<table class="table"><thead><tr><th>Name</th><th>Type</th><th>Required</th></tr></thead><tbody>';
            data.operation.args.forEach(arg => {
                const sig = formatTypeRef(arg.type);
                const required = arg.type.kind === 'NON_NULL';
                html += '<tr><td><code>' + arg.name + '</code></td><td><code>' + sig + '</code></td>';
                html += '<td>' + (required ? '<span class="required">Yes</span>' : '') + '</td></tr>';
            });
            html += '</tbody></table></div>';
        }

        // Generated query
        html += '<div class="detail-section">';
        html += '<h3>Generated Query</h3>';
        html += '<div style="position:relative;">';
        html += '<button class="btn btn-sm" style="position:absolute;top:0.5rem;right:0.5rem;" onclick="copyToClipboard(\'query-output\')">Copy</button>';
        html += '<pre class="code-block" id="query-output">' + escapeHtml(data.query) + '</pre>';
        html += '</div></div>';

        // Variables
        if (data.variables && Object.keys(data.variables).length > 0) {
            html += '<div class="detail-section">';
            html += '<h3>Variables</h3>';
            html += '<div style="position:relative;">';
            html += '<button class="btn btn-sm" style="position:absolute;top:0.5rem;right:0.5rem;" onclick="copyToClipboard(\'vars-output\')">Copy</button>';
            html += '<pre class="code-block" id="vars-output">' + escapeHtml(JSON.stringify(data.variables, null, 2)) + '</pre>';
            html += '</div></div>';
        }

        // cURL command
        html += '<div class="detail-section">';
        html += '<h3>cURL Command</h3>';
        const curlBody = JSON.stringify({query: data.query, variables: data.variables || {}});
        const curl = "curl -X POST \\\n  -H 'Content-Type: application/json' \\\n  -d '" + curlBody.replace(/'/g, "'\\''") + "' \\\n  <TARGET_URL>/graphql";
        html += '<pre class="code-block" id="curl-output">' + escapeHtml(curl) + '</pre>';
        html += '</div>';

        resultDiv.innerHTML = html;
    })
    .catch(err => {
        resultDiv.innerHTML = '<div class="parse-result error">Error: ' + err.message + '</div>';
    });
}

function formatTypeRef(ref) {
    if (!ref) return 'Unknown';
    if (ref.kind === 'NON_NULL' && ref.ofType) return formatTypeRef(ref.ofType) + '!';
    if (ref.kind === 'LIST' && ref.ofType) return '[' + formatTypeRef(ref.ofType) + ']';
    return ref.name || ref.kind || 'Unknown';
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function copyToClipboard(elementId) {
    const el = document.getElementById(elementId);
    if (!el) return;
    navigator.clipboard.writeText(el.textContent).then(() => {
        // Brief visual feedback
        el.style.borderColor = '#3fb950';
        setTimeout(() => { el.style.borderColor = ''; }, 500);
    });
}
