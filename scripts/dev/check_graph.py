import httpx

resp = httpx.get('http://localhost:9621/graphs?label=*&max_depth=2&max_nodes=1000', timeout=10)
data = resp.json()

print('=== KNOWLEDGE GRAPH ===')
print('Nodes:', len(data.get('nodes', [])), 'Edges:', len(data.get('edges', [])))
print()
print('--- Classes (entity_type=CLASS) ---')
for n in data.get('nodes', []):
    props = n.get('properties', {})
    if props.get('entity_type') == 'CLASS':
        name = n.get('id', '?')
        desc = props.get('description', '')[:80]
        print(f'  {name}: {desc}')

print()
print('--- Methods (entity_type=METHOD) ---')
for n in data.get('nodes', []):
    props = n.get('properties', {})
    if props.get('entity_type') == 'METHOD':
        name = n.get('id', '?')
        desc = props.get('description', '')[:60]
        print(f'  {name}: {desc}')

print()
print('--- Relations (src_id/tgt_id from properties) ---')
for e in data.get('edges', []):
    props = e.get('properties', {})
    src = props.get('src_id', e.get('source', '?'))
    tgt = props.get('tgt_id', e.get('target', '?'))
    desc = props.get('description', '')[:60]
    print(f'  {src} -> {tgt}: {desc}')

print()
print('--- Raw source/target (alphabetically sorted by LightRAG) ---')
for e in data.get('edges', []):
    print(f'  {e.get("id")}: source={e.get("source")} target={e.get("target")}')