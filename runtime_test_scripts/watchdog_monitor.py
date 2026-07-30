#!/usr/bin/env python3
"""Monitor watchdog output and check graph state after changes"""
import subprocess
import time
import sys
import os

def run_watchdog(project_path, duration=30):
    cmd = [
        os.path.join(os.path.dirname(__file__), '..', 'watchdog', 'watchdog.exe'),
        'watch', project_path,
        '--debounce', '1',
        '--workers', '2'
    ]

    print(f"Starting watchdog for {project_path}")
    print(f"Duration: {duration} seconds")
    print("-" * 60)

    proc = subprocess.Popen(
        cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1
    )

    start = time.time()
    for line in iter(proc.stdout.readline, ''):
        if line:
            print(line.rstrip())
        if time.time() - start > duration:
            proc.terminate()
            break

    proc.wait()
    print("-" * 60)
    print("Watchdog stopped")

def main():
    if len(sys.argv) < 2:
        print("Usage: python watchdog_monitor.py <project_path> [duration]")
        return 1

    project_path = sys.argv[1]
    duration = int(sys.argv[2]) if len(sys.argv) > 2 else 30

    run_watchdog(project_path, duration)
    return 0

if __name__ == '__main__':
    sys.exit(main())