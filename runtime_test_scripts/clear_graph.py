import httpx
import json

client = httpx.Client(timeout=10)

resp = client.get('http://localhost:9621/graphs?label=*&max_depth=1&max_nodes=1000')
data = resp.json()

print(f'Found {len(data.get("nodes", []))} nodes to delete')
for n in data.get('nodes', []):
    node_id = n.get('id') or n.get('properties', {}).get('entity_id')
    if node_id:
        try:
            req = httpx.Request('DELETE', 'http://localhost:9621/documents/delete_entity',
                               json={'entity_name': node_id})
            resp2 = client.send(req)
            print(f'  Deleted: {node_id} -> {resp2.status_code}')
        except Exception as e:
            print(f'  Failed to delete {node_id}: {e}')

print('Done')