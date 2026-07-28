"""Test watchdog behavior with file changes."""

import subprocess
import time
import httpx
import os

def get_graph_state():
    client = httpx.Client(base_url='http://localhost:9621', timeout=30)
    r = client.get('/graph/label/list')
    labels = r.json()
    return len(labels), 'Increment' in labels, 'Calculator' in labels

def main():
    test_project = "E:\\_dev\\_Projects\\code-lightrag\\test-csharp-project"
    calc_file = os.path.join(test_project, "TestLib", "Calculator.cs")

    print("=== TESTING GO WATCHDOG ===\n")

    # Get initial state
    count, has_increment, has_calculator = get_graph_state()
    print(f"Initial graph state: {count} labels, Increment={has_increment}, Calculator={has_calculator}")

    # Start watchdog
    print("\n--- Starting watchdog ---")
    proc = subprocess.Popen(
        ["./watchdog.exe", "watch", test_project, "--debounce", "1"],
        cwd="E:\\_dev\\_Projects\\code-lightrag\\watchdog",
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1
    )

    # Read output for a few seconds
    print("Watchdog output (5 seconds):")
    output_lines = []
    for i in range(50):  # ~5 seconds of output
        line = proc.stdout.readline()
        if not line:
            break
        output_lines.append(line.rstrip())
        print(f"  {line.rstrip()}")
        if i >= 30 and "Initial sync" in line:
            break

    # Check graph state after initial sync
    time.sleep(2)
    count, has_increment, has_calculator = get_graph_state()
    print(f"\nAfter initial sync: {count} labels, Increment={has_increment}, Calculator={has_calculator}")

    # Make a modification to Calculator.cs
    print("\n--- Modifying Calculator.cs (adding Modulo method) ---")
    with open(calc_file, 'r') as f:
        content = f.read()

    new_content = content.replace(
        "public int Increment(int value) => value + 1;",
        "public int Increment(int value) => value + 1;\n    public int Modulo(int a, int b) => a % b;"
    )

    with open(calc_file, 'w') as f:
        f.write(new_content)

    print("File modified, waiting for watchdog to detect...")
    time.sleep(5)  # Wait for debounce + processing

    # Check if Modulo appears in graph
    client = httpx.Client(base_url='http://localhost:9621', timeout=30)
    r = client.get('/graph/label/list')
    labels = r.json()
    print(f"After file change: {len(labels)} labels, 'Modulo' in labels: {'Modulo' in labels}")

    # Restore original content
    print("\n--- Restoring Calculator.cs ---")
    with open(calc_file, 'w') as f:
        f.write(content)

    # Cleanup
    print("\n--- Stopping watchdog ---")
    proc.terminate()
    proc.wait(timeout=5)

    print("\n=== TEST COMPLETE ===")

if __name__ == '__main__':
    main()