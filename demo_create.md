# Demonstração do Comando `alfred create`

## Como usar o comando create:

```bash
./alfred create
```

## Fluxo interativo da TUI:

### Passo 1: Inserção do nome do contexto
```
Create New Context
==================

Context Name:
my-new-feature█

Press Enter to continue, Esc to cancel
```

### Passo 2: Seleção de repositórios com checkboxes
```
Select repositories for 'my-new-feature'
========================================

> ☑ core (./example/core)
  ☐ ui (./example/ui)  
  ☑ app (./example/app)

↑/↓ navigate • Space select • Enter confirm • Esc back
```

### Resultado:
```
✅ Context 'my-new-feature' will be created with repositories: core, app
✅ Created context 'my-new-feature' with repositories: core, app
```

## Funcionalidades implementadas:

1. **Interface em duas etapas:**
   - Primeira etapa: Input de texto para nome do contexto
   - Segunda etapa: Seleção de repositórios com checkboxes

2. **Navegação intuitiva:**
   - Setas ↑/↓ para navegar entre repositórios
   - Espaço para marcar/desmarcar
   - Enter para confirmar
   - Esc para voltar ou cancelar

3. **Validações:**
   - Nome do contexto não pode estar vazio
   - Deve selecionar pelo menos um repositório
   - Verifica se contexto já existe

4. **Integração completa:**
   - Salva no alfred.yaml automaticamente
   - Mostra feedback visual da operação
   - Mantém consistência com o resto da CLI

## Comandos relacionados:

- `alfred list` - Lista contextos (incluindo o recém-criado)
- `alfred switch my-new-feature` - Muda para o contexto criado
- `alfred status` - Mostra status do projeto