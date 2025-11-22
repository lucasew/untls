# untls

Reexposes a TCP over TLS port as a local TCP port

Usecases:
- Minecraft server over Tailscale Funnel: Unfortunately Minecraft doesn't support TLS sockets and Tailscale Funnel require them as implementation detail for multiplexing.

## Desenvolvimento

### Criar uma Release

Para criar uma nova release, use o script `release.sh`:

```bash
# Release patch (0.0.5 -> 0.0.6)
./release.sh patch

# Release minor (0.0.5 -> 0.1.0)
./release.sh minor

# Release major (0.0.5 -> 1.0.0)
./release.sh major

# Versão específica
./release.sh 1.2.3
```

O script irá:
1. Fazer build dos binários para Windows e Linux
2. Incrementar a versão no `version.txt`
3. Criar commit e tag
4. Fazer push para o repositório
5. Criar a release no GitHub com os binários anexados
6. Opcionalmente, fazer build e push da imagem Docker

**Requisitos:**
- [GitHub CLI (gh)](https://cli.github.com/) instalado e autenticado
- Go instalado
- Docker (opcional, para publicar imagem)

