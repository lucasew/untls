#!/usr/bin/env bash

set -e

# Script para criar releases usando gh CLI
# Uso: ./release.sh [patch|minor|major|VERSION]

VERSION_TYPE="${1:-patch}"

# Cores para output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}==> Verificando dependências...${NC}"
if ! command -v gh &> /dev/null; then
    echo -e "${RED}Erro: gh CLI não está instalado${NC}"
    echo "Instale com: https://cli.github.com/"
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo -e "${RED}Erro: Go não está instalado${NC}"
    exit 1
fi

# Incrementar versão
CURRENT_VERSION=($(cat version.txt | sed 's;\.; ;g'))

case "$VERSION_TYPE" in
    patch)
        NEW_VERSION="${CURRENT_VERSION[0]}.${CURRENT_VERSION[1]}.$((${CURRENT_VERSION[2]}+1))"
    ;;
    minor)
        NEW_VERSION="${CURRENT_VERSION[0]}.$((${CURRENT_VERSION[1]}+1)).0"
    ;;
    major)
        NEW_VERSION="$((${CURRENT_VERSION[0]}+1)).0.0"
    ;;
    *)
        # Versão específica fornecida
        NEW_VERSION="$VERSION_TYPE"
    ;;
esac

echo -e "${BLUE}==> Nova versão: ${GREEN}$NEW_VERSION${NC}"

# Criar diretório de build
echo -e "${BLUE}==> Criando diretório de build...${NC}"
mkdir -p build
rm -f build/*

# Build dos binários
echo -e "${BLUE}==> Building binário Windows (amd64)...${NC}"
GOOS=windows GOARCH=amd64 go build -o build/untls-windows-amd64.exe .

echo -e "${BLUE}==> Building binário Linux (amd64)...${NC}"
GOOS=linux GOARCH=amd64 go build -o build/untls-linux-amd64 .

# Atualizar version.txt
echo -e "${BLUE}==> Atualizando version.txt...${NC}"
printf "%s" "$NEW_VERSION" > version.txt

# Commit e tag
echo -e "${BLUE}==> Criando commit e tag...${NC}"
git add version.txt
git commit -sm "bump to $NEW_VERSION" || echo "Nada para commitar"
git tag "$NEW_VERSION"

# Push
echo -e "${BLUE}==> Fazendo push...${NC}"
git push -u origin "$(git branch --show-current)"
git push origin "$NEW_VERSION"

# Criar release com gh CLI
echo -e "${BLUE}==> Criando release no GitHub...${NC}"
gh release create "$NEW_VERSION" \
    --title "Release $NEW_VERSION" \
    --generate-notes \
    build/untls-windows-amd64.exe \
    build/untls-linux-amd64

echo -e "${GREEN}==> Release $NEW_VERSION criada com sucesso!${NC}"
echo -e "${BLUE}==> Ver release: ${NC}$(gh release view $NEW_VERSION --web 2>&1 | grep -o 'https://.*' || echo '')"

# Opcional: Build e push do container Docker
read -p "Deseja fazer build e push da imagem Docker? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    REGISTRY="ghcr.io"
    IMAGE_NAME="$(git config --get remote.origin.url | sed 's/.*:\(.*\)\.git/\1/')"
    TAG="$REGISTRY/$IMAGE_NAME"

    echo -e "${BLUE}==> Building imagem Docker...${NC}"
    docker build -t "$TAG:$NEW_VERSION" .
    docker tag "$TAG:$NEW_VERSION" "$TAG:latest"

    echo -e "${BLUE}==> Fazendo push da imagem Docker...${NC}"
    docker push "$TAG:$NEW_VERSION"
    docker push "$TAG:latest"

    echo -e "${GREEN}==> Imagem Docker publicada!${NC}"
fi

echo -e "${GREEN}==> Processo completo!${NC}"
