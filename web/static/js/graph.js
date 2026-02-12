// 0xGQLForge — D3.js Schema Visualization

document.addEventListener('DOMContentLoaded', function() {
    if (typeof graphData === 'undefined' || !graphData.nodes) return;
    if (typeof d3 === 'undefined') return;

    const container = document.querySelector('.graph-container');
    const svg = d3.select('#schema-graph');
    const tooltip = document.getElementById('graph-tooltip');

    const width = container.clientWidth;
    const height = container.clientHeight;

    svg.attr('viewBox', [0, 0, width, height]);

    // Color mapping by type kind
    const kindColors = {
        'OBJECT': '#22c55e',
        'INPUT_OBJECT': '#a78bfa',
        'ENUM': '#f97316',
        'INTERFACE': '#06b6d4',
        'UNION': '#ec4899',
        'SCALAR': '#64748b'
    };

    const rootColors = {
        'query': '#3b82f6',
        'mutation': '#ef4444',
        'subscription': '#eab308'
    };

    // Extract schema ID from the page URL (/schema/{id}/graph)
    const schemaId = window.location.pathname.split('/')[2] || '';

    // Build a lookup: for each non-root node, find which operation (link from root)
    // points to it, so we can generate a query on click.
    const rootKindMap = {}; // nodeId -> rootKind ("query"/"mutation"/"subscription")
    graphData.nodes.forEach(n => {
        if (n.isRoot) rootKindMap[n.id] = n.rootKind;
    });

    // Map: targetNodeId -> { opName, kind } for nodes directly reachable from a root type
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

    // Keep a pristine copy of nodes/links for re-filtering (D3 mutates them)
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
        const links = pristineLinks.filter(l => nodeIds.has(l.source) && nodeIds.has(l.target));
        // Deep clone to prevent D3 mutation from affecting pristine data
        return {
            nodes: JSON.parse(JSON.stringify(nodes)),
            links: JSON.parse(JSON.stringify(links))
        };
    }

    // Create zoom behavior
    const zoom = d3.zoom()
        .scaleExtent([0.1, 4])
        .on('zoom', (event) => {
            g.attr('transform', event.transform);
        });

    svg.call(zoom);

    const g = svg.append('g');

    // Arrow marker for directed edges
    svg.append('defs').append('marker')
        .attr('id', 'arrowhead')
        .attr('viewBox', '0 -5 10 10')
        .attr('refX', 20)
        .attr('refY', 0)
        .attr('markerWidth', 6)
        .attr('markerHeight', 6)
        .attr('orient', 'auto')
        .append('path')
        .attr('d', 'M0,-5L10,0L0,5')
        .attr('fill', '#2d3548');

    // Toast notification for clipboard feedback
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
        toast._timer = setTimeout(() => { toast.style.opacity = '0'; }, 2000);
    }

    // Click handler: generate query for operations reachable from this node
    function onNodeClick(event, d) {
        // Check if this node is directly returned by a root operation
        const op = nodeOps[d.id];
        if (op && schemaId) {
            generateAndCopy(schemaId, op.opName, op.kind);
            return;
        }
        // If this IS a root type node, show a hint
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
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({ schemaId: sid, operation: opName, kind: kind, maxDepth: 3 })
        })
        .then(r => r.json())
        .then(data => {
            if (data.error) {
                showToast('Error: ' + data.error, true);
                return;
            }
            navigator.clipboard.writeText(data.query).then(() => {
                showToast('Copied ' + opName + ' query to clipboard', false);
            }).catch(() => {
                showToast('Generated but clipboard access denied', true);
            });
        })
        .catch(err => {
            showToast('Error: ' + err.message, true);
        });
    }

    function render(data) {
        // Stop previous simulation
        if (simulation) simulation.stop();

        g.selectAll('*').remove();

        simulation = d3.forceSimulation(data.nodes)
            .force('link', d3.forceLink(data.links).id(d => d.id).distance(120))
            .force('charge', d3.forceManyBody().strength(-300))
            .force('center', d3.forceCenter(width / 2, height / 2))
            .force('collision', d3.forceCollide().radius(40));

        // Links
        const linkElements = g.append('g')
            .selectAll('line')
            .data(data.links)
            .join('line')
            .attr('class', 'link-line')
            .attr('stroke-width', d => d.isList ? 2 : 1)
            .attr('stroke-dasharray', d => d.isNonNull ? 'none' : '4 2')
            .attr('marker-end', 'url(#arrowhead)');

        // Link labels
        const linkLabels = g.append('g')
            .selectAll('text')
            .data(data.links)
            .join('text')
            .attr('class', 'link-label')
            .text(d => d.fieldName);

        // Nodes
        const nodeElements = g.append('g')
            .selectAll('circle')
            .data(data.nodes)
            .join('circle')
            .attr('r', d => {
                if (d.isRoot) return 18;
                return Math.max(8, Math.min(16, 6 + d.fieldCount * 0.5));
            })
            .attr('fill', d => {
                if (d.isRoot) return rootColors[d.rootKind] || kindColors[d.kind];
                return kindColors[d.kind] || '#64748b';
            })
            .attr('stroke', d => d.isRoot ? '#e2e8f0' : '#232a3b')
            .attr('stroke-width', d => d.isRoot ? 2.5 : 1)
            .attr('cursor', 'pointer')
            .call(drag(simulation));

        // Node labels
        const labelElements = g.append('g')
            .selectAll('text')
            .data(data.nodes)
            .join('text')
            .attr('class', 'node-label')
            .attr('dy', d => {
                const r = d.isRoot ? 18 : Math.max(8, Math.min(16, 6 + d.fieldCount * 0.5));
                return r + 14;
            })
            .attr('text-anchor', 'middle')
            .attr('font-weight', d => d.isRoot ? 'bold' : 'normal')
            .text(d => d.id);

        // Hover interactions
        nodeElements
            .on('mouseover', (event, d) => {
                tooltip.style.display = 'block';
                const op = nodeOps[d.id];
                const opHint = op ? '<br><em>Click to generate & copy: ' + op.opName + '</em>' : '';
                tooltip.innerHTML =
                    '<strong>' + d.id + '</strong><br>' +
                    'Kind: ' + d.kind + '<br>' +
                    'Fields: ' + d.fieldCount +
                    (d.isRoot ? '<br>Root: ' + d.rootKind : '') +
                    (d.description ? '<br>' + d.description : '') +
                    opHint;
            })
            .on('mousemove', (event) => {
                const rect = container.getBoundingClientRect();
                tooltip.style.left = (event.clientX - rect.left + 15) + 'px';
                tooltip.style.top = (event.clientY - rect.top - 10) + 'px';
            })
            .on('mouseout', () => {
                tooltip.style.display = 'none';
            })
            .on('click', onNodeClick);

        // Simulation tick
        simulation.on('tick', () => {
            linkElements
                .attr('x1', d => d.source.x)
                .attr('y1', d => d.source.y)
                .attr('x2', d => d.target.x)
                .attr('y2', d => d.target.y);

            linkLabels
                .attr('x', d => (d.source.x + d.target.x) / 2)
                .attr('y', d => (d.source.y + d.target.y) / 2);

            nodeElements
                .attr('cx', d => d.x)
                .attr('cy', d => d.y);

            labelElements
                .attr('x', d => d.x)
                .attr('y', d => d.y);
        });
    }

    function drag(sim) {
        return d3.drag()
            .on('start', (event, d) => {
                if (!event.active) sim.alphaTarget(0.3).restart();
                d.fx = d.x;
                d.fy = d.y;
            })
            .on('drag', (event, d) => {
                d.fx = event.x;
                d.fy = event.y;
            })
            .on('end', (event, d) => {
                if (!event.active) sim.alphaTarget(0);
                d.fx = null;
                d.fy = null;
            });
    }

    render(filterData());

    // Global functions for controls
    window.resetZoom = function() {
        svg.transition().duration(500).call(zoom.transform, d3.zoomIdentity);
    };

    window.toggleScalars = function(checked) {
        showScalars = checked;
        render(filterData());
    };
});
