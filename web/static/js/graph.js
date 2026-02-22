// 0xGQLForge — D3.js Schema Visualization (Voyager ERD, static hierarchical layout)

document.addEventListener('DOMContentLoaded', function () {
    if (typeof graphData === 'undefined' || !graphData.nodes) return;
    if (typeof d3 === 'undefined') return;

    const container = document.querySelector('.graph-container');
    const svg = d3.select('#schema-graph');
    const tooltip = document.getElementById('graph-tooltip');

    const width  = container.clientWidth;
    const height = container.clientHeight;
    svg.attr('viewBox', [0, 0, width, height]);

    // ── Layout constants ──────────────────────────────────────────────────────
    const NODE_W     = 280;
    const HEADER_H   = 36;
    const FIELD_H    = 22;
    const MAX_FIELDS = 14;
    const COL_X_GAP  = 90;
    const ROW_Y_GAP  = 24;

    // ── Color scheme ──────────────────────────────────────────────────────────
    const kindStyle = {
        'OBJECT':       { header: '#0e7490', text: '#cffafe', link: '#67e8f9' },
        'INPUT_OBJECT': { header: '#7c3aed', text: '#ede9fe', link: '#c4b5fd' },
        'ENUM':         { header: '#c2410c', text: '#ffedd5', link: '#fdba74' },
        'INTERFACE':    { header: '#0369a1', text: '#e0f2fe', link: '#7dd3fc' },
        'UNION':        { header: '#be185d', text: '#fce7f3', link: '#f9a8d4' },
        'SCALAR':       { header: '#334155', text: '#f1f5f9', link: '#94a3b8' },
    };
    const rootStyle = {
        'query':        { header: '#1d4ed8', text: '#dbeafe', link: '#93c5fd' },
        'mutation':     { header: '#b91c1c', text: '#fee2e2', link: '#fca5a5' },
        'subscription': { header: '#92400e', text: '#fef3c7', link: '#fcd34d' },
    };

    function getStyle(d) {
        if (d.isRoot) return rootStyle[d.rootKind] || kindStyle['OBJECT'];
        return kindStyle[d.kind] || kindStyle['SCALAR'];
    }

    // Truncate SVG text to prevent overlap
    function truncText(s, max) {
        return s && s.length > max ? s.slice(0, max - 1) + '…' : (s || '');
    }

    // ── Node geometry ─────────────────────────────────────────────────────────
    function nodeHeight(d) {
        const n = (d.fields || []).length;
        return HEADER_H + Math.min(n, MAX_FIELDS) * FIELD_H + (n > MAX_FIELDS ? FIELD_H : 0);
    }

    function fieldConnectorY(node, fieldName) {
        const fields = node.fields || [];
        const top    = node.y - nodeHeight(node) / 2;
        const idx    = fields.findIndex(f => f.name === fieldName);
        const row    = idx < 0 ? 0 : Math.min(idx, MAX_FIELDS - 1);
        return top + HEADER_H + row * FIELD_H + FIELD_H / 2;
    }

    function linkPath(d) {
        if (!d.source || !d.target) return '';
        const x1 = d.source.x + NODE_W / 2;
        const y1 = fieldConnectorY(d.source, d.fieldName);
        const x2 = d.target.x - NODE_W / 2;
        const y2 = d.target.y - nodeHeight(d.target) / 2 + HEADER_H / 2;
        const cp = Math.max(80, Math.abs(x2 - x1) * 0.45);
        return `M ${x1} ${y1} C ${x1 + cp} ${y1}, ${x2 - cp} ${y2}, ${x2} ${y2}`;
    }

    // ── Operation lookup ──────────────────────────────────────────────────────
    const schemaId    = window.location.pathname.split('/')[2] || '';
    const rootKindMap = {};
    graphData.nodes.forEach(n => { if (n.isRoot) rootKindMap[n.id] = n.rootKind; });

    const nodeOps = {};
    graphData.links.forEach(l => {
        const srcId = typeof l.source === 'object' ? l.source.id : l.source;
        if (rootKindMap[srcId]) {
            const tgtId = typeof l.target === 'object' ? l.target.id : l.target;
            if (!nodeOps[tgtId]) nodeOps[tgtId] = { opName: l.fieldName, kind: rootKindMap[srcId] };
        }
    });

    // ── Pristine data ─────────────────────────────────────────────────────────
    const pristineNodes = JSON.parse(JSON.stringify(graphData.nodes));
    const pristineLinks = JSON.parse(JSON.stringify(graphData.links));

    let showScalars = false;

    function filterData() {
        let nodes = pristineNodes;
        if (!showScalars) nodes = nodes.filter(n => n.kind !== 'SCALAR');
        const ids = new Set(nodes.map(n => n.id));
        const links = pristineLinks.filter(l => ids.has(l.source) && ids.has(l.target));
        return {
            nodes: JSON.parse(JSON.stringify(nodes)),
            links: JSON.parse(JSON.stringify(links)),
        };
    }

    // ── Hierarchical layout ───────────────────────────────────────────────────
    function computeLayout(nodes, links) {
        const nodeById = new Map(nodes.map(n => [n.id, n]));

        // Resolve string IDs → node objects
        links.forEach(l => {
            if (typeof l.source === 'string') l.source = nodeById.get(l.source);
            if (typeof l.target === 'string') l.target = nodeById.get(l.target);
        });

        // BFS from root nodes to assign columns
        const colMap = {};
        const queue  = [];
        nodes.forEach(n => { if (n.isRoot) { colMap[n.id] = 0; queue.push(n.id); } });

        let qi = 0;
        while (qi < queue.length) {
            const id = queue[qi++];
            links.forEach(l => {
                if (!l.source || !l.target) return;
                const src = l.source.id, tgt = l.target.id;
                if (src === id && colMap[tgt] === undefined) {
                    colMap[tgt] = colMap[id] + 1;
                    queue.push(tgt);
                }
            });
        }

        // Orphans go to the column after the deepest reachable
        const maxReached = Object.values(colMap).reduce((m, v) => Math.max(m, v), 0);
        nodes.forEach(n => { if (colMap[n.id] === undefined) colMap[n.id] = maxReached + 1; });

        // Group by column, sort: roots first then alphabetically
        const byCol = {};
        nodes.forEach(n => { (byCol[colMap[n.id]] = byCol[colMap[n.id]] || []).push(n); });
        Object.values(byCol).forEach(arr =>
            arr.sort((a, b) => (b.isRoot - a.isRoot) || a.id.localeCompare(b.id))
        );

        const maxCol    = Math.max(...Object.keys(byCol).map(Number));
        const totalW    = (maxCol + 1) * (NODE_W + COL_X_GAP) - COL_X_GAP;
        const startX    = Math.max(NODE_W / 2 + 20, width / 2 - totalW / 2 + NODE_W / 2);

        Object.entries(byCol).forEach(([cStr, colNodes]) => {
            const c      = parseInt(cStr);
            const x      = startX + c * (NODE_W + COL_X_GAP);
            const totalH = colNodes.reduce((s, n) => s + nodeHeight(n) + ROW_Y_GAP, -ROW_Y_GAP);
            let y        = height / 2 - totalH / 2;
            colNodes.forEach(n => {
                n.x = x;
                n.y = y + nodeHeight(n) / 2;
                y  += nodeHeight(n) + ROW_Y_GAP;
            });
        });
    }

    // ── Zoom ──────────────────────────────────────────────────────────────────
    const zoom = d3.zoom()
        .scaleExtent([0.05, 4])
        .on('zoom', e => g.attr('transform', e.transform));
    svg.call(zoom);

    // Click on bare SVG background → clear focus
    svg.on('click', e => {
        if (e.target === svg.node()) clearFocus();
    });

    const g    = svg.append('g');
    const defs = svg.append('defs');

    defs.append('marker')
        .attr('id', 'erd-arrow')
        .attr('viewBox', '0 -4 8 8')
        .attr('refX', 7).attr('refY', 0)
        .attr('markerWidth', 5).attr('markerHeight', 5)
        .attr('orient', 'auto')
        .append('path').attr('d', 'M0,-4L8,0L0,4').attr('fill', '#4a5568');

    // ── Focus / highlight state ───────────────────────────────────────────────
    let focusedId   = null;
    let linkSel     = null;
    let nodeSel     = null;
    let activeLinks = [];  // resolved link objects from current render
    let activeNodes = [];  // resolved node objects from current render

    function clearFocus() {
        focusedId = null;
        if (nodeSel) nodeSel.transition().duration(250).attr('opacity', 1);
        if (linkSel) linkSel.transition().duration(250)
            .attr('opacity', 1).attr('stroke', '#2d3548');
    }

    function applyFocus(id) {
        // Toggle off if clicking the already-focused node
        if (focusedId === id) { clearFocus(); return; }
        focusedId = id;

        // Build set of directly connected node IDs
        const connIds = new Set([id]);
        activeLinks.forEach(l => {
            const s = l.source ? (l.source.id || l.source) : null;
            const t = l.target ? (l.target.id || l.target) : null;
            if (s === id && t) connIds.add(t);
            if (t === id && s) connIds.add(s);
        });

        // Dim unrelated nodes; fade to near-invisible
        if (nodeSel) {
            nodeSel.transition().duration(250)
                .attr('opacity', d => connIds.has(d.id) ? 1 : 0.08);
        }

        // Dim unrelated links; brighten connected ones
        if (linkSel) {
            linkSel.transition().duration(250)
                .attr('opacity', d => {
                    const s = d.source ? (d.source.id || d.source) : null;
                    const t = d.target ? (d.target.id || d.target) : null;
                    return (s === id || t === id) ? 1 : 0.03;
                })
                .attr('stroke', d => {
                    const s = d.source ? (d.source.id || d.source) : null;
                    const t = d.target ? (d.target.id || d.target) : null;
                    return (s === id || t === id) ? '#94a3b8' : '#2d3548';
                });
        }
    }

    // ── Toast ─────────────────────────────────────────────────────────────────
    function showToast(msg, isError) {
        let t = document.getElementById('graph-toast');
        if (!t) {
            t = document.createElement('div');
            t.id = 'graph-toast';
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

    function generateAndCopy(sid, opName, kind) {
        showToast('Generating ' + opName + '...', false);
        fetch('/api/generate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ schemaId: sid, operation: opName, kind: kind, maxDepth: 3 }),
        })
        .then(r => r.json())
        .then(data => {
            if (data.error) { showToast('Error: ' + data.error, true); return; }
            navigator.clipboard.writeText(data.query)
                .then(() => showToast('Copied ' + opName + ' query!', false))
                .catch(() => showToast('Generated but clipboard access denied', true));
        })
        .catch(err => showToast('Error: ' + err.message, true));
    }

    // ── Render ────────────────────────────────────────────────────────────────
    function redrawLinks() {
        if (linkSel) linkSel.attr('d', linkPath);
    }

    function render(data) {
        focusedId = null;
        g.selectAll('*').remove();

        computeLayout(data.nodes, data.links);
        activeLinks = data.links;
        activeNodes = data.nodes;

        // ── Links ──────────────────────────────────────────────────────────────
        linkSel = g.append('g')
            .selectAll('path')
            .data(data.links)
            .join('path')
            .attr('fill', 'none')
            .attr('stroke', '#2d3548')
            .attr('stroke-width', d => d.isList ? 2 : 1.5)
            .attr('stroke-dasharray', d => d.isNonNull ? 'none' : '6 3')
            .attr('marker-end', 'url(#erd-arrow)')
            .attr('d', linkPath)
            .attr('opacity', 0)  // fade-in
            .on('mouseover', function (_e, d) {
                if (focusedId) return; // don't interfere when focused
                d3.select(this).attr('stroke', '#64748b');
                tooltip.style.display = 'block';
                tooltip.innerHTML =
                    `<strong>${d.fieldName}</strong><br>` +
                    `${d.isList ? '[List]' : 'Single'} · ${d.isNonNull ? 'NonNull' : 'Nullable'}`;
            })
            .on('mousemove', e => {
                const r = container.getBoundingClientRect();
                tooltip.style.left = (e.clientX - r.left + 15) + 'px';
                tooltip.style.top  = (e.clientY - r.top  - 10) + 'px';
            })
            .on('mouseout', function () {
                if (!focusedId) d3.select(this).attr('stroke', '#2d3548');
                tooltip.style.display = 'none';
            });

        // Fade links in
        linkSel.transition().duration(400).attr('opacity', 1);

        // ── Node groups ────────────────────────────────────────────────────────
        nodeSel = g.append('g')
            .selectAll('g')
            .data(data.nodes)
            .join('g')
            .attr('cursor', 'pointer')
            .attr('transform', d => `translate(${d.x},${d.y})`)
            .attr('opacity', 0)  // fade-in
            .call(dragBehavior())
            .on('click', (_e, d) => applyFocus(d.id));

        nodeSel.each(function (d) { buildCard(d3.select(this), d); });

        // Fade nodes in with a stagger by column order (left → right)
        nodeSel.transition().duration(350)
            .delay((_d, i) => i * 12)
            .attr('opacity', 1);

        // Tooltip
        nodeSel
            .on('mouseover', (_e, d) => {
                // Hover dimming preview (only when nothing is focused)
                if (!focusedId) {
                    const connIds = new Set([d.id]);
                    activeLinks.forEach(l => {
                        const s = l.source ? (l.source.id || l.source) : null;
                        const t = l.target ? (l.target.id || l.target) : null;
                        if (s === d.id && t) connIds.add(t);
                        if (t === d.id && s) connIds.add(s);
                    });
                    nodeSel.transition().duration(120)
                        .attr('opacity', n => connIds.has(n.id) ? 1 : 0.35);
                    linkSel.transition().duration(120)
                        .attr('opacity', l => {
                            const s = l.source ? (l.source.id || l.source) : null;
                            const t = l.target ? (l.target.id || l.target) : null;
                            return (s === d.id || t === d.id) ? 1 : 0.15;
                        });
                }
                tooltip.style.display = 'block';
                const op   = nodeOps[d.id];
                const hint = op
                    ? `<br><em style="color:#22c55e">Green dot → generate &amp; copy: ${op.opName}</em>`
                    : '';
                tooltip.innerHTML =
                    `<strong>${d.id}</strong><br>Kind: ${d.kind}<br>Fields: ${d.fieldCount}` +
                    (d.isRoot ? `<br>Root: ${d.rootKind}` : '') +
                    (d.description ? `<br><span style="color:#94a3b8">${d.description}</span>` : '') +
                    hint;
            })
            .on('mousemove', e => {
                const r = container.getBoundingClientRect();
                tooltip.style.left = (e.clientX - r.left + 15) + 'px';
                tooltip.style.top  = (e.clientY - r.top  - 10) + 'px';
            })
            .on('mouseout', () => {
                if (!focusedId) {
                    nodeSel.transition().duration(200).attr('opacity', 1);
                    linkSel.transition().duration(200).attr('opacity', 1);
                }
                tooltip.style.display = 'none';
            });
    }

    // ── Card builder ──────────────────────────────────────────────────────────
    function buildCard(ng, d) {
        const style   = getStyle(d);
        const h       = nodeHeight(d);
        const hw      = NODE_W / 2;
        const fields  = d.fields || [];
        const visible = fields.slice(0, MAX_FIELDS);
        const extra   = fields.length - MAX_FIELDS;

        // Outer glow ring visible when focused (initially hidden)
        ng.append('rect')
            .attr('class', 'focus-ring')
            .attr('x', -hw - 3).attr('y', -h / 2 - 3)
            .attr('width', NODE_W + 6).attr('height', h + 6)
            .attr('rx', 8).attr('fill', 'none')
            .attr('stroke', style.header).attr('stroke-width', 2)
            .attr('opacity', 0);

        // Card body
        ng.append('rect')
            .attr('x', -hw).attr('y', -h / 2)
            .attr('width', NODE_W).attr('height', h)
            .attr('rx', 6).attr('fill', '#0d1117')
            .attr('stroke', style.header)
            .attr('stroke-width', d.isRoot ? 2 : 1)
            .attr('stroke-opacity', d.isRoot ? 0.9 : 0.45);

        // Header (rounded top + square-bottom trick)
        ng.append('rect')
            .attr('x', -hw).attr('y', -h / 2)
            .attr('width', NODE_W).attr('height', HEADER_H)
            .attr('rx', 6).attr('fill', style.header);
        ng.append('rect')
            .attr('x', -hw).attr('y', -h / 2 + HEADER_H - 6)
            .attr('width', NODE_W).attr('height', 6)
            .attr('fill', style.header);

        // Kind badge
        ng.append('text')
            .attr('x', -hw + 7).attr('y', -h / 2 + 10)
            .attr('font-size', '7.5px').attr('font-family', 'monospace')
            .attr('fill', style.text).attr('opacity', 0.7)
            .text(d.isRoot ? d.rootKind.toUpperCase() : d.kind);

        // Type name
        ng.append('text')
            .attr('x', 0).attr('y', -h / 2 + HEADER_H / 2)
            .attr('text-anchor', 'middle').attr('dominant-baseline', 'middle')
            .attr('font-size', d.id.length > 22 ? '10px' : '12px')
            .attr('font-weight', '700').attr('fill', style.text)
            .attr('letter-spacing', '0.02em')
            .text(d.id);

        // Separator
        ng.append('line')
            .attr('x1', -hw).attr('y1', -h / 2 + HEADER_H)
            .attr('x2',  hw).attr('y2', -h / 2 + HEADER_H)
            .attr('stroke', '#1e293b').attr('stroke-width', 1);

        // Field rows
        visible.forEach((f, i) => {
            const ry = -h / 2 + HEADER_H + i * FIELD_H;
            ng.append('rect')
                .attr('x', -hw).attr('y', ry)
                .attr('width', NODE_W).attr('height', FIELD_H)
                .attr('fill', i % 2 === 0 ? '#0f172a' : '#0d1117');
            ng.append('line')
                .attr('x1', -hw).attr('y1', ry + FIELD_H)
                .attr('x2',  hw).attr('y2', ry + FIELD_H)
                .attr('stroke', '#1e293b').attr('stroke-width', 0.5);
            ng.append('text')
                .attr('x', -hw + 10).attr('y', ry + FIELD_H / 2)
                .attr('dominant-baseline', 'middle')
                .attr('font-size', '10.5px').attr('fill', '#cbd5e1')
                .text(truncText(f.name, 20));
            if (f.typeSig) {
                ng.append('text')
                    .attr('x', hw - (f.isLink ? 16 : 10)).attr('y', ry + FIELD_H / 2)
                    .attr('text-anchor', 'end').attr('dominant-baseline', 'middle')
                    .attr('font-size', '9.5px')
                    .attr('fill', f.isLink ? style.link : '#475569')
                    .text(truncText(f.typeSig, 18));
            }
            if (f.isLink) {
                ng.append('circle')
                    .attr('cx', hw).attr('cy', ry + FIELD_H / 2)
                    .attr('r', 3.5).attr('fill', style.link)
                    .attr('stroke', '#0d1117').attr('stroke-width', 1.5);
            }
        });

        // "…N more" row
        if (extra > 0) {
            const ry = -h / 2 + HEADER_H + MAX_FIELDS * FIELD_H;
            ng.append('rect')
                .attr('x', -hw).attr('y', ry)
                .attr('width', NODE_W).attr('height', FIELD_H).attr('fill', '#0a0e14');
            ng.append('text')
                .attr('x', 0).attr('y', ry + FIELD_H / 2)
                .attr('text-anchor', 'middle').attr('dominant-baseline', 'middle')
                .attr('font-size', '10px').attr('fill', '#334155')
                .text(`…${extra} more fields`);
        }

        // Green dot = click to generate (separate click target, stopPropagation)
        if (nodeOps[d.id]) {
            const op = nodeOps[d.id];
            ng.append('circle')
                .attr('class', 'gen-btn')
                .attr('cx', hw - 9).attr('cy', -h / 2 + HEADER_H - 9)
                .attr('r', 5).attr('fill', '#22c55e')
                .attr('stroke', '#0a0e14').attr('stroke-width', 1.5)
                .attr('cursor', 'pointer')
                .on('click', e => {
                    e.stopPropagation();  // don't trigger focus
                    if (schemaId) generateAndCopy(schemaId, op.opName, op.kind);
                })
                .on('mouseover', function () { d3.select(this).attr('r', 6.5).attr('fill', '#4ade80'); })
                .on('mouseout',  function () { d3.select(this).attr('r', 5).attr('fill', '#22c55e'); });
        }
    }

    // ── Pan viewport to fully reveal a node ──────────────────────────────────
    function panToNode(id) {
        const nd = activeNodes.find(n => n.id === id);
        if (!nd) return;
        const h   = nodeHeight(nd);
        const hw  = NODE_W / 2;
        const pad = 32;
        const vw  = container.clientWidth;
        const vh  = container.clientHeight;
        const t   = d3.zoomTransform(svg.node());

        // Node bounding box in screen-space
        const sx1 = nd.x * t.k + t.x - hw  * t.k;
        const sy1 = nd.y * t.k + t.y - (h / 2) * t.k;
        const sx2 = nd.x * t.k + t.x + hw  * t.k;
        const sy2 = nd.y * t.k + t.y + (h / 2) * t.k;

        // If the node is taller/wider than the viewport at current zoom, fit it
        if ((sx2 - sx1) > vw - 2 * pad || (sy2 - sy1) > vh - 2 * pad) {
            const scale = Math.min(
                (vw - 2 * pad) / (NODE_W),
                (vh - 2 * pad) / h,
                t.k  // never zoom in beyond current scale
            );
            const tx = vw / 2 - nd.x * scale;
            const ty = vh / 2 - nd.y * scale;
            svg.transition().duration(400)
                .call(zoom.transform, d3.zoomIdentity.translate(tx, ty).scale(scale));
            return;
        }

        // Otherwise just nudge the pan so all four edges are inside the viewport
        let dx = 0, dy = 0;
        if      (sx1 < pad)      dx = pad - sx1;
        else if (sx2 > vw - pad) dx = (vw - pad) - sx2;
        if      (sy1 < pad)      dy = pad - sy1;
        else if (sy2 > vh - pad) dy = (vh - pad) - sy2;

        if (dx !== 0 || dy !== 0) {
            svg.transition().duration(350).call(zoom.translateBy, dx / t.k, dy / t.k);
        }
    }

    // ── Focus ring on focused node ────────────────────────────────────────────
    const _origApplyFocus = applyFocus;
    function applyFocusWithRing(id) {
        _origApplyFocus(id);
        if (nodeSel) {
            nodeSel.selectAll('.focus-ring')
                .transition().duration(250)
                .attr('opacity', 0);
            if (focusedId) {
                nodeSel.filter(d => d.id === focusedId)
                    .select('.focus-ring')
                    .transition().duration(250)
                    .attr('opacity', 0.6);
            }
        }
        // Pan viewport so the focused node is fully visible
        if (focusedId) panToNode(focusedId);
    }

    // Patch the click handler to use the ring version
    // (We rebind after render so we reference the current nodeSel)

    // ── Drag (manual, no simulation) ─────────────────────────────────────────
    function dragBehavior() {
        let dragging = false;
        return d3.drag()
            .on('start', () => { dragging = false; })
            .on('drag', function (e, d) {
                dragging = true;
                d.x = e.x; d.y = e.y;
                d3.select(this).attr('transform', `translate(${d.x},${d.y})`);
                redrawLinks();
            })
            .on('end', () => {
                // If it was just a click (no actual drag movement), don't update focus
                // The 'click' event fires naturally after 'end' when dragging = false
                if (dragging) {
                    // Re-draw links one final time with settled position
                    redrawLinks();
                }
            });
    }

    // ── Initial render ────────────────────────────────────────────────────────
    render(filterData());

    // After render, rebind click to use ring version
    function rebindClick() {
        if (nodeSel) {
            nodeSel.on('click', (_e, d) => {
                applyFocusWithRing(d.id);
            });
        }
    }
    rebindClick();

    // ── Global controls ───────────────────────────────────────────────────────
    window.resetZoom = function () {
        svg.transition().duration(400).call(zoom.transform, d3.zoomIdentity);
        render(filterData());
        rebindClick();
    };

    window.toggleScalars = function (checked) {
        showScalars = checked;
        render(filterData());
        rebindClick();
    };
});
