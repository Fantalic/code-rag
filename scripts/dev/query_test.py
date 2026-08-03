#!/usr/bin/env python3
"""Test semantic queries against the vector DB"""
import httpx
import sys

QUERIES = [
    "What classes are in this project?",
    "What methods does this project have?",
    "What is the main purpose of this codebase?",
    "List all entities and their relationships",
    "How are the classes organized?",
]

def main():
    client = httpx.Client(base_url='http://localhost:9621', follow_redirects=True)

    for query in QUERIES:
        print(f"\n{'='*60}")
        print(f"Query: {query}")
        print('='*60)
        try:
            r = client.post('/query', json={'query': query})
            resp = r.json()
            print(resp.get('response', 'No response')[:400])
        except Exception as e:
            print(f"Error: {e}")

    return 0

if __name__ == '__main__':
    sys.exit(main())