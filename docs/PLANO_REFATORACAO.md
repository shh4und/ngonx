# Plano de Implementação e Refatoração — ngonx

> Projeto didático. O foco é aprender HTTP implementando-o, não produzir software de produção.
> As prioridades abaixo refletem o que mais agrega aprendizado vs. esforço.

---

## Sumário

1. [Bugs — O código não funciona como esperado](#1-bugs)
2. [Features HTTP importantes que faltam](#2-features)
3. [Refatorações que melhoram a clareza](#3-refatoracoes)
4. [Coisas que estão boas e não precisam mudar](#4-coisas-boas)
5. [O que fica de fora propositalmente](#5-fora-de-escopo)

---

## 1. Bugs

### 1.1 `RequestFromReader` sai do loop antes de processar o body

**Arquivo:** `internal/request/request.go:221`

```go
for !request.bodyDone() && !request.bodyError() &&
    !request.headersDone() && !request.headersError() && !request.reqLineError() {
```

Quando o parser termina os headers (`StateHeadersDone`), a condição `!request.headersDone()` passa a ser `false` e o loop **encerra imediatamente**, sem dar chance ao parser de processar `StateBodyInitialized` e `StateBodyDone`. O body inteiro é ignorado.

**Consequência:** requisições com `Content-Length` > 0 perdem o body.

**Correção:** a condição de parada deve ser apenas `bodyDone()` ou `bodyError()` (ou erro fatal). `headersDone()` não deve interromper o loop.

```go
for !request.bodyDone() && !request.bodyError() && !request.reqLineError() {
```

- [ ] Corrigir condição do loop em `RequestFromReader`

---

### 1.2 Handler error nunca é enviado ao cliente

**Arquivo:** `internal/server/server.go:83-99`

```go
func (s *Server) handle(conn net.Conn) {
    defer conn.Close()
    // ...
    buf := new(bytes.Buffer)
    handlerErr := s.handler(buf, req)
    if handlerErr != nil {
        writeHandlerError(buf, handlerErr)  // escreve no buffer
        return                               // mas nunca escreve o buffer no conn!
    }
    // ... escreve buf no conn (só chega aqui se handlerErr == nil)
}
```

Quando um handler retorna `*HandlerError`, a função `writeHandlerError` escreve a resposta de erro no `buf`, mas o `return` logo em seguida faz com que o buffer nunca seja copiado para o `conn`. O cliente recebe **conexão fechada sem resposta HTTP** — nem status line, nada.

**Correção:** após `writeHandlerError`, o código precisa escrever o `buf` no `conn`, igual faz no caminho feliz.

```go
if handlerErr != nil {
    writeHandlerError(buf, handlerErr)
    body := buf.Bytes()
    conn.Write(body)  // ← linha faltando
    return
}
```

> Nota: `writeHandlerError` já escreve status line e headers no buffer, então faltou só o flush.

- [ ] Enviar buffer de erro para o `conn` antes do `return`

---

### 1.3 Rota sem match retorna `nil` (resposta fantasma)

**Arquivo:** `cmd/httpserver/main.go:43-44`

```go
    // final do switch sem case correspondente
    return nil
}

// Quem chamou (server.go):
handlerErr := s.handler(buf, req)
if handlerErr != nil {
    writeHandlerError(buf, handlerErr)
    return
}
// Se handlerErr == nil, escreve 200 OK com body vazio
```

Para uma rota não encontrada (ex: `GET /nao-existe`), o `fakeRouterHandler` retorna `nil`. O servidor entende que foi sucesso e escreve `200 OK` com body vazio — comportamento incorreto.

**Correção:** retornar `404 Not Found` no fallthrough:

```go
    // fallthrough — nenhum match
    return &server.HandlerError{
        StatusCode: response.StatusNotFound,
        Message:    "Not Found\n",
    }
```

- [ ] Retornar 404 no fallthrough do `fakeRouterHandler`

---

### 1.4 `fileHandler` vulnerável a path traversal

**Arquivo:** `cmd/httpserver/main.go:57-59`

```go
filePath := filepath.Join(".", req.RequestLine.RequestURI)
```

`GET /../../etc/passwd` resolve para `etc/passwd` — o `filepath.Join` limpa o `.` mas não impede saída do diretório `static/`.

**Correção:** usar `filepath.Clean` e verificar se o prefixo é o diretório esperado:

```go
func fileHandler(w io.Writer, req *request.Request) *server.HandlerError {
    // Garantir que o path fique dentro de ./static/
    baseDir := "static"
    cleanPath := filepath.Clean(req.RequestLine.RequestURI)
    if !strings.HasPrefix(cleanPath, "/"+baseDir) {
        return &server.HandlerError{
            StatusCode: response.StatusForbidden,
            Message:    "Forbidden\n",
        }
    }
    filePath := filepath.Join(baseDir, strings.TrimPrefix(cleanPath, "/"+baseDir+"/"))
    // ...
}
```

Ou, mais simples para um projeto didático: extrair o path relativo e juntar com `static/`:

```go
safePath := filepath.Base(req.RequestLine.RequestURI)
filePath := filepath.Join("static", safePath)
```

- [ ] Adicionar sanitização de path no `fileHandler`

---

## 2. Features HTTP Importantes

### 2.1 `Transfer-Encoding: chunked` (parsing de request)

**Arquivo:** `internal/request/request.go`

**O que precisa ser feito:**

- [ ] Detectar `Transfer-Encoding` no `StateHeadersDone` (além de `Content-Length`)
- [ ] Implementar máquina de estados `ChunkState { Size, Data, CRLF, Trailer, Done }`
- [ ] Método `parseChunked(data []byte) (int, error)` que consome chunks hex
- [ ] Suportar chunk extensions (`1E;extension=value\r\n`)
- [ ] Suportar trailer headers
- [ ] Adicionar campo `Body []byte` que usa `append` em vez de alocar tamanho fixo
- [ ] Tratar `0\r\n\r\n` como término do stream
- [ ] Erro específico para chunked incompleto (`ErrIncompleteChunkedBody`)

**Detalhamento da lógica:**

```go
case StateHeadersDone:
    transferEncoding, teExists := r.Headers["transfer-encoding"]
    if teExists && strings.Contains(transferEncoding, "chunked") {
        r.chunkedBody = true
        r.ParserState = StateBodyInitialized
        r.Body = make([]byte, 0) // slice dinâmico
        continue
    }
    // lógica atual com Content-Length...
```

- [ ] Implementar parsing chunked

---

### 2.2 `Transfer-Encoding: chunked` (escrita de resposta)

**Arquivo:** `internal/response/response.go`

**O que precisa ser feito:**

- [ ] Nova função `WriteChunkedHeaders(writer, contentType)` — status line + headers **sem** `content-length`, com `transfer-encoding: chunked`
- [ ] Nova função `WriteChunk(writer, data)` — `hexSize\r\ndata\r\n`
- [ ] Nova função `WriteChunkedTrailer(writer)` — `0\r\n\r\n`
- [ ] Opcional: `WriteChunkedTrailer` com headers opcionais
- [ ] Atualizar `server.go` para suportar handlers que fazem streaming chunked

**Mudanças no server.go:**

- [ ] `server.handle()` deve detectar se o handler escreveu headers HTTP diretamente no writer, ou se deve escrever os headers padrão. Alternativa: handler chunked escreve no `conn` diretamente (não pasa por buffer).
- [ ] Ou: criar um `ChunkedResponseWriter` que implementa `io.Writer` e faz chunked automaticamente.

- [ ] Implementar escrita chunked

---

### 2.3 Suporte a HTTP/1.0

**Arquivo:** `internal/request/request.go:205`

```go
if string(httpParts[1]) != "1.1" {
    return n, nil, ErrUnsuportedHTTPVersion
}
```

O servidor rejeita qualquer versão que não seja HTTP/1.1. HTTP/1.0 ainda é usado por alguns clientes (ex: proxies antigos, alguns dispositivos IoT). A diferença prática:

| Versão | Conexão default | Host header |
|---|---|---|
| HTTP/1.0 | Close (não persiste) | Opcional |
| HTTP/1.1 | Keep-Alive (persiste) | Obrigatório |

**Mudança:** aceitar HTTP/1.0 e armazenar a versão. A lógica de keep-alive vs close pode vir depois.

- [ ] Aceitar HTTP/1.0 no parser (remover a restrição)
- [ ] Teste para requisição HTTP/1.0 válida

---

### 2.4 Validação de método HTTP (opcional)

**Arquivo:** `internal/request/request.go:27-36` (comentado)

Há um `MethodsSet` comentado. Para um parser didático, validar o método contra os métodos registrados na RFC 9110 §9.1 pode pegar casos como `GET` vs `get` (já feito) e também rejeitar métodos nonsense como `FOOBAR`.

- [ ] Reativar o `MethodsSet` ou similar, com os 8 métodos padrão (GET, HEAD, POST, PUT, DELETE, CONNECT, OPTIONS, TRACE)
- [ ] Adicionar PATCH (RFC 5789)

---

### 2.5 Router com suporte a métodos HTTP e 404 real

**Arquivo:** `cmd/httpserver/main.go:32-44`

O router atual só olha a URI, ignora o método HTTP. Exemplo: `DELETE /` cai no `defaultHandler` e retorna 200.

**Router mínimo — switch por método + path:**

```go
type route struct {
    method  string
    pattern string
    handler server.Handler
}

func methodAwareRouter(w io.Writer, req *request.Request) *server.HandlerError {
    switch {
    case req.Method == "GET" && req.RequestURI == "/":
        return defaultHandler(w, req)
    case req.Method == "GET" && req.RequestURI == "/static/landing.html":
        return fileHandler(w, req)
    default:
        return &server.HandlerError{
            StatusCode: response.StatusNotFound,
            Message:    "Not Found\n",
        }
    }
}
```

- [ ] Adicionar verificação de método HTTP no router
- [ ] Retornar 404/405 conforme o caso (405 Method Not Allowed se a rota existe mas o método não)

---

### 2.6 Keep-Alive (conexão persistente)

**Atualmente:** toda resposta tem `Connection: close`. O servidor fecha o socket depois de escrever a resposta.

**HTTP/1.1 assume keep-alive por default** (RFC 9112 §9.3). Para implementar:

- [ ] Se `Connection` do request não for `close`, manter a conexão aberta
- [ ] No `handle()`, ler múltiplas requisições da mesma conexão até `Connection: close` ou timeout
- [ ] `Connection: keep-alive` na resposta
- [ ] Adicionar timeout de idle (simples: `conn.SetReadDeadline`)

Isso também força o parser a funcionar corretamente com múltiplas mensagens no mesmo stream TCP — excelente exercício didático.

- [ ] Implementar keep-alive no `handle()`

---

## 3. Refatorações

### 3.1 Limpeza de código morto em `request.go`

| Linha | Problema |
|---|---|
| `isRequestLineValid` (L15) | Regex declarada, nunca usada |
| `ErrMalformedMsg` (L19) | Definida, usada em zero lugares |
| `ErrMalformedMsg` (L25) | Segunda declaração comentada |
| `MethodsSet` (L27-36) | Bloco comentado |
| `import "log/slog"` (L8) | Comentado |

- [ ] Remover código morto (ou reativar o que for útil)

---

### 3.2 Assinatura de `GetDefaultHeaders` e `WriteHeaders` — muitos `*string`

**Arquivo:** `internal/response/response.go:53-89`

A função recebe 7 parâmetros opcionais como `*string`. Isso é verboso e fácil de errar a ordem.

**Sugestão:** substituir por um struct de configuração:

```go
type HeaderOptions struct {
    ContentType     string
    Connection      string
    Date            string
    ContentEncoding string
    CacheControl    string
    Server          string
}
```

E usar ponteiro nulo ou valores zero para defaults:

```go
func GetDefaultHeaders(contentLen int, opts *HeaderOptions) headers.Headers {
    h := headers.NewHeaders()
    h["content-length"] = strconv.Itoa(contentLen)
    h["content-type"] = "text/html; charset=utf-8"
    h["connection"] = "close"
    // ...
    if opts != nil {
        if opts.ContentType != "" {
            h["content-type"] = opts.ContentType
        }
        // ...
    }
}
```

Ou usar o padrão functional options (mais idiomático em Go):

```go
type HeaderOption func(*headerConfig)
func WithContentType(ct string) HeaderOption { ... }
```

- [ ] Refatorar `GetDefaultHeaders`/`WriteHeaders` para usar `HeaderOptions` struct ou functional options

---

### 3.3 Extrair router para pacote próprio

**Atualmente:** o router `fakeRouterHandler` vive em `cmd/httpserver/main.go`, junto com o `main()` e os handlers. Para o projeto crescer de forma organizada:

**Sugestão:**

```
internal/
├── handler/         ← handlers (fileHandler, defaultHandler)
│   └── handler.go
├── router/          ← lógica de roteamento
│   └── router.go
└── server/          ← já existe (não mexer)
```

A separação permite testar o router e os handlers independentemente do servidor.

- [ ] Mover `fileHandler` e `defaultHandler` para `internal/handler/`
- [ ] Mover `fakeRouterHandler` para `internal/router/`

---

### 3.4 Usar `slog` em vez de `log.Printf`

**Arquivo:** `internal/server/server.go`, `cmd/httpserver/main.go`, `cmd/tcplistener/main.go`

`log.Printf` não tem níveis (info, warn, error). `log/slog` é padrão desde Go 1.21 e já tem import comentado em `request.go`:

```go
// import "log/slog"
```

Migrar para `slog`:

```go
slog.Info("request", "method", req.Method, "uri", req.RequestURI)
slog.Error("failed to parse request", "err", err)
```

- [ ] Migrar logging para `slog`

---

### 3.5 Renomear `tcplistener` para `requestdebug` ou similar

**Arquivo:** `cmd/tcplistener/main.go`

O nome `tcplistener` descreve o mecanismo, não o propósito. O binário faz um dump da requisição parseada no terminal — é uma ferramenta de debug.

- [ ] Renomear `cmd/tcplistener/` para `cmd/requestdump/`
- [ ] Atualizar `main()` function

---

## 4. Coisas que Estão Boas

Estes pontos funcionam bem e não precisam de mudança:

- **Máquina de estados do parser** — abordagem correta para lidar com dados fragmentados de TCP. Manter.
- **`Headers.Parse()` com merge de valores múltiplos** — segue RFC 9110 §5.2. Manter.
- **Case-insensitivity nos headers** — normalizar para lowercase. Correto.
- **Validação de caracteres nos field names** — regex de tokens HTTP válidos. Correto.
- **`atomic.Bool` para `inShutdown`** — shutdown gracioso sem data race. Manter.
- **Separação entre produção de conteúdo (handler → buffer) e serialização (response → conn)** — bom design. Manter.
- **Leitura com buffer deslizante** (`copy(buf, buf[parsedN:])`) — eficiente e correto. Manter.
- **Testes existentes** — cobrem parse de request-line, headers, body, fragmentação, caracteres inválidos. Manter e expandir.

---

## 5. Fora de Escopo (Propositalmente)

Estes tópicos são importantes em produção mas não agregam ao propósito didático deste projeto:

| Tópico | Motivo |
|---|---|
| TLS/HTTPS | Complexidade de certificados não agrega ao aprendizado de HTTP |
| HTTP/2 | Paradigma completamente diferente (streams binários, multiplexação) |
| Chunked trailer com extensions complexas | Basta ignorar extensions |
| Compression (gzip) | Camada separada, não afeta o parser HTTP |
| Cache-control semantics | Header escrito mas sem implementação de cache |
| CORS, CSRF, autenticação | São camadas de aplicação, não de protocolo |
| Connection pooling | Só faz sentido com keep-alive (que está na lista) |
| Graceful shutdown com deadline | O atual `Close()` já é suficiente para um didático |
| Benchmark/otimização de performance | Não é o objetivo |

---

## Prioridade Sugerida

Organizado do mais urgente/impactante para o menos:

| # | Item | Esforço | Impacto didático |
|---|---|---|---|
| 1 | 🔴 Bug: body não é parseado (loop condition) | 1 linha | Alto — sem isso, POST não funciona |
| 2 | 🔴 Bug: handler error não é enviado ao cliente | 2 linhas | Alto — cliente fica sem resposta |
| 3 | 🔴 Bug: rota sem match retorna 200 | 4 linhas | Médio — comportamento errado |
| 4 | 🟡 Path traversal no fileHandler | 5 linhas | Médio — segurança básica |
| 5 | 🟡 Suporte a Transfer-Encoding: chunked (request) | ~80 linhas | Alto — feature HTTP fundamental |
| 6 | 🟡 Suporte a Transfer-Encoding: chunked (response) | ~40 linhas | Alto — streaming é caso de uso real |
| 7 | 🟢 Router com método HTTP + 404 real | ~20 linhas | Médio — organização |
| 8 | 🟢 Keep-Alive (conexão persistente) | ~30 linhas | Alto — HTTP/1.1 sem keep-alive é capenga |
| 9 | 🔵 Refatoração de `GetDefaultHeaders` | ~30 linhas | Baixo — só legibilidade |
| 10 | 🔵 Limpeza de código morto | ~5 linhas | Baixo — estética |
| 11 | 🔵 Extrair router para pacote próprio | ~20 linhas | Médio — organização |
| 12 | 🔵 Suporte a HTTP/1.0 | 2 linhas | Baixo — quase não usado |
| 13 | 🔵 Migrar para `slog` | ~10 linhas | Baixo — boas práticas |
| 14 | 🔵 Renomear `tcplistener` | ~5 min | Baixo — estética |

**Lenda:** 🔴 bug → 🟡 feature HTTP → 🟢 feature server → 🔵 refatoração

---

## Checklist Consolidado

### Bugs (corrigir primeiro)

- [ ] [1.1](plano#11-requestfromreader-sai-do-loop-antes-de-processar-o-body) — `RequestFromReader`: loop sai cedo, body ignorado
- [ ] [1.2](plano#12-handler-error-nunca-é-enviado-ao-cliente) — `server.handle()`: handler error nunca é escrito no `conn`
- [ ] [1.3](plano#13-rota-sem-match-retorna-nil-resposta-fantasma) — `fakeRouterHandler`: rota sem match retorna 200 vazio
- [ ] [1.4](plano#14-filehandler-vulnerável-a-path-traversal) — `fileHandler`: path traversal possível

### Features HTTP

- [ ] [2.1](plano#21-transfer-encoding-chunked-parsing-de-request) — Parse de `Transfer-Encoding: chunked` no request
- [ ] [2.2](plano#22-transfer-encoding-chunked-escrita-de-resposta) — Escrita chunked na resposta
- [ ] [2.3](plano#23-suporte-a-http10) — Aceitar HTTP/1.0
- [ ] [2.4](plano#24-validação-de-método-http-opcional) — Validar métodos HTTP (opcional)
- [ ] [2.5](plano#25-router-com-suporte-a-métodos-http-e-404-real) — Router com method-based routing + 404/405
- [ ] [2.6](plano#26-keep-alive-conexão-persistente) — Keep-Alive (conexão persistente)

### Refatorações

- [ ] [3.1](plano#31-limpeza-de-código-morto-em-requestgo) — Limpeza de código morto
- [ ] [3.2](plano#32-assinatura-de-getdefaultheaders-e-writeheaders-muitos-string) — Refatorar `GetDefaultHeaders`/`WriteHeaders`
- [ ] [3.3](plano#33-extrair-router-para-pacote-próprio) — Extrair router para pacote próprio
- [ ] [3.4](plano#34-usar-slog-em-vez-de-logprintf) — Migrar para `slog`
- [ ] [3.5](plano#35-renomear-tcplistener-para-requestdebug-ou-similar) — Renomear `cmd/tcplistener`