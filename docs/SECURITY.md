# Security

> What codeRAG does to keep secrets out of your knowledge graph, and what you should
> know before indexing a private repository.

---

## 1. The threat model

codeRAG builds a **local** knowledge graph from your source. It talks to no cloud
services; nothing is uploaded. The practical risks are:

1. **Secrets embedded in code** (API keys, tokens, passwords) landing in the graph
   and then in the vector store — where they could be echoed back by AI tools or
   read by anyone with access to `rag_storage`.
2. **PII** (emails, IP addresses, internal paths) leaking into descriptions.
3. The graph being served on `:9621` / `:9622` — **both bind to all interfaces by
   default** inside Docker. Anyone who can reach those ports can read the graph.

## 2. The security filter

Every entity description, relation description, and keyword passes through a filter
**after analysis and before graph insertion**. Matched values are replaced with
`[REDACTED]`.

| Category | Examples |
|----------|----------|
| API keys | `sk-…`, `pk-…`, `ghp_…`, `gho_…`, `ghu_…`, `AKIA…` |
| Auth tokens | `Bearer …`, JWTs (`eyJ…`), `token=` |
| Passwords | `password=`, `passwd=`, `PASS=`, `secret=` |
| Private keys | `-----BEGIN … PRIVATE KEY-----` (RSA, EC, OpenSSH PEM) |
| PII | emails, IPv4, IPv6 |
| Internal paths | `\\server\share`, `//internal/…`, UNC/SMB paths |
| Env secrets | values of variables named `*SECRET*`, `*TOKEN*` |

The filter operates on structured data (not raw file bytes), so it survives nested
maps/lists and relation keyword fields.

## 3. What is NOT filtered

- **Raw `.cs` content is not ingested** in Path A (only extracted entities/relations
  are), so most source text never reaches the graph. Prose files ingested via
  LightRAG's pipeline **do** store chunk content — review them before indexing.
- The **analyser output** (`<project>/.coderag/analyser_output/*.txt`) contains the
  parsed symbol lines. It is local disk, but treat it like source code.
- Entity **names** are stored verbatim (e.g. a class literally named `MyApiKey`).
  We don't rename symbols, only redact matched values.

## 4. Network exposure

- `:9621` and `:9622` are published on all host interfaces by the compose file.
  On a shared network, either:
  - bind to localhost only — edit `docker-compose.yml` ports to `127.0.0.1:9621:9621`
    (same for 9622), or
  - run `docker compose up` behind a firewall / VPN.
- `LIGHTRAG_API_KEY` is optional and **unused in local mode** — do not rely on it as
  an access control mechanism. If you need auth, put the API behind a reverse proxy.

## 5. Recommendations

- Add codeRAG's ports to your local firewall rules (allow localhost only).
- Don't point `coderag` at repositories containing live credentials; rotate any
  secret that was ever scanned.
- For privacy-sensitive deployments, consider hashing `file_path` (not done by
  default — paths are stored as-is).
- Use `coderag scan --clean` + wipe `<project>/.coderag` when you decommission a
  workspace to remove the graph from disk.

## 6. Reporting

This project is experimental. If you find a redaction bypass or another security
issue, report it privately to the maintainers rather than opening a public issue
with exploit details.

---

Back to [README](../README.md) · [DOCUMENTATION.md](../DOCUMENTATION.md)
