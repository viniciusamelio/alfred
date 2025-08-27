# Alfred CLI (Português)

Uma ferramenta CLI poderosa para gerenciar projetos multi-repositório Flutter/Dart com fluxos de trabalho baseados em contextos, permitindo desenvolvimento coordenado entre múltiplos repositórios através de troca inteligente de contextos e gerenciamento de dependências.

## ✨ Funcionalidades

- **🎯 Gestão de Contextos**: Crie e alterne entre diferentes contextos de desenvolvimento
- **🌿 Git Worktrees & Branches**: Suporte para git worktrees e fluxos baseados em branches
- **📦 Gestão de Dependências**: Sincronização automática de dependências entre repositórios
- **💻 Interface de Commit Interativa**: Interface visual para commitar mudanças em múltiplos repositórios
- **🚀 Pronto para Produção**: Preparação automatizada para deploy com reversão de dependências git
- **🔄 Upstream Automático**: Configuração inteligente de upstream para operações push/pull
- **🔍 Diagnósticos**: Ferramentas integradas para solução de problemas de status de repositório

## 🚀 Instalação

### Script de Instalação Seguro (Recomendado)

Execute nosso script de instalação seguro que funciona no **macOS** e **Linux** com **bash**, **zsh** e **fish**:

```bash
curl -fsSL https://raw.githubusercontent.com/viniciusamelio/alfred/main/scripts/install.sh | bash
```

## 🚀 Começando

### 1. Inicialize o Alfred no diretório do seu projeto

```bash
# Escaneie e configure automaticamente pacotes Dart/Flutter existentes
alfred scan

# Ou inicialize com configuração manual
alfred init
```

### 2. Crie e alterne para um contexto de desenvolvimento

```bash
# Crie um novo contexto
alfred create

# Alterne para um contexto
alfred switch minha-feature
```

### 3. Trabalhe com seus repositórios

```bash
# Commit interativo em todos os repositórios
alfred commit

# Push com configuração automática de upstream
alfred push

# Pull com configuração automática de upstream
alfred pull

# Diagnostique problemas de repositório
alfred diagnose
```

Para documentação completa em inglês, veja [README.md](README.md).