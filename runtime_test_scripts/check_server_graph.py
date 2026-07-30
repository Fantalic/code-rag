import httpx
client = httpx.Client(base_url='http://localhost:9621', follow_redirects=True)
r = client.get('/graphs?label=*&max_depth=1&max_nodes=1000')
data = r.json()
print(f'Nodes: {len(data.get("nodes", []))}')
print(f'Edges: {len(data.get("edges", []))}')

# List all labels
r = client.get('/graph/label/list')
labels = r.json()
print(f'Labels: {labels}')