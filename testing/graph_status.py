#!/usr/bin/env python3
"""Quick check of graph state"""
import httpx
import sys

def main():
    client = httpx.Client(base_url='http://localhost:9621', timeout=30, follow_redirects=True)

    try:
        # Check labels
        r = client.get('/graph/label/list')
        labels = r.json()
        print(f"Entities: {len(labels)}")

        # Check edges
        r = client.get('/graphs?label=*&max_depth=2&max_nodes=1000')
        data = r.json()
        print(f"Edges: {len(data.get('edges', []))}")

        # Show recent labels
        sorted_labels = sorted(labels)
        print(f"Labels: {sorted_labels[:10]}{'...' if len(sorted_labels) > 10 else ''}")

        return 0 if labels else 1
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1

if __name__ == '__main__':
    sys.exit(main())