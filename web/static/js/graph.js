// 0xGQLForge — D3.js Schema Visualization (card layout)

document.addEventListener('DOMContentLoaded', function() {
    if (typeof graphData === 'undefined' || !graphData.nodes) return;
    if (typeof d3 === 'undefined') return;

    const container = document.querySelector('.graph-container');
    const svg = d3.select('#schema-graph');
    const tooltip = document.getElementById('graph-tooltip');

    const width = container.clientWidth;
    const height = container.clientHeight;

    svg.attr('viewBox', [0, 0, width, height]);

    // Card dimensions
    const NODE_W = 160;
    const NODE_H = 56;

    // Color scheme: border color + fill tint per kind
    const kindStyle = {
        'OBJECT':       { border: '#22c55e', fill: '#0d2015' },
        'INPUT_OBJECT': { border: '#a78bfa', fill: '#1a1030' },
        'ENUM':         { border: '#f97316', fill: '#201408' },
        'INTERFACE':    { border: '#06b6d4', fill: '#061820' },
        'UNION':        { border: '#ec4899', fill: '#200d18' },
        'SCALAR':       { border: '#64748b', fill: '#0e1118' },
    };

    const rootStyle = {
        'query':        { border: '#3b82f6', fill: '#080f20' },
        'mutation':     { border: '#ef4444', fill: '#1e0808' },
        'subscription': { border: '#eab308', fill: '#1c1800' },
    };

    // Extract schema ID from URL (/schema/{id}/graph)
    const schemaId = window.location.pathname.split('/')[2] || '';

    // Build root kind lookup: nodeId -> rootKind
    const rootKindMap = {};
    graphData.nodes.forEach(n => {
        if (n.isRoot) rootKindMap[n.id] = n.rootKind;
    });

    // Map: targetNodeId -> { opName, kind } for nodes directly off a root type
    const nodeOps = {};
    graphData.links.forEach(l => {
        const srcId = typeof l.source === 'object' ? l.source.id : l.source;
        if (rootKindMap[srcId]) {
            const tgtId = typeof l.target === 'object' ? l.target.id : l.target;
            if (!nodeOps[tgtId]) {
                nodeOps[tgtId] = { opName: l.fieldName, kind: rootKindMap[srcId] };
            }
        }
    });

    // Pristine copies — D3 mutates source/target in-place
    const pristineNodes = JSON.parse(JSON.stringify(graphData.nodes));
    const pristineLinks = JSON.parse(JSON.stringify(graphData.links));

    let showScalars = false;
    let simulation = null;

    function filterData() {
        let nodes = pristineNodes;
        if (!showScalars) {
            nodes = nodes.filter(n => n.kind !== 'SCALAR');
        }
        const nodeIds = new Set(nodes.map(n => n.id));
        const links = pristineLinks.filter(l => {
            const s = typeof l.source === 'object' ? l.source.id : l.source;
            const t = typeof l.target === 'object' ? l.target.id : l.target;
            return nodeIds.has(s) && nodeIds.has(t);
        });
        return {
            nodes: JSON.parse(JSON.stringify(nodes)),
            links: JSON.parse(JSON.stringify(links))
        };
    }

    // Compute intersection of a line from (cx,cy) to (tx,ty) with a rect of half-extents (hw,hh)
    function edgePoint(cx, cy, tx, ty, hw, hh) {
        const dx = tx - cx;
        const dy = ty - cy;
        if (dx === 0 && dy === 0) return { x: cx, y: cy };
        const scaleX = dx !== 0 ? hw / Math.abs(dx) : Infinity;
        const scaleY = dy !== 0 ? hh / Math.abs(dy) : Infinity;
        const scale = Math.min(scaleX, scaleY);
        return { x: cx + dx * scale, y: cy + dy * scale };
    }

    // Zoom
    const zoom = d3.zoom()
        .scaleExtent([0.08, 4])
        .on('zoom', (event) => { g.attr('transform', event.transform); });

    svg.call(zoom);

    const g = svg.append('g');

    // Arrow marker
    svg.append('defs').append('marker')
        .attr('id', 'arrowhead')
        .attr('viewBox', '0 -5 10 10')
        .attr('refX', 8)
        .attr('refY', 0)
        .attr('markerWidth', 5)
        .attr('markerHeight', 5)
        .attr('orient', 'auto')
        .append('path')
        .attr('d', 'M0,-5L10,0L0,5')
        .attr('fill', '#4a5568');

    // Toast
    function showToast(msg, isError) {
        let toast = document.getElementById('graph-toast');
        if (!toast) {
            toast = document.createElement('div');
            toast.id = 'graph-toast';
            toast.style.cssText = 'position:fixed;bottom:2rem;right:2rem;padding:0.75rem 1.25rem;border-radius:8px;font-size:0.85rem;z-index:200;transition:opacity 0.3s;pointer-events:none;';
            document.body.appendChild(toast);
        }
        toast.style.background = isError ? '#dc2626' : '#22c55e';
        toast.style.color = '#fff';
        toast.style.opacity = '1';
        toast.textContent = msg;
        clearTimeout(toast._timer);
        toast._timer = setTimeout(() => { toast.style.opacity = '0'; }, 2200);
    }

    function onNodeClick(event, d) {
        const op = nodeOps[d.id];
        if (op && schemaId) {
            generateAndCopy(schemaId, op.opName, op.kind);
            return;
        }
        if (d.isRoot) {
            showToast('Click a connected type to generate its query', false);
            return;
        }
        showToast('No direct operation for ' + d.id, true);
    }

    function generateAndCopy(sid, opName, kind) {
        showToast('Generating ' + opName + '...', false);
        fetch('/api/generate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ schemaId: sid, operation: opName, kind: kind, maxDepth: 3 })
        })
        .then(r => r.json())
        .then(data => {
            if (data.error) { showToast('Error: ' + data.error, true); return; }
            navigator.clipboard.writeText(data.query).then(() => {
                showToast('Copied ' + opName + ' query!', false);
            }).catch(() => {
                showToast('Generated but clipboard access denied', true);
            });
        })
        .catch(err => { showToast('Error: ' + err.message, true); });
    }

    function render(data) {
        if (simulation) simulation.stop();
        g.selectAll('*').remove();

        const hw = NODE_W / 2;
        const hh = NODE_H / 2;

        simulation = d3.forceSimulation(data.nodes)
            .force('link', d3.forceLink(data.links).id(d => d.id).distance(220))
            .force('charge', d3.forceManyBody().strength(-600))
            .force('center', d3.forceCenter(width / 2, height / 2))
            .force('collision', d3.forceCollide().radius(Math.max(hw, hh) + 20));

        // Links
        const linkG = g.append('g');
        const linkLines = linkG.selectAll('line')
            .data(data.links)
            .join('line')
            .attr('stroke', '#2d3548')
            .attr('stroke-width', d => d.isList ? 2 : 1)
            .attr('stroke-dasharray', d => d.isNonNull ? 'none' : '5 3')
            .attr('marker-end', 'url(#arrowhead)');

        // Link labels (shown on link hover via CSS)
        const linkLabels = linkG.selectAll('text')
            .data(data.links)
            .join('text')
            .attr('text-anchor', 'middle')
            .attr('font-size', '9px')
            .attr('fill', '#64748b')
            .attr('pointer-events', 'none')
            .style('opacity', 0)
            .text(d => d.fieldName);

        linkLines
            .on('mouseover', function(_event, d) {
                d3.select(this).attr('stroke', '#94a3b8');
                linkLabels.filter(l => l === d).style('opacity', 1);
            })
            .on('mouseout', function(_event, d) {
                d3.select(this).attr('stroke', '#2d3548');
                linkLabels.filter(l => l === d).style('opacity', 0);
            });

        // Node groups
        const nodeG = g.append('g');
        const nodeGroups = nodeG.selectAll('g')
            .data(data.nodes)
            .join('g')
            .attr('cursor', 'pointer')
            .call(drag(simulation))
            .on('click', onNodeClick);

        // Card background rect
        nodeGroups.append('rect')
            .attr('x', -hw)
            .attr('y', -hh)
            .attr('width', NODE_W)
            .attr('height', NODE_H)
            .attr('rx', 6)
            .attr('ry', 6)
            .attr('fill', d => {
                if (d.isRoot) return (rootStyle[d.rootKind] || {}).fill || '#0d1117';
                return (kindStyle[d.kind] || {}).fill || '#0d1117';
            })
            .attr('stroke', d => {
                if (d.isRoot) return (rootStyle[d.rootKind] || {}).border || '#475569';
                return (kindStyle[d.kind] || {}).border || '#475569';
            })
            .attr('stroke-width', d => d.isRoot ? 2.5 : 1.5)
            .attr('filter', d => d.isRoot ? 'url(#glow)' : null);

        // Glow filter for root nodes
        const defs = svg.select('defs');
        const glowFilter = defs.append('filter').attr('id', 'glow').attr('x', '-30%').attr('y', '-30%').attr('width', '160%').attr('height', '160%');
        glowFilter.append('feGaussianBlur').attr('stdDeviation', '3').attr('result', 'blur');
        const feMerge = glowFilter.append('feMerge');
        feMerge.append('feMergeNode').attr('in', 'blur');
        feMerge.append('feMergeNode').attr('in', 'SourceGraphic');

        // Kind badge — top-left inside card
        nodeGroups.append('text')
            .attr('x', -hw + 7)
            .attr('y', -hh + 11)
            .attr('font-size', '8px')
            .attr('font-family', 'monospace')
            .attr('fill', d => {
                if (d.isRoot) return (rootStyle[d.rootKind] || {}).border || '#94a3b8';
                return (kindStyle[d.kind] || {}).border || '#94a3b8';
            })
            .text(d => d.isRoot ? d.rootKind.toUpperCase() : d.kind);

        // Field count — top-right inside card
        nodeGroups.append('text')
            .attr('x', hw - 7)
            .attr('y', -hh + 11)
            .attr('text-anchor', 'end')
            .attr('font-size', '8px')
            .attr('fill', '#475569')
            .text(d => d.fieldCount > 0 ? d.fieldCount + ' fields' : '');

        // Type name — centered
        nodeGroups.append('text')
            .attr('x', 0)
            .attr('y', 5)
            .attr('text-anchor', 'middle')
            .attr('dominant-baseline', 'middle')
            .attr('font-size', d => d.id.length > 18 ? '10px' : '12px')
            .attr('font-weight', d => d.isRoot ? '700' : '500')
            .attr('fill', '#e2e8f0')
            .text(d => d.id);

        // Click hint dot for nodes with an operation
        nodeGroups.filter(d => !!nodeOps[d.id])
            .append('circle')
            .attr('cx', hw - 6)
            .attr('cy', hh - 6)
            .attr('r', 4)
            .attr('fill', '#22c55e')
            .attr('stroke', '#0a0e14')
            .attr('stroke-width', 1.5);

        // Hover tooltip
        nodeGroups
            .on('mouseover', (_event, d) => {
                tooltip.style.display = 'block';
                const op = nodeOps[d.id];
                const hint = op ? '<br><em style="color:#22c55e">Click to generate &amp; copy: ' + op.opName + '</em>' : '';
                tooltip.innerHTML =
                    '<strong>' + d.id + '</strong><br>' +
                    'Kind: ' + d.kind + '<br>' +
                    'Fields: ' + d.fieldCount +
                    (d.isRoot ? '<br>Root: ' + d.rootKind : '') +
                    (d.description ? '<br><span style="color:#94a3b8">' + d.description + '</span>' : '') +
                    hint;
            })
            .on('mousemove', (event) => {
                const rect = container.getBoundingClientRect();
                tooltip.style.left = (event.clientX - rect.left + 15) + 'px';
                tooltip.style.top  = (event.clientY - rect.top  - 10) + 'px';
            })
            .on('mouseout', () => {
                tooltip.style.display = 'none';
            });

        // Tick: position links to card edges, move node groups
        simulation.on('tick', () => {
            linkLines.each(function(d) {
                const sx = d.source.x, sy = d.source.y;
                const tx = d.target.x, ty = d.target.y;
                const p1 = edgePoint(sx, sy, tx, ty, hw, hh);
                const p2 = edgePoint(tx, ty, sx, sy, hw, hh);
                d3.select(this)
                    .attr('x1', p1.x).attr('y1', p1.y)
                    .attr('x2', p2.x).attr('y2', p2.y);
            });

            linkLabels
                .attr('x', d => (d.source.x + d.target.x) / 2)
                .attr('y', d => (d.source.y + d.target.y) / 2 - 5);

            nodeGroups.attr('transform', d => `translate(${d.x},${d.y})`);
        });
    }

    function drag(sim) {
        return d3.drag()
            .on('start', (event, d) => {
                if (!event.active) sim.alphaTarget(0.3).restart();
                d.fx = d.x; d.fy = d.y;
            })
            .on('drag', (event, d) => {
                d.fx = event.x; d.fy = event.y;
            })
            .on('end', (event, d) => {
                if (!event.active) sim.alphaTarget(0);
                d.fx = null; d.fy = null;
            });
    }

    render(filterData());

    window.resetZoom = function() {
        svg.transition().duration(500).call(zoom.transform, d3.zoomIdentity);
    };

    window.toggleScalars = function(checked) {
        showScalars = checked;
        render(filterData());
    };
});
