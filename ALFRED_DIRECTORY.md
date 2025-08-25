# Diretório .alfred

O Alfred CLI agora utiliza um diretório `.alfred` para gerenciar todas as configurações e estado da ferramenta, mantendo o projeto principal organizado e evitando conflitos.

## 📁 Estrutura do Diretório

```
projeto/
├── .alfred/
│   ├── alfred.yaml       # Configuração principal (repos e contexts)
│   └── current-context   # Contexto ativo atual
├── .gitignore           # Automaticamente atualizado para ignorar .alfred/
└── ... (seus arquivos do projeto)
```

## 🚀 Como Funciona

### Inicialização
```bash
alfred init
```

Quando você executa `alfred init`, a ferramenta:

1. ✅ **Cria o diretório `.alfred`**
2. ✅ **Gera `.alfred/alfred.yaml`** com configuração exemplo
3. ✅ **Atualiza `.gitignore`** para ignorar o diretório `.alfred/`
4. ✅ **Previne inicialização duplicada**

### Gerenciamento de Estado

- **`.alfred/alfred.yaml`**: Configuração principal com repositórios e contextos
- **`.alfred/current-context`**: Armazena o contexto ativo atual
- **Futuros arquivos de estado**: Cache, histórico, etc.

## 🔒 Por que .gitignore?

O diretório `.alfred/` é automaticamente adicionado ao `.gitignore` porque contém:

- **Estado local** do desenvolvedor (contexto atual)
- **Configurações pessoais** que podem variar entre membros da equipe
- **Arquivos temporários** e cache da ferramenta

Isso evita conflitos de merge e mantém cada desenvolvedor com seu próprio estado local.

## 📝 Conteúdo Adicionado ao .gitignore

```gitignore
# Alfred CLI state and configuration
.alfred/
```

## ✨ Vantagens

1. **Organização**: Todos os arquivos do Alfred em um local
2. **Não-intrusivo**: Não polui a raiz do projeto
3. **Segurança**: Estado local não vaza para o repositório
4. **Escalabilidade**: Fácil adicionar novas funcionalidades

## 🔄 Migração

Se você tinha uma instalação antiga do Alfred:

1. Os arquivos antigos (`alfred.yaml`, `.alfred-context`) não são mais utilizados
2. Execute `alfred init` em um diretório limpo
3. Copie suas configurações antigas para `.alfred/alfred.yaml`

## 🛠️ Comandos Afetados

Todos os comandos agora utilizam a nova estrutura automaticamente:

- `alfred init` - Cria `.alfred/` e atualiza `.gitignore`
- `alfred list` - Lê de `.alfred/alfred.yaml`
- `alfred switch` - Salva estado em `.alfred/current-context`
- `alfred status` - Lê estado de `.alfred/current-context`
- `alfred create` - Salva novos contextos em `.alfred/alfred.yaml`