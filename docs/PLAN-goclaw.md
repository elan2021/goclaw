# GoClaw — Plano de Implementação Técnico

> **Sistema Operacional Local para Agentes de IA**, escrito em Go.
> Daemon leve, binário único, ponte entre mensageiros e LLMs.

---

## 1. Visão Geral

O GoClaw é um daemon local que:

- Consome **pouca memória** (compila para binário único)
- Conecta **WhatsApp** e **Telegram** a modelos de IA (OpenAI, Anthropic, Ollama)
- É **autônomo**: lê/escreve arquivos, executa comandos, navega na web headless
- Gerencia **memória de longo prazo** via Markdown (local-first)
- Embute um **painel web React/Tailwind** no próprio binário via `go:embed`

| Stack      | Tecnologia                           |
|------------|--------------------------------------|
| Linguagem  | Go 1.22+                            |
| WhatsApp   | `go.mau.fi/whatsmeow`               |
| Telegram   | `gopkg.in/telebot.v3`               |
| Web Driver | `github.com/chromedp/chromedp`       |
| Config     | `github.com/BurntSushi/toml`        |
| LLM APIs   | `net/http` + JSON nativo            |
| Frontend   | React + Tailwind (embutido via `//go:embed`) |
| Web Server | `net/http` (stdlib)                  |

---

## 2. Arquitetura de Diretórios

```
goclaw/
├── cmd/
│   └── goclaw/
│       └── main.go                 # Entrypoint: bootstrap, DI, signal handling
│
├── internal/
│   ├── config/
│   │   ├── config.go               # Struct Config + load de config.toml
│   │   └── paths.go                # Resolução de ~/.goclaw/
│   │
│   ├── channel/                    # === ADAPTERS DE MENSAGEM ===
│   │   ├── channel.go              # Interface MessageChannel
│   │   ├── whatsapp/
│   │   │   └── adapter.go          # Implementação whatsmeow
│   │   └── telegram/
│   │       └── adapter.go          # Implementação telebot.v3
│   │
│   ├── gateway/                    # === ROTEADOR CENTRAL ===
│   │   ├── gateway.go              # Gateway struct, message fan-in, session dispatch
│   │   └── session.go              # SessionManager: goroutine por usuário
│   │
│   ├── agent/                      # === CÉREBRO DO AGENTE ===
│   │   ├── agent.go                # AgentRunner: ReAct loop, tool dispatch
│   │   ├── prompt.go               # System prompt builder (SOUL.md + AGENTS.md)
│   │   └── react.go                # ReAct cycle: Thought → Action → Observation
│   │
│   ├── llm/                        # === PROVIDERS DE LLM ===
│   │   ├── provider.go             # Interface LLMProvider
│   │   ├── openai/
│   │   │   └── client.go           # OpenAI API (chat completions + tool_call)
│   │   ├── anthropic/
│   │   │   └── client.go           # Anthropic Claude API
│   │   └── ollama/
│   │       └── client.go           # Ollama local API
│   │
│   ├── skill/                      # === FERRAMENTAS / TOOLS ===
│   │   ├── registry.go             # ToolRegistry: registro e lookup
│   │   ├── skill.go                # Interface Skill + ToolDefinition
│   │   ├── terminal/
│   │   │   └── exec.go             # os/exec wrapper com timeout
│   │   ├── browser/
│   │   │   └── browse.go           # chromedp: navegação headless
│   │   ├── filesystem/
│   │   │   └── fs.go               # Leitura/escrita de arquivos
│   │   ├── memory/
│   │   │   └── recall.go           # Busca em arquivos de memória
│   │   ├── reminder/
│   │   │   └── reminder.go         # Skill set_reminder (agendamento único)
│   │   └── cronjob/
│   │       └── cronjob.go          # Skill set_cronjob (tarefas recorrentes)
│   │
│   ├── memory/                     # === MEMÓRIA LOCAL-FIRST ===
│   │   ├── store.go                # MemoryStore: CRUD em arquivos .md
│   │   ├── compressor.go           # Compressão via LLM summarization
│   │   └── history.go              # Chat history (JSONL por sessão)
│   │
│   ├── scheduler/                  # === AGENDADOR DINÂMICO ===
│   │   ├── scheduler.go            # Priority queue + dispatcher de tarefas
│   │   ├── store.go                # Persistência em ~/.goclaw/schedules.json
│   │   └── types.go                # ScheduledTask, CronRule structs
│   │
│   ├── jobs/                       # === BACKGROUND JOBS ===
│   │   ├── runner.go               # Ticker-based job runner (compressão, etc.)
│   │   └── compress_job.go         # Job de compressão de memória
│   │
│   └── web/                        # === PAINEL WEB EMBUTIDO ===
│       ├── server.go               # HTTP server + API routes
│       ├── handler.go              # Handlers REST (status, config, logs)
│       └── embed.go                # //go:embed do frontend build
│
├── web/                            # Frontend source (React + Tailwind)
│   ├── src/
│   ├── public/
│   ├── package.json
│   └── dist/                       # Build output → embutido no Go
│
├── go.mod
├── go.sum
├── Makefile                        # Build, test, run targets
└── README.md
```

### Responsabilidades por pacote

| Pacote            | Responsabilidade                                                    |
|-------------------|---------------------------------------------------------------------|
| `config`          | Carregar `config.toml`, resolver paths `~/.goclaw/`                |
| `channel`         | Abstração de plataformas de mensagem (interface + adapters)        |
| `gateway`         | Fan-in de mensagens, despacho para sessões, kill-switch            |
| `agent`           | Loop ReAct, montagem de prompts, orquestração tool→LLM            |
| `llm`             | Interface de providers de LLM, serialização de payloads            |
| `skill`           | Registro de ferramentas, execução com argumentos dinâmicos         |
| `memory`          | Leitura/escrita de `.md`, compressão de histórico, recall          |
| `scheduler`       | Agendamento dinâmico: lembretes, cronjobs, tarefas futuras do agente |
| `jobs`            | Tarefas periódicas via `time.Ticker` (compressão, cleanup)         |
| `web`             | Servidor HTTP embutido, API REST, serve frontend estático          |

---

## 3. Design de Interfaces Fundamentais

### 3.1 — Interface de Canal de Mensagem (`channel`)

```go
package channel

import "context"

// IncomingMessage representa uma mensagem recebida de qualquer plataforma.
type IncomingMessage struct {
    Platform  string // "whatsapp" | "telegram"
    UserID    string // Identificador único do usuário na plataforma
    ChatID    string // Identificador do chat/grupo
    Text      string
    Timestamp int64
    Metadata  map[string]string // Dados extras (nome, mídia, etc.)
}

// OutgoingMessage representa uma resposta para enviar ao usuário.
type OutgoingMessage struct {
    ChatID string
    Text   string
    ReplyTo string // ID da mensagem original (opcional)
}

// MessageChannel é a interface que todo adapter de plataforma deve implementar.
type MessageChannel interface {
    // Name retorna o identificador da plataforma ("whatsapp", "telegram").
    Name() string

    // Start inicializa a conexão e começa a escutar mensagens.
    // O channel `incoming` recebe mensagens indefinidamente até ctx ser cancelado.
    Start(ctx context.Context, incoming chan<- IncomingMessage) error

    // Send envia uma mensagem de volta para a plataforma.
    Send(ctx context.Context, msg OutgoingMessage) error

    // Stop encerra a conexão graciosamente.
    Stop() error
}
```

### 3.2 — Gateway e Gerenciamento de Sessão

```go
package gateway

import (
    "context"
    "sync"

    "goclaw/internal/channel"
    "goclaw/internal/agent"
)

// Gateway é o roteador central que recebe mensagens de todos os canais
// e despacha para sessões individuais por usuário.
type Gateway struct {
    mu        sync.RWMutex
    sessions  map[string]*Session         // key: userID
    incoming  chan channel.IncomingMessage  // fan-in de todos os canais
    agentCfg  agent.Config
}

// Session representa uma goroutine dedicada por usuário.
type Session struct {
    UserID   string
    Cancel   context.CancelFunc       // Kill-switch: cancela o contexto da sessão
    MsgCh    chan channel.IncomingMessage // Canal de mensagens para esta sessão
    Runner   *agent.AgentRunner
}

func NewGateway(cfg agent.Config) *Gateway {
    return &Gateway{
        sessions: make(map[string]*Session),
        incoming: make(chan channel.IncomingMessage, 256),
        agentCfg: cfg,
    }
}

// Run inicia o loop principal do gateway.
// Lê do canal fan-in e despacha para a sessão do usuário.
func (g *Gateway) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            g.shutdownAll()
            return
        case msg := <-g.incoming:
            g.dispatch(ctx, msg)
        }
    }
}

// dispatch encontra ou cria uma sessão e envia a mensagem.
func (g *Gateway) dispatch(ctx context.Context, msg channel.IncomingMessage) {
    g.mu.Lock()
    sess, exists := g.sessions[msg.UserID]
    if !exists {
        sess = g.createSession(ctx, msg.UserID)
        g.sessions[msg.UserID] = sess
    }
    g.mu.Unlock()

    // Envio non-blocking com timeout.
    select {
    case sess.MsgCh <- msg:
    default:
        // Canal cheio — sessão sobrecarregada. Log de warning.
    }
}

// createSession inicia uma goroutine independente para o usuário.
func (g *Gateway) createSession(parentCtx context.Context, userID string) *Session {
    ctx, cancel := context.WithCancel(parentCtx)
    sess := &Session{
        UserID: userID,
        Cancel: cancel,
        MsgCh:  make(chan channel.IncomingMessage, 32),
        Runner: agent.NewAgentRunner(g.agentCfg, userID),
    }

    go sess.Runner.Listen(ctx, sess.MsgCh)
    return sess
}

// KillSession cancela a goroutine de um usuário (kill-switch).
func (g *Gateway) KillSession(userID string) {
    g.mu.Lock()
    defer g.mu.Unlock()
    if sess, ok := g.sessions[userID]; ok {
        sess.Cancel()
        delete(g.sessions, userID)
    }
}

// IncomingChannel retorna o canal fan-in para os adapters publicarem mensagens.
func (g *Gateway) IncomingChannel() chan<- channel.IncomingMessage {
    return g.incoming
}
```

### 3.3 — Interface de Skills/Tools

```go
package skill

import "context"

// ToolDefinition descreve uma ferramenta para o LLM (JSON Schema compatível).
type ToolDefinition struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]ParamSchema `json:"parameters"`
    Required    []string               `json:"required"`
}

// ParamSchema descreve um parâmetro de ferramenta.
type ParamSchema struct {
    Type        string   `json:"type"`
    Description string   `json:"description"`
    Enum        []string `json:"enum,omitempty"`
}

// ToolCall representa a requisição do LLM para executar uma ferramenta.
type ToolCall struct {
    Name      string            `json:"name"`
    Arguments map[string]string `json:"arguments"`
}

// ToolResult é o retorno da execução de uma ferramenta.
type ToolResult struct {
    Output string `json:"output"`
    Error  string `json:"error,omitempty"`
}

// Skill é a interface que toda ferramenta deve implementar.
type Skill interface {
    // Definition retorna a spec da ferramenta para o LLM.
    Definition() ToolDefinition

    // Execute roda a ferramenta com os argumentos fornecidos.
    // O context deve ser respeitado para timeout e cancelamento.
    Execute(ctx context.Context, args map[string]string) (ToolResult, error)
}

// Registry gerencia todas as ferramentas disponíveis.
type Registry struct {
    tools map[string]Skill
}

func NewRegistry() *Registry {
    return &Registry{tools: make(map[string]Skill)}
}

func (r *Registry) Register(s Skill) {
    r.tools[s.Definition().Name] = s
}

func (r *Registry) Get(name string) (Skill, bool) {
    s, ok := r.tools[name]
    return s, ok
}

// Definitions retorna todas as specs para enviar ao LLM.
func (r *Registry) Definitions() []ToolDefinition {
    defs := make([]ToolDefinition, 0, len(r.tools))
    for _, s := range r.tools {
        defs = append(defs, s.Definition())
    }
    return defs
}
```

### 3.4 — Scheduler Dinâmico (Lembretes + Cronjobs)

```go
package scheduler

import (
    "context"
    "encoding/json"
    "log/slog"
    "os"
    "sync"
    "time"

    "goclaw/internal/channel"
)

// ScheduledTask representa uma tarefa agendada (lembrete ou cronjob).
type ScheduledTask struct {
    ID        string    `json:"id"`
    UserID    string    `json:"user_id"`
    ChatID    string    `json:"chat_id"`
    Platform  string    `json:"platform"`
    Message   string    `json:"message"`     // Mensagem para enviar OU prompt para o agente executar
    RunAt     time.Time `json:"run_at"`       // Para lembretes: horário exato
    CronExpr  string    `json:"cron_expr"`    // Para cronjobs: "0 9 * * *" (todo dia às 9h)
    IsAgentTask bool    `json:"is_agent_task"` // true = agente executa prompt; false = apenas lembra
    CreatedAt time.Time `json:"created_at"`
}

// Dispatcher é a callback que o scheduler chama quando uma tarefa dispara.
type Dispatcher func(ctx context.Context, task ScheduledTask) error

// Scheduler gerencia tarefas agendadas com persistência em disco.
type Scheduler struct {
    mu       sync.RWMutex
    tasks    []ScheduledTask
    filePath string        // ~/.goclaw/schedules.json
    dispatch Dispatcher
}

func New(filePath string, dispatch Dispatcher) (*Scheduler, error) {
    s := &Scheduler{
        filePath: filePath,
        dispatch: dispatch,
    }
    if err := s.load(); err != nil && !os.IsNotExist(err) {
        return nil, err
    }
    return s, nil
}

// Add adiciona uma nova tarefa agendada.
func (s *Scheduler) Add(task ScheduledTask) error {
    s.mu.Lock()
    s.tasks = append(s.tasks, task)
    s.mu.Unlock()
    return s.persist()
}

// Run inicia o loop do scheduler. Verifica a cada 30s se há tarefas para disparar.
func (s *Scheduler) Run(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    slog.Info("scheduler running", "tasks_loaded", len(s.tasks))

    for {
        select {
        case <-ctx.Done():
            return
        case now := <-ticker.C:
            s.checkAndDispatch(ctx, now)
        }
    }
}

// checkAndDispatch verifica tarefas pendentes e dispara as que estão no horário.
func (s *Scheduler) checkAndDispatch(ctx context.Context, now time.Time) {
    s.mu.Lock()
    defer s.mu.Unlock()

    remaining := make([]ScheduledTask, 0, len(s.tasks))
    for _, task := range s.tasks {
        if task.CronExpr == "" && now.After(task.RunAt) {
            // Lembrete único: dispara e remove.
            go s.dispatch(ctx, task)
            continue
        }
        if task.CronExpr != "" && s.cronMatches(task.CronExpr, now) {
            // Cronjob: dispara mas mantém na lista.
            go s.dispatch(ctx, task)
        }
        remaining = append(remaining, task)
    }
    s.tasks = remaining
}

// persist salva tarefas em disco (sobrevive a restarts).
func (s *Scheduler) persist() error {
    data, err := json.MarshalIndent(s.tasks, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(s.filePath, data, 0644)
}

// load carrega tarefas do disco ao iniciar.
func (s *Scheduler) load() error {
    data, err := os.ReadFile(s.filePath)
    if err != nil {
        return err
    }
    return json.Unmarshal(data, &s.tasks)
}
```

---

## 4. Fluxo de Execução — ReAct Loop em Go

```
┌──────────────────────────────────────────────────┐
│              REACT LOOP (por sessão)              │
│                                                  │
│  User Msg ──► Build Prompt ──► LLM Call          │
│                                    │             │
│                      ┌─────────────┼──────┐      │
│                      ▼             ▼      ▼      │
│                   [text]      [tool_call]  [done] │
│                      │             │        │    │
│                      ▼             ▼        ▼    │
│               Respond User   Execute Skill  END  │
│                                    │             │
│                                    ▼             │
│                             Observation          │
│                                    │             │
│                                    ▼             │
│                          Append to History       │
│                          ──► LLM Call (loop) ◄── │
└──────────────────────────────────────────────────┘
```

### Pseudo-código Go do ReAct Loop

```go
package agent

import (
    "context"
    "fmt"
    "time"

    "goclaw/internal/channel"
    "goclaw/internal/llm"
    "goclaw/internal/skill"
    "goclaw/internal/memory"
)

const maxReActSteps = 10

type AgentRunner struct {
    provider  llm.Provider
    registry  *skill.Registry
    memory    *memory.Store
    history   *memory.History
    replyCh   chan<- channel.OutgoingMessage
    userID    string
}

// Listen é a goroutine principal da sessão do usuário.
func (a *AgentRunner) Listen(ctx context.Context, msgCh <-chan channel.IncomingMessage) {
    for {
        select {
        case <-ctx.Done():
            return
        case msg := <-msgCh:
            a.handleMessage(ctx, msg)
        }
    }
}

// handleMessage executa o ReAct loop para uma mensagem.
func (a *AgentRunner) handleMessage(ctx context.Context, msg channel.IncomingMessage) {
    // Timeout de segurança: máximo 2 minutos por interação.
    ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
    defer cancel()

    // Montar prompt com SOUL.md + histórico + mensagem do usuário.
    prompt := a.buildPrompt(msg)
    a.history.Append(msg.UserID, "user", msg.Text)

    for step := 0; step < maxReActSteps; step++ {
        // Chamar o LLM com as tool definitions.
        resp, err := a.provider.ChatCompletion(ctx, llm.Request{
            Messages: prompt,
            Tools:    a.registry.Definitions(),
        })
        if err != nil {
            a.reply(msg.ChatID, fmt.Sprintf("Erro interno: %v", err))
            return
        }

        // Caso 1: LLM retornou texto puro → responder ao usuário.
        if resp.Content != "" && len(resp.ToolCalls) == 0 {
            a.history.Append(msg.UserID, "assistant", resp.Content)
            a.reply(msg.ChatID, resp.Content)
            return
        }

        // Caso 2: LLM pediu para executar ferramenta(s).
        for _, tc := range resp.ToolCalls {
            tool, ok := a.registry.Get(tc.Name)
            if !ok {
                observation := fmt.Sprintf("Tool '%s' não encontrada.", tc.Name)
                prompt = append(prompt, llm.Msg("tool", observation))
                continue
            }

            result, err := tool.Execute(ctx, tc.Arguments)
            if err != nil {
                observation := fmt.Sprintf("Erro ao executar '%s': %v", tc.Name, err)
                prompt = append(prompt, llm.Msg("tool", observation))
                continue
            }

            // Observation → vai de volta pro LLM no próximo step.
            prompt = append(prompt, llm.Msg("tool", result.Output))
        }
    }

    // Safety: se chegou aqui, o agente entrou em loop.
    a.reply(msg.ChatID, "⚠️ Limite de passos atingido. Operação interrompida.")
}
```

---

## 5. Gerenciamento de Memória e Embed

### 5.1 — Compressão via Ticker

```go
package jobs

import (
    "context"
    "log/slog"
    "time"

    "goclaw/internal/memory"
    "goclaw/internal/llm"
)

type Scheduler struct {
    interval   time.Duration
    compressor *memory.Compressor
}

func NewScheduler(interval time.Duration, provider llm.Provider, store *memory.Store) *Scheduler {
    return &Scheduler{
        interval:   interval,
        compressor: memory.NewCompressor(provider, store),
    }
}

func (s *Scheduler) Start(ctx context.Context) {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()

    slog.Info("scheduler started", "interval", s.interval)

    for {
        select {
        case <-ctx.Done():
            slog.Info("scheduler stopped")
            return
        case <-ticker.C:
            if err := s.compressor.CompressOldHistories(ctx); err != nil {
                slog.Error("compression failed", "error", err)
            }
        }
    }
}
```

### 5.2 — Frontend Embutido via `go:embed`

```go
package web

import (
    "embed"
    "io/fs"
    "net/http"
    "log/slog"
)

//go:embed all:dist
var frontendFS embed.FS

func NewServer(addr string) *http.Server {
    mux := http.NewServeMux()

    // API routes.
    mux.HandleFunc("GET /api/status", handleStatus)
    mux.HandleFunc("GET /api/sessions", handleSessions)
    mux.HandleFunc("POST /api/sessions/{id}/kill", handleKillSession)

    // Frontend estático embutido.
    distFS, _ := fs.Sub(frontendFS, "dist")
    fileServer := http.FileServer(http.FS(distFS))
    mux.Handle("/", fileServer)

    slog.Info("web panel available", "addr", addr)

    return &http.Server{
        Addr:    addr,
        Handler: mux,
    }
}
```

---

## 6. Estado e Memória (Local-First)

### Estrutura de `~/.goclaw/`

```
~/.goclaw/
├── config.toml          # Configuração global (API keys, portas, etc.)
├── SOUL.md              # Personalidade do agente
├── AGENTS.md            # Instruções/regras adicionais
├── schedules.json       # Tarefas agendadas (lembretes + cronjobs)
├── memory/
│   ├── notes/           # Notas de longo prazo (Markdown)
│   └── summaries/       # Resumos comprimidos de conversas
└── history/
    ├── user_5511xxx.jsonl   # Histórico bruto por usuário
    └── user_9988xxx.jsonl
```

### `config.toml` (exemplo)

```toml
[server]
port = 18789

[llm]
default_provider = "openai"

[llm.openai]
api_key = "sk-..."
model = "gpt-4o"

[llm.anthropic]
api_key = "sk-ant-..."
model = "claude-sonnet-4-20250514"

[llm.ollama]
base_url = "http://localhost:11434"
model = "llama3"

[channels.whatsapp]
enabled = true

[channels.telegram]
enabled = true
bot_token = "123456:ABC..."

[jobs]
compress_interval_hours = 6

[agent]
max_react_steps = 10
timeout_seconds = 120
```

---

## 7. Roadmap de Implementação (Fases 1–4)

### Fase 1 — Fundação (MVP Core)

> **Objetivo**: Daemon rodando, recebendo mensagem de 1 canal, respondendo via 1 LLM.

| #  | Tarefa                                                         | Pacote        | Verificação                                             |
|----|----------------------------------------------------------------|---------------|---------------------------------------------------------|
| 1  | Criar `go.mod` (`module goclaw`) e estrutura de diretórios    | root          | `go build ./...` compila sem erros                      |
| 2  | Implementar `config.Load()` (parse `config.toml` + paths)     | `config`      | Test unitário: carrega TOML de exemplo e valida campos  |
| 3  | Implementar `LLMProvider` interface + client **OpenAI**       | `llm/openai`  | Test: envia prompt simples e recebe resposta             |
| 4  | Implementar `ToolRegistry` + 1 skill: `filesystem.ReadFile`   | `skill`       | Test: registra skill, busca por nome, executa            |
| 5  | Implementar `AgentRunner` com ReAct loop básico (sem tools)   | `agent`       | Test: prompt → resposta texto do LLM                     |
| 6  | Implementar adapter **Telegram** (mais simples para MVP)      | `channel/telegram` | Bot responde "echo" no Telegram                     |
| 7  | Implementar `Gateway.Run()` + `Session` por usuário           | `gateway`     | 2 usuários simultâneos, cada um com sessão independente  |
| 8  | Wiring em `cmd/goclaw/main.go` (DI, signal handling)          | `cmd`         | `go run ./cmd/goclaw` inicia, SIGINT faz shutdown limpo  |

**Marco**: Bot Telegram respondendo perguntas via OpenAI. ✅

---

### Fase 2 — Autonomia (Tools + WhatsApp)

> **Objetivo**: Agente usando ferramentas e conectado a ambas as plataformas.

| #  | Tarefa                                                         | Pacote              | Verificação                                           |
|----|----------------------------------------------------------------|---------------------|-------------------------------------------------------|
| 9  | Skill `terminal.Exec` (os/exec com timeout via context)       | `skill/terminal`    | Test: executa `echo hello`, respeita timeout           |
| 10 | Skill `browser.Navigate` (chromedp headless)                  | `skill/browser`     | Test: abre URL, extrai título da página                |
| 11 | Skill `filesystem.WriteFile`                                  | `skill/filesystem`  | Test: escreve arquivo temp, verifica conteúdo          |
| 12 | Skill `memory.Recall` (busca em arquivos .md)                 | `skill/memory`      | Test: busca keyword em nota de teste                   |
| 13 | Integrar tool_call no ReAct loop (parse de ToolCalls do LLM)  | `agent`             | Test E2E: "liste os arquivos do diretório" → usa exec  |
| 14 | Implementar adapter **WhatsApp** (whatsmeow)                  | `channel/whatsapp`  | QR code escaneado, mensagem recebida e respondida      |
| 15 | Implementar `LLMProvider` para **Anthropic**                  | `llm/anthropic`     | Test: mesma interface, resposta de Claude               |
| 16 | Implementar `LLMProvider` para **Ollama**                     | `llm/ollama`        | Test: resposta de modelo local                          |
| 17 | Implementar `scheduler.Scheduler` (priority queue + persist)  | `scheduler`         | Test: agenda tarefa, salva em JSON, recarrega ao reiniciar |
| 18 | Skill `set_reminder` (LLM agenda lembrete único)              | `skill/reminder`    | Test: "me lembre em 1min" → recebe msg após 1min       |
| 19 | Skill `set_cronjob` (LLM agenda tarefa recorrente)            | `skill/cronjob`     | Test: cronjob criado, persiste no JSON                  |
| 20 | Dispatcher: scheduler envia msgs proativas via canal correto  | `scheduler`+`gateway` | Test: lembrete dispara → msg chega no Telegram/WhatsApp |

**Marco**: Agente autônomo executando comandos e navegando web. ✅

---

### Fase 3 — Memória e Resiliência

> **Objetivo**: Agente com memória persistente e sistema robusto.

| #  | Tarefa                                                         | Pacote        | Verificação                                              |
|----|----------------------------------------------------------------|---------------|----------------------------------------------------------|
| 21 | `memory.Store` (CRUD de notas .md)                            | `memory`      | Test: cria, lê, atualiza, deleta nota                    |
| 22 | `memory.History` (append JSONL por sessão)                    | `memory`      | Test: append 100 mensagens, relê em ordem                |
| 23 | `memory.Compressor` (resume histórico via LLM)               | `memory`      | Test: histórico de 50 msgs → resumo de 1 parágrafo      |
| 24 | `jobs.Runner` (Ticker para compressão periódica)              | `jobs`        | Test: ticker dispara após intervalo configurado          |
| 25 | Prompt builder: carrega `SOUL.md` + `AGENTS.md` + memória    | `agent`       | Test: prompt montado inclui persona + instruções         |
| 26 | Kill-switch: `Gateway.KillSession()` cancela contexto        | `gateway`     | Test: sessão em loop é cancelada em < 1s                 |
| 27 | Graceful shutdown: `SIGINT`/`SIGTERM` → close channels → wait | `cmd`        | Test: shutdown completa em < 5s com sessões ativas       |
| 28 | Rate limiting por sessão (max msgs/min)                       | `gateway`     | Test: excesso de mensagens retorna aviso                 |

**Marco**: Agente com memória persistente e proteção contra loops. ✅

---

### Fase 4 — Painel Web e Polish

> **Objetivo**: Interface de administração + build final + documentação.

| #  | Tarefa                                                         | Pacote        | Verificação                                              |
|----|----------------------------------------------------------------|---------------|----------------------------------------------------------|
| 29 | Scaffold frontend React + Tailwind em `web/`                  | `web/`        | `npm run build` gera `dist/`                             |
| 30 | Dashboard: status do daemon, sessões, **agendamentos**        | `web/`        | Painel mostra dados reais via API (incl. lembretes)      |
| 31 | API REST: `/api/status`, `/api/sessions/{id}/kill`, `/api/schedules` | `internal/web` | `curl localhost:18789/api/schedules` retorna JSON  |
| 32 | `//go:embed` do `web/dist` no binário Go                      | `internal/web` | `go build` inclui frontend, acessível no browser        |
| 33 | `Makefile`: targets `build`, `dev`, `test`, `clean`           | root          | `make build` gera binário único funcional                |
| 34 | Testes de integração: mensagem → tool → resposta              | `*_test.go`   | `go test ./...` passa 100%                               |
| 35 | README.md: instalação, configuração, uso                      | root          | Documentação clara e completa                            |
| 36 | Build multi-plataforma (GOOS/GOARCH)                          | `Makefile`    | Binários para Linux, macOS, Windows                      |

**Marco**: Binário único com painel web embutido. Produto entregável. ✅

---

## 8. Critérios de Sucesso

| Critério                                           | Métrica                              |
|----------------------------------------------------|--------------------------------------|
| Binário único, sem dependências runtime             | `file goclaw` → ELF/PE executable   |
| Uso de memória em idle                              | < 50MB RSS                          |
| Latência de resposta (sem tool call)                | < 3s (depende do LLM)               |
| Sessões simultâneas                                 | ≥ 50 sem degradação                 |
| Kill-switch funcional                               | Sessão cancelada em < 1s            |
| Compressão de memória automática                    | Ticker dispara conforme configurado |
| Lembretes disparam no horário                       | Desvio máximo de ±30s               |
| Cronjobs sobrevivem a restart                       | `schedules.json` recarregado        |
| Frontend acessível                                  | `http://localhost:18789` carrega     |
| Testes                                              | `go test ./...` → 0 failures        |

---

## 9. Riscos e Mitigações

| Risco                                    | Impacto | Mitigação                                                  |
|------------------------------------------|---------|------------------------------------------------------------|
| whatsmeow instável / ban do WhatsApp     | Alto    | Isolamento via interface; fallback apenas Telegram          |
| LLM gerando tool_calls malformados       | Médio   | Validação rígida + retry com prompt de correção             |
| Goroutine leak em sessões órfãs          | Alto    | Context com timeout + session reaper periódico              |
| chromedp travando em páginas pesadas     | Médio   | Context timeout de 30s + pool limitado de browsers          |
| Histórico crescendo sem limite           | Baixo   | Compressor automático + limite de linhas por arquivo JSONL  |
| Lembretes perdidos após crash            | Médio   | Persistência em `schedules.json` + reload no boot           |
| Cronjob executando tarefa destrutiva     | Alto    | Sandboxing de skills + confirmação para ações perigosas     |

---

## 10. Verificação Final (Fase X)

- [ ] `go vet ./...` — sem warnings
- [ ] `go test ./...` — 100% pass
- [ ] `golangci-lint run` — sem issues críticos
- [ ] Build multi-plataforma funcional
- [ ] Teste E2E: mensagem Telegram → tool call → resposta
- [ ] Teste E2E: mensagem WhatsApp → resposta com memória
- [ ] Painel web funcional em `localhost:18789`
- [ ] Graceful shutdown testado
- [ ] `README.md` completo
