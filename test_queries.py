import httpx

client = httpx.Client(timeout=60)

queries = [
    "What classes are in this project?",
    "What methods does the Calculator class have?",
    "What is the DataStore class used for?",
    "List all entities and their relationships",
]

for q in queries:
    print(f"\n{'='*60}")
    print(f"QUERY: {q}")
    print('='*60)
    try:
        resp = client.post('http://localhost:9621/query', json={'query': q, 'mode': 'hybrid'}, timeout=60)
        print(f'Status: {resp.status_code}')
        data = resp.json()
        if 'response' in data:
            text = data['response']
            print(text[:1500] if len(text) > 1500 else text)
        elif 'result' in data:
            print(data['result'][:1500] if len(data.get('result','')) > 1500 else data.get('result',''))
        else:
            print(str(data)[:500])
    except Exception as e:
        print(f'Error: {e}')