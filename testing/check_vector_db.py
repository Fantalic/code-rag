import json
with open('rag_storage/vdb_entities.json', 'r') as f:
    data = json.load(f)
entities = data.get('data', [])
print(f'Total entities in vector DB: {len(entities)}')
for ent in entities:
    name = ent.get('entity_name', ent.get('__id__', 'unknown'))
    print(f'  - {name}')