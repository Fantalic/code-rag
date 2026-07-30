import httpx

resp = httpx.get('http://localhost:9621/graphs?label=*&max_depth=2&max_nodes=1000', follow_redirects=True)
data = resp.json()
print('=== KNOWLEDGE GRAPH ===')
print('Nodes:', len(data.get('nodes', [])))
print('Edges:', len(data.get('edges', [])))
print()
print('--- Classes ---')
for n in data.get('nodes', []):
    props = n.get('properties', {})
    etype = props.get('entity_type', '?')
    if etype == 'CLASS':
        name = n.get('id', props.get('entity_id', '?'))
        desc = props.get('description', '')[:80]
        print(f'  {name}: {desc}')
print()
print('--- Relations ---')
for e in data.get('edges', []):
    src = e.get('source', '?')
    tgt = e.get('target', '?')
    print(f'  {src} -> {tgt}')
print()
print('--- All nodes by type ---')
for n in data.get('nodes', []):
    props = n.get('properties', {})
    etype = props.get('entity_type', '?')
    name = n.get('id', props.get('entity_id', '?'))
    print(f'  {name} [{etype}]')