import httpx
client = httpx.Client(base_url='http://localhost:9621', follow_redirects=True, timeout=None)

# Check all labels
r = client.get('/graph/label/list')
labels = r.json()
print('Total entities:', len(labels))
print('Labels:', sorted(labels))
print()

# Check graph edges
r = client.get('/graphs?label=*&max_depth=2&max_nodes=1000')
data = r.json()
edges = data.get('edges', [])
print('Graph edges:', len(edges))
print()

# Test key queries
queries = ['Calculator class', 'Sqrt method', 'AdvancedCalculator inheritance', 'DataStore class methods']
for q in queries:
    r = client.post('/query', json={'query': q})
    resp = r.json()
    print('Query:', q)
    response_text = resp.get('response', '')[:200] if resp else ''
    print('Response:', response_text)
    print()