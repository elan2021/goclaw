# GoClaw — AI Gateway Daemon

GoClaw é um daemon de alto desempenho escrito em Go que transforma LLMs em agentes autônomos acessíveis via WhatsApp e Telegram. Projetado para rodar em hardware modesto ou servidores locais.

## ✨ Funcionalidades

- **Múltiplos Canais**: Suporte nativo para WhatsApp (whatsmeow) e Telegram.
- **Multitenancy**: Cada usuário possui uma sessão isolada (`goroutine`) com seu próprio contexto e memória.
- **ReAct Planning**: Loop de raciocínio avançado que permite ao agente usar ferramentas sequencialmente.
- **Toolkit Poderoso**:
    - **Navegação Web**: Browser headless (`chromedp`) para leitura de sites e screenshots.
    - **Acesso ao Sistema**: Execução de comandos de terminal e manipulação de arquivos.
    - **Scheduler**: Lembretes e Cronjobs persistentes.
- **Memória Avançada**: 
    - **Long-term**: Notas em Markdown salvas localmente.
    - **Compressão**: Sumarização automática de históricos longos via IA.
- **Painel de Controle**: Dashboard web moderno (Dark Mode) embutido para monitoramento em tempo real.

## 🚀 Como Iniciar

### 1. Pré-requisitos
- [Go 1.22+](https://go.dev/dl/)
- Google Chrome/Chromium (para a skill de browser)

### 2. Instalação e Configuração Inicial
```bash
# Clone e build
git clone https://github.com/elan2021/goclaw.git
cd goclaw
go build -o goclaw.exe ./cmd/goclaw

# Primeira execução (Cria pastas e arquivos padrão)
./goclaw.exe
```

### 3. Configuração dos Canais
O GoClaw cria os arquivos em `~/.goclaw/` (no Windows: `%USERPROFILE%\.goclaw\`).
Edite o arquivo `config.toml`:

#### Para Telegram:
1. Obtenha um token com o [@BotFather](https://t.me/botfather).
2. Em `config.toml`, defina `enabled = true` em `[channels.telegram]` e cole o `bot_token`.

#### Para WhatsApp:
1. Em `config.toml`, defina `enabled = true` em `[channels.whatsapp]`.
2. Rode o bot e escaneie o QR Code que aparecerá no terminal usando o WhatsApp do celular (Aparelhos Conectados).

### 4. Experiência de Onboarding (Primeiro Contato)
O GoClaw possui um fluxo de configuração inteligente integrado:
1. **Envie a primeira mensagem** para o seu bot (pelo WhatsApp ou Telegram).
2. O agente detectará que é a primeira vez e irá se apresentar.
3. Ele perguntará seu **nome**, **como deve te chamar** e quais seus **objetivos**.
4. Essas informações serão salvas automaticamente no arquivo `SOUL.md`, definindo a "personalidade" e a "memória de base" do seu agente.

## 🛠️ Comandos Úteis
No terminal ou via bot:
- `ajuda`: Lista o que o agente pode fazer.
- `status`: Mostra estatísticas de uso (disponível também no Painel Web).
- `limpar histórico`: Reinicia o contexto da conversa atual.

## 🛠️ Arquitetura

O projeto segue princípios de Clean Architecture e SOLID:
- `internal/agent`: Loop ReAct e Prompt Engineering.
- `internal/gateway`: Roteador central de mensagens e sessões.
- `internal/llm`: Abstração de provedores (OpenAI, Anthropic, Ollama).
- `internal/skill`: Implementação de ferramentas plugáveis.
- `internal/web`: API REST e Dashboard embutido.

## 📝 Licença
MIT
