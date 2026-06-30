---
name: knowledge-master
type: mcp_server
description: Local knowledge graph — semantic search, blast radius, ownership, and convention checks over your code/docs.
version: 1
transport: stdio
command:
  - km-server
install:
  pip: knowledge-master
tool_prefix: km
allowed_agents:
  - m
  - moe-router
  - moe-reasoning
  - moe-longctx
---
Knowledge Master gives agents persistent, structured memory about a codebase.
Unlike flat keyword search, it builds a graph of files, services, people, and
technologies, so agents can reason about relationships and impact.

Tools are exposed under the `km__` prefix:

- `km__search` — semantic search across indexed code, docs, and emails with
  graph context (author, repo, relationships).
- `km__blast_radius` — multi-layer dependency analysis: what breaks if you
  change a file, function, service, or technology.
- `km__safe_to_change` — risk assessment combining blast radius and test
  coverage (safe / risky / dangerous).
- `km__who_owns` — file ownership from git blame, weighted by recency.
- `km__check_conventions` — verify code follows detected team conventions.
- `km__index_repo` / `km__index_directory` — add a repo or directory to the
  knowledge graph.
- `km__get_status` — knowledge base statistics (chunks, documents, repos).

Setup (one-time):

    pipx install knowledge-master   # or: pip install knowledge-master
    km start                        # starts the local backend
    km index ~/path/to/your/project # build the graph

The server is local-first — nothing leaves your machine. See
https://github.com/subzone/knowledge-master for details.
