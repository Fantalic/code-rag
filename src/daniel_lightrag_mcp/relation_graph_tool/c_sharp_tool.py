import os
import xml.etree.ElementTree as ET
from collections import defaultdict
import re

def find_csproj_files(root):
    csprojs = []
    for dirpath, _, files in os.walk(root):
        for f in files:
            if f.endswith('.csproj'):
                csprojs.append(os.path.join(dirpath, f))
    return csprojs

def parse_project_references(csproj_path):
    tree = ET.parse(csproj_path)
    root = tree.getroot()
    ns = {'msbuild': 'http://schemas.microsoft.com/developer/msbuild/2003'}
    refs = []
    for pr in root.findall('.//ProjectReference', ns):
        ref_path = pr.attrib.get('Include', '')
        ref_proj = os.path.splitext(os.path.basename(ref_path))[0]
        refs.append(ref_proj)
    return refs

def parse_csharp_files(project_dir):
    # Very basic parsing approach: for demo only
    class_relations = defaultdict(lambda: {'inherits': [], 'methods': defaultdict(list)})
    namespace = None
    current_class = None
    current_method = None

    method_call_regex = re.compile(r'\b(\w+)\s*\(')

    for dirpath, _, files in os.walk(project_dir):
        for file in files:
            if file.endswith('.cs'):
                path = os.path.join(dirpath, file)

                lines = []
                # Try reading with utf-8, fallback to latin1 with ignore errors
                try:
                    with open(path, 'r', encoding='utf-8') as f:
                        lines = f.readlines()
                except UnicodeDecodeError:
                    with open(path, 'r', encoding='latin1', errors='ignore') as f:
                        lines = f.readlines()

                if len(lines) == 0:
                    continue
                else:
                # with open(path, 'r', encoding='utf-8') as f:
                    for line in lines:
                        line_strip = line.strip()
                        # Namespace
                        if line_strip.startswith('namespace '):
                            namespace = line_strip.split()[1]
                        # Class & inheritance
                        if line_strip.startswith('public class') or line_strip.startswith('class'):
                            parts = line_strip.split()
                            idx = parts.index('class')
                            current_class = parts[idx+1]
                            if ':' in line_strip:
                                inherits = line_strip.split(':')[1].strip().split()[0]
                                class_relations[current_class]['inherits'].append(inherits)
                        # Method 
                        if re.match(r'public|private|protected', line_strip) and '(' in line_strip and ')' in line_strip and '{' in line_strip:
                            method_name = line_strip.split('(')[0].split()[-1]
                            current_method = method_name
                        # Calls inside methods
                        if current_method:
                            calls = method_call_regex.findall(line_strip)
                            for call in calls:
                                if call != current_method:  # avoid self calls
                                    class_relations[current_class]['methods'][current_method].append(call)
    return namespace, class_relations


def build_relation_txt(project_name, refs, namespace, class_rels, out_path):
    lines = []
    lines.append(f"PROJECT: {project_name}")
    for r in refs:
        lines.append(f"  REFERS_TO -> {r}")
    if namespace:
        lines.append(f"  NAMESPACE: {namespace}")
    for cls, data in class_rels.items():
        lines.append(f"    CLASS: {cls}")
        for base in data['inherits']:
            lines.append(f"      INHERITS_FROM -> {base}")
        for method, calls in data['methods'].items():
            lines.append(f"      METHOD: {method}")
            # for c in calls:
            #     lines.append(f"        METHOD: {method} -> CALLS -> {c}")

    with open(out_path, 'w', encoding='utf-8') as f:
        f.write('\n'.join(lines))


def main(root, output_dir):
    print(" CREATING RELATION GRAPH FILES .... ")
    csproj_files = find_csproj_files(root)
    os.makedirs(output_dir, exist_ok=True)
    for csproj in csproj_files:
        proj_name = os.path.splitext(os.path.basename(csproj))[0]
        project_dir = os.path.dirname(csproj)
        refs = parse_project_references(csproj)
        namespace, class_rels = parse_csharp_files(project_dir)
        out_path = os.path.join(output_dir, f"{proj_name}.txt")
        build_relation_txt(proj_name, refs, namespace, class_rels, out_path)
        print(f"Generated relation graph for project {proj_name} at {out_path}")

if __name__ == "__main__":
    root_folder = "./"
    output_folder = "relation_graph_txt"
    main(root_folder, output_folder)
