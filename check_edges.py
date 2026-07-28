import httpx

resp = httpx.get('http://localhost:9621/graphs?label=*&max_depth=2&max_nodes=1000', timeout=10)
data = resp.json()

print('RAW EDGES:')
for e in data.get('edges', []):
    sid = e.get('id', '?')
    src = e.get('source', '?')
    tgt = e.get('target', '?')
    print(f'  {sid}: src={src} tgt={tgt}')