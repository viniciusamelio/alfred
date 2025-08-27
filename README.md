# Alfred CLI

Alfred é uma ferramenta CLI para gerenciar projetos multi-repositório Flutter/Dart, permitindo trabalhar com múltiplos repositórios de forma coordenada através de contextos.

## Funcionalidades

- **Gestão de Contextos**: Crie e alterne entre diferentes contextos de trabalho
- **Worktrees/Branches**: Suporte para trabalhar com git worktrees ou branches
- **Gestão de Dependências**: Atualização automática de dependências entre repositórios
- **Interface de Commit Interativa**: Interface visual para commitar mudanças em múltiplos repositórios simultaneamente
- **Preparação para Produção**: Reverter dependências locais para referências git

## Instalação

```bash
go install github.com/viniciusamelio/alfred@latest
```

Ou compile localmente:

```bash
git clone https://github.com/viniciusamelio/alfred
cd alfred
go build -o alfred .
```

## Uso Básico

### Inicialização

```bash
# Inicializar alfred no diretório atual
alfred init

# Ou escanear automaticamente por projetos Dart/Flutter
alfred scan
```

### Gestão de Contextos

```bash
# Listar contextos disponíveis
alfred list

# Criar um novo contexto
alfred create

# Alternar para um contexto
alfred switch <nome-do-contexto>

# Alternar para o contexto principal (main/master branches)
alfred switch main

# Ver status atual
alfred status
```

### Nova Funcionalidade: Interface de Commit Interativa

A nova funcionalidade de commit permite visualizar e commitar mudanças em todos os repositórios do contexto ativo através de uma interface interativa similar ao VS Code.

```bash
# Abrir interface de commit interativa
alfred commit
```

#### Funcionalidades da Interface de Commit:

**Visualização de Arquivos:**
- 📁 Arquivos agrupados por repositório
- 🎨 Cores diferentes para cada tipo de mudança:
  - 🟠 **Modified** (M) - Arquivos modificados
  - 🟢 **Added** (A) - Arquivos adicionados
  - 🔴 **Deleted** (D) - Arquivos deletados
  - 🔵 **Renamed** (R) - Arquivos renomeados
  - ⚪ **Untracked** (??) - Arquivos não rastreados

**Controles de Navegação:**
- `↑/↓` ou `j/k` - Navegar entre arquivos
- `Space` - Selecionar/deselecionar arquivo individual
- `A` - Selecionar todos os arquivos
- `N` - Deselecionar todos os arquivos
- `D` - Alternar painel de diff (mostrar/ocultar)
- `Enter` ou `C` - Prosseguir para mensagem de commit
- `Q` - Sair

**Visualização de Diff em Tempo Real:**
- **Painel lateral automático**: As mudanças do arquivo selecionado são exibidas automaticamente ao lado da lista
- **Layout responsivo**: Interface se adapta ao tamanho do terminal
- **Cores diferenciadas**: Linhas adicionadas em verde, removidas em vermelho, contexto em azul
- **Informações do arquivo**: Status (Modified, Added, etc.) e se está staged ou unstaged

**Mensagem de Commit:**
- Editor de texto multi-linha
- `Ctrl+S` ou `Ctrl+Enter` - Confirmar commit
- `Esc` - Voltar à seleção de arquivos
- `Ctrl+C` - Cancelar

**Commit Simultâneo:**
- Uma única mensagem de commit é aplicada a todos os repositórios selecionados
- Cada repositório recebe apenas os arquivos que você selecionou dele
- Feedback detalhado sobre sucessos e erros por repositório

### Exemplo de Fluxo de Trabalho

```bash
# 1. Inicializar alfred
alfred scan

# 2. Criar um contexto para uma nova feature
alfred create
# Selecione os repositórios necessários e nomeie o contexto

# 3. Alternar para o contexto
alfred switch minha-feature

# 4. Trabalhar nos arquivos...
# (fazer mudanças nos repositórios)

# 5. Usar a interface de commit interativa
alfred commit
# - Visualizar todas as mudanças
# - Selecionar arquivos específicos
# - Ver diffs se necessário
# - Escrever mensagem de commit
# - Commitar em todos os repos simultaneamente

# 6. Push das mudanças (upstream configurado automaticamente)
alfred push

# 7. Pull de atualizações (upstream configurado automaticamente)
alfred pull

# 8. Voltar ao contexto principal quando terminar
alfred switch main
```

### Fluxo Simplificado com Upstream Automático

Com as novas funcionalidades, o fluxo de trabalho fica ainda mais simples:

```bash
# Criar nova feature branch em todos os repos do contexto
alfred switch nova-feature

# Trabalhar nos arquivos...
# (fazer mudanças)

# Commit interativo
alfred commit

# Push automático - sem se preocupar com upstream!
alfred push
# ✅ Alfred configura automaticamente origin/nova-feature para todos os repos

# Colaborar com outros devs
alfred pull
# ✅ Alfred puxa as mudanças automaticamente, configurando upstream se necessário

# Continuar trabalhando...
alfred commit
alfred push  # Agora já tem upstream configurado
```

## Configuração

O alfred usa um arquivo `.alfred/alfred.yaml` para configuração:

```yaml
repos:
  - name: core
    path: ./core
  - name: ui  
    path: ./ui
  - name: app
    path: ./app

master: app
mode: worktree
main_branch: main

contexts:
  feature-1:
    - ui
    - app
  feature-2:
    - ui
    - app
    - core
```

## Modos de Operação

### Worktree Mode (Recomendado)
- Cria git worktrees separados para cada contexto
- Repositórios não-master ficam isolados por contexto
- Repositório master permanece no diretório original

### Branch Mode
- Alterna branches diretamente nos repositórios
- Todos os repositórios ficam nos diretórios originais
- Usa git stash para preservar mudanças

### Gestão de Repositórios

```bash
# Push com configuração automática de upstream
alfred push

# Push forçando reconfiguração de upstream
alfred push -u

# Pull com configuração automática de upstream
alfred pull

# Pull usando rebase (padrão)
alfred pull -r
```

#### Nova Funcionalidade: Configuração Automática de Upstream

Os comandos `push` e `pull` agora configuram automaticamente o upstream das branches quando necessário:

**Push Automático:**
- Detecta se a branch atual tem upstream configurado
- Se não tiver, configura automaticamente para `origin/<branch-atual>`
- Faz o push normalmente
- Flag `-u` força reconfiguração mesmo se já existir upstream

**Pull Automático:**
- Verifica se existe upstream configurado
- Se não existir, tenta configurar para `origin/<branch-atual>`
- Executa o pull (com rebase por padrão)
- Elimina erros de "no tracking information"

## Comandos Disponíveis

| Comando | Descrição |
|---------|-----------|
| `alfred init` | Inicializar alfred no diretório atual |
| `alfred scan` | Escanear e configurar automaticamente |
| `alfred list` | Listar contextos disponíveis |
| `alfred switch <contexto>` | Alternar para um contexto |
| `alfred create` | Criar novo contexto |
| `alfred delete <contexto>` | Deletar contexto |
| `alfred status` | Ver status atual |
| `alfred commit` | Interface interativa de commit |
| `alfred push` | **Melhorado!** Push com upstream automático |
| `alfred pull` | **Melhorado!** Pull com upstream automático |
| `alfred prepare` | Preparar para produção |
| `alfred main-branch <branch>` | Definir branch principal |

## Contribuição

Contribuições são bem-vindas! Por favor, abra uma issue ou pull request.

## Licença

MIT License