# Diretrizes para Agentes

- **Stack**: Desktop Wails v2 (Go) + Svelte 5 + TypeScript `strict`. O backend Go (`backend/pkg/`) roda in-process via bindings do Wails, sem HTTP. Lógica financeira e persistência residem estritamente no backend. O repositório é um workspace Go (`go.work`: `backend` + `wails-app`).
- **Proteção de Dados**: Nunca modifique, apague ou teste sobre a base real (`$XDG_DATA_HOME/rerum/rerum.sqlite` ou, por padrão, `~/.local/share/rerum/rerum.sqlite`), os recibos, os backups ou `references/`. O desenvolvimento usa banco temporário por padrão. Testes devem usar apenas dados isolados (`make test`). Para testes manuais use `RERUM_DB_PATH` apontando para uma cópia em `/tmp`.
- **Segurança do Repositório**: Proibido executar limpeza destrutiva, reescrever histórico ou mover dados locais sem ordem expressa.
- **Verificações Obrigatórias**: Valide alterações sempre com `make test`, `make lint` e `make typecheck`.
- **Launcher Local**: Compile e instale/atualize o app e o atalho do sistema via `make install-launcher`.
- **Referência**: Consulte o [`README.md`](README.md) para detalhes de arquitetura e fluxos.
