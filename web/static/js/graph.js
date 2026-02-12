// GraphScope — D3.js Schema Visualization

(function() {
    if (typeof graphData === 'undefined' || !graphData.nodes) return;

    const container = document.querySelector('.graph-container');
    const svg = d3.select('#schema-graph');
    const tooltip = document.getElementById('graph-tooltip');

    const width = container.clientWidth;
    const height = container.clientHeight;

    svg.attr('viewBox', [0, 0, width, height]);

    // Color mapping by type kind
    const kindColors = {
        'OBJECT': '#3fb950',
        'INPUT_OBJECT': '#bc8cff',
        'ENUM': '#f0883e',
        'INTERFACE': '#79c0ff',
        'UNION': '#db61a2',
        'SCALAR': '#8b949e'
    };

    const rootColors = {
        'query': '#58a6ff',
        'mutation': '#f85149',
        'subscription': '#d29922'
    };

    // Filter out scalar nodes by default
    let showScalars = false;
    let filteredData = filterData();

    function filterData() {
        let nodes = graphData.nodes;
        if (!showScalars) {
            nodes = nodes.filter(n => n.kind !== 'SCALAR');
        }
        const nodeIds = new Set(nodes.map(n => n.id));
        const links = graphData.links.filter(l => nodeIds.has(l.source) && nodeIds.has(l.target));
        return { nodes: [...nodes], links: [...links] };
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
        .attr('fill', '#30363d');

    let simulation, linkElements, nodeElements, labelElements;

    function render(data) {
        g.selectAll('*').remove();

        simulation = d3.forceSimulation(data.nodes)
            .force('link', d3.forceLink(data.links).id(d => d.id).distance(120))
            .force('charge', d3.forceManyBody().strength(-300))
            .force('center', d3.forceCenter(width / 2, height / 2))
            .force('collision', d3.forceCollide().radius(40));

        // Links
        linkElements = g.append('g')
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
        nodeElements = g.append('g')
            .selectAll('circle')
            .data(data.nodes)
            .join('circle')
            .attr('r', d => {
                if (d.isRoot) return 18;
                return Math.max(8, Math.min(16, 6 + d.fieldCount * 0.5));
            })
            .attr('fill', d => {
                if (d.isRoot) return rootColors[d.rootKind] || kindColors[d.kind];
                return kindColors[d.kind] || '#8b949e';
            })
            .attr('stroke', d => d.isRoot ? '#fff' : '#30363d')
            .attr('stroke-width', d => d.isRoot ? 2.5 : 1)
            .attr('cursor', 'pointer')
            .call(drag(simulation));

        // Node labels
        labelElements = g.append('g')
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
                tooltip.innerHTML = `
                    <strong>${d.id}</strong><br>
                    Kind: ${d.kind}<br>
                    Fields: ${d.fieldCount}
                    ${d.isRoot ? '<br>Root: ' + d.rootKind : ''}
                    ${d.description ? '<br>' + d.description : ''}
                `;
            })
            .on('mousemove', (event) => {
                const rect = container.getBoundingClientRect();
                tooltip.style.left = (event.clientX - rect.left + 15) + 'px';
                tooltip.style.top = (event.clientY - rect.top - 10) + 'px';
            })
            .on('mouseout', () => {
                tooltip.style.display = 'none';
            });

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

    function drag(simulation) {
        return d3.drag()
            .on('start', (event, d) => {
                if (!event.active) simulation.alphaTarget(0.3).restart();
                d.fx = d.x;
                d.fy = d.y;
            })
            .on('drag', (event, d) => {
                d.fx = event.x;
                d.fy = event.y;
            })
            .on('end', (event, d) => {
                if (!event.active) simulation.alphaTarget(0);
                d.fx = null;
                d.fy = null;
            });
    }

    render(filteredData);

    // Global functions for controls
    window.resetZoom = function() {
        svg.transition().duration(500).call(zoom.transform, d3.zoomIdentity);
    };

    window.toggleScalars = function(checked) {
        showScalars = checked;
        filteredData = filterData();
        render(filteredData);
    };
})();
