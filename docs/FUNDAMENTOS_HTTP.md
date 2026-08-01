# Fundamentos HTTP — Um Servidor Web do Zero em Go

## Resumo

| Camada | Responsabilidade | No seu código |
|---|---|---|
| **Listener** | Aceitar conexões TCP | `net.Listen("tcp", ":port")` + `listener.Accept()` |
| **Parser** | Transformar bytes em `Request` | `request.RequestFromReader(conn)` |
| **Router** | Decidir qual handler chamar pela URL | `fakeRouterHandler` (switch-case manual) |
| **Handler** | Produzir o conteúdo da resposta | `defaultHandler`, `fileHandler`, `Handler func(w io.Writer, req *Request) *HandlerError` |
| **Response Writer** | Escrever status + headers + body | `response.WriteStatusLine`, `response.WriteHeaders`, `conn.Write(body)` |

---

## 1. Listener — Aceitar Conexões TCP

### O que diz a RFC

O HTTP/1.1 opera sobre uma conexão TCP (ou TLS). O **Listener** é a camada de transporte que fica escutando em uma porta, aceita conexões de clientes e entrega um `net.Conn` — um stream bidirecional de bytes — para as camadas superiores.

### No seu código

**`internal/server/server.go`** — `Serve()` e `listen()`:

```go
listener, err := net.Listen("tcp", ":"+addressPort)  // (1)
// ...
go server.listen()                                     // (2)
```

```go
func (s *Server) listen() {
    for !s.inShutdown.Load() {
        conn, err := s.listener.Accept()               // (3)
        // ...
        go s.handle(conn)                              // (4)
    }
}
```

| Etapa | O que acontece |
|---|---|
| `(1)` | Cria um socket TCP na porta especificada. O kernel começa a enfileirar conexões SYN recebidas (backlog). |
| `(2)` | Dispara o loop de aceitação em uma goroutine — o servidor não bloqueia o `main()`. |
| `(3)` | `Accept()` bloqueia até que o handshake TCP triplo (SYN, SYN-ACK, ACK) seja concluído e retorna um `net.Conn`. |
| `(4)` | Cada conexão é tratada em sua própria goroutine — concorrência sem threads do SO. |

O `inShutdown atomic.Bool` permite um desligamento gracioso: ao chamar `Close()`, o listener para de aceitar novas conexões, mas as goroutines já em execução terminam naturalmente.

**`cmd/tcplistener/main.go`** — versão minimalista que faz o mesmo manualmente, sem o pacote `server`.

---

## 2. Parser — Transformar Bytes em `Request`

### O que diz a RFC

O formato de uma requisição HTTP/1.1 é definido pelo **RFC 9112** (antes RFC 7230). A estrutura é:

```
request-line   = method SP request-target SP HTTP-version CRLF
header-field   = field-name ":" OWS field-value OWS CRLF
message-body   = *OCTET  (quando Content-Length ou Transfer-Encoding presente)
```

Separador entre headers e body: `CRLF` solitário (`\r\n\r\n`).

### No seu código

**`internal/request/request.go`** — `RequestFromReader()`:

```go
func RequestFromReader(reader io.Reader) (*Request, error) {
    request := NewRequest()
    buf := make([]byte, BufferSize)  // buffer de 4096 bytes
    bufLen := 0

    for !request.bodyDone() && ... {
        n, err := reader.Read(buf[bufLen:])   // (1) lê do socket
        bufLen += n

        parsedN, err := request.parse(buf[:bufLen])  // (2) tenta parsear
        copy(buf, buf[parsedN:bufLen])                // (3) desloca resto
        bufLen -= parsedN
    }
    return request, nil
}
```

#### Máquina de estados do parser

O `Request.parse()` usa uma **máquina de estados** (`ParserState`) que avança à medida que cada parte da mensagem é consumida:

```
StateReqLineInitialized
    ↓ (parseRequestLine ok)
StateReqLineDone
    ↓
StateHeadersInitialized
    ↓ (Headers.Parse ok + done)
StateHeadersDone
    ↓ (se Content-Length > 0)
StateBodyInitialized
    ↓ (bodyWritten >= contentLength)
StateBodyDone  ← sai do loop
```

Cada estado só avança quando os bytes necessários estão disponíveis no buffer. Se faltam dados (`n == 0`), o loop de leitura busca mais bytes do socket.

#### Request-Line

```go
func parseRequestLine(text []byte) (int, *RequestLine, error) {
    // "GET / HTTP/1.1\r\n"
    reqParts := bytes.Split(reqLine, []byte(" "))
    // reqParts = ["GET", "/", "HTTP/1.1"]
    httpParts := bytes.Split(reqParts[2], []byte("/"))
    // httpParts = ["HTTP", "1.1"]
}
```

Validações:
- Método deve conter **apenas letras maiúsculas** (`^[A-Z]+$`)
- Exatamente 3 partes separadas por espaço
- Versão HTTP deve ser `1.1` (rejeita `HTTP/2.0`, `HTTP/1.0`, etc.)

#### Headers

**`internal/headers/headers.go`** — `Headers.Parse()`:

```go
func (h Headers) Parse(data []byte) (int, bool, error) {
    for {
        fieldLineIdx := bytes.Index(data[numBytesRead:], CRLFByte)
        // ...
        fieldLineParts := bytes.SplitN(fieldLine, ColonByte, 2)
        fieldName := fieldLineParts[0]
        fieldValue := bytes.TrimSpace(fieldLineParts[1])
        // Valida nome do campo com regex
        if !isHeaderLineValid(string(fieldName)) { ... }
        // Armazena em lowercase (case-insensitive)
        fieldNameLower := strings.ToLower(string(fieldName))
        // Se já existe, faz merge com ", " (RFC 9110 §5.2)
        h[fieldNameLower] = strings.Join([...], ", ")
    }
}
```

Pontos de atenção:
- **Case-insensitive**: nomes são normalizados para lowercase (ex: `Content-Length` → `content-length`)
- **Merge de valores múltiplos**: `Accept: text/html` + `Accept: application/json` → `"text/html, application/json"`
- **Trim de espaços**: valores com espaços leading/trailing são limpos
- **Validação rigorosa**: caracteres inválidos no nome do campo (ex: `©`) ou espaços antes do `:` são rejeitados

#### Body

O body só é lido se o header `Content-Length` existir e for > 0. O parser aloca `Body = make([]byte, contentLength)` e copia os bytes conforme chegam, rastreando `bodyWritten`. Se a conexão fechar antes de completar o body, retorna `ErrIncompleteBody`.

---

## 3. Router — Decidir Qual Handler Chamar

### O que diz a RFC

O HTTP não define um mecanismo de roteamento — ele é responsabilidade da aplicação. O **request-target** (a URI) é o único dado que o servidor tem para decidir como responder. Cabe ao servidor Web implementar a lógica de dispatcher.

### No seu código

**`cmd/httpserver/main.go`** — `fakeRouterHandler`:

```go
func fakeRouterHandler(w io.Writer, req *request.Request) *server.HandlerError {
    switch req.RequestLine.RequestURI {
    case "/static/landing.html":
        return fileHandler(w, req)
    case "/":
        return defaultHandler(w, req)
    }
    return nil  // fallthrough — sem match explicito
}
```

Este é um **roteador manual baseado em switch-case**. Ele:
1. Lê `req.RequestURI` (ex: `/static/landing.html`)
2. Compara com padrões fixos
3. Delega para o handler correspondente

**Limitação atual**: não há suporte a:
- Parâmetros de path (`/users/:id`)
- Query strings roteadas (apenas passam como parte da URI)
- Métodos HTTP diferentes na mesma rota
- Middleware chain
- Fallback 404 explícito (o `return nil` sem escrever nada no `w` não envia resposta)

Para um roteador mais completo, você poderia implementar uma **trie** ou **radix tree** de padrões de URL — é o que fazem `http.ServeMux`, `gin`, `chi`, etc.

---

## 4. Handler — Produzir o Conteúdo da Resposta

### O que diz a RFC

O handler é a lógica da aplicação que recebe uma requisição já parseada e produz os bytes que formarão a resposta. O HTTP não dita como isso deve ser feito — apenas que o resultado deve ser uma mensagem HTTP válida.

### No seu código

**`internal/server/server.go`** — definição do tipo Handler:

```go
type Handler func(w io.Writer, req *request.Request) *HandlerError

type HandlerError struct {
    StatusCode response.StatusCode
    Message    string
}
```

O handler recebe um `io.Writer` (buffer) e o `*Request` parseado. Se retornar `nil`, o servidor assume sucesso e escreve `200 OK` + headers + body. Se retornar `*HandlerError`, o servidor escreve o status code e mensagem de erro correspondentes.

**`cmd/httpserver/main.go`** — dois handlers concretos:

```go
func defaultHandler(w io.Writer, req *request.Request) *server.HandlerError {
    _, err := w.Write([]byte("<strong>Greetings!</strong><br/>" +
        "Welcome to the '" + req.RequestLine.RequestURI + "'<br/>"))
    if err != nil {
        return &server.HandlerError{
            StatusCode: response.StatusInternalServerError,
            Message:    "Failed to read file\n",
        }
    }
    return nil
}

func fileHandler(w io.Writer, req *request.Request) *server.HandlerError {
    filePath := filepath.Join(".", req.RequestLine.RequestURI)
    file, err := os.Open(filePath)
    if err != nil {
        return &server.HandlerError{
            StatusCode: response.StatusNotFound,
            Message:    "File not found\n",
        }
    }
    defer file.Close()
    _, err = io.Copy(w, file)
    // ...
}
```

| Handler | Função |
|---|---|
| `defaultHandler` | Escreve uma string HTML simples no buffer |
| `fileHandler` | Abre um arquivo do sistema de arquivos e copia seu conteúdo para o buffer |

O handler escreve em um `bytes.Buffer` (não diretamente no `conn`). Depois o servidor pega esse buffer e escreve a resposta HTTP completa no socket. Isso separa a **produção do conteúdo** da **serialização HTTP**.

---

## 5. Response Writer — Escrever Status + Headers + Body

### O que diz a RFC

Uma resposta HTTP/1.1 tem o formato (RFC 9112):

```
status-line    = HTTP-version SP status-code SP reason-phrase CRLF
header-field   = field-name ":" field-value CRLF
CRLF
message-body   = *OCTET
```

### No seu código

**`internal/response/response.go`** — três funções principais:

#### `WriteStatusLine`

```go
func WriteStatusLine(writer io.Writer, status StatusCode) error {
    text, ok := reasonPhrases[status]
    // ...
    _, err := writer.Write([]byte("HTTP/1.1 " +
        strconv.Itoa(int(status)) + " " + text + "\r\n"))
    return err
}
```

Produz: `HTTP/1.1 200 OK\r\n`

Os status codes suportados são mapeados no mapa `reasonPhrases`:

| Código | Frase | Uso |
|---|---|---|
| 200 | OK | Sucesso |
| 400 | Bad Request | Requisição mal-formada |
| 404 | Not Found | Recurso não encontrado |
| 500 | Internal Server Error | Erro inesperado no servidor |

#### `WriteHeaders`

```go
func WriteHeaders(writer io.Writer, responseContentLen int, ...) error {
    h := GetDefaultHeaders(responseContentLen, contentType, ...)
    for _, key := range defaultHeaders {
        value, ok := h[key]
        // ...
        writer.Write([]byte(key + ": " + value + "\r\n"))
    }
    writer.Write([]byte("\r\n"))  // CRLF final separador headers/body
}
```

Headers padrão escritos em toda resposta:

| Header | Default |
|---|---|
| `content-length` | Tamanho do body |
| `content-type` | `text/html; charset=utf-8` |
| `connection` | `close` |
| `date` | Data UTC atual no formato RFC 7231 |
| `content-encoding` | `identity` |
| `cache-control` | `no-cache` |
| `server` | `NGONX/Apache` |

#### `WriteResponse` (atalho)

Junta as três etapas em uma chamada:

```go
func WriteResponse(writer io.Writer, status int, ..., body []byte) error {
    WriteStatusLine(writer, StatusCode(status))
    WriteHeaders(writer, ...)
    writer.Write(body)
}
```

#### O fluxo completo no `server.handle()`

```go
func (s *Server) handle(conn net.Conn) {
    defer conn.Close()

    req, err := request.RequestFromReader(conn)  // Parser
    // ...
    buf := new(bytes.Buffer)
    handlerErr := s.handler(buf, req)             // Handler escreve no buffer
    // ...
    body := buf.Bytes()
    response.WriteStatusLine(conn, response.StatusOK)  // (1)
    response.WriteHeaders(conn, len(body), ...)         // (2)
    conn.Write(body)                                    // (3)
}
```

O `conn` (que implementa `io.Writer`) recebe os bytes diretamente — não há buffering adicional além do TCP stack do kernel.

---

## Mapa do Projeto

```
ngonx/
├── cmd/
│   ├── httpserver/main.go      # Servidor completo: router + handlers
│   └── tcplistener/main.go     # Listener TCP puro + parser (debug)
├── internal/
│   ├── headers/headers.go      # Parse de headers HTTP
│   ├── request/request.go      # Parse de request-line + body + máq. estados
│   ├── response/response.go    # Serialização de resposta HTTP
│   └── server/server.go        # Listener + accept loop + handler dispatch
├── static/landing.html         # Arquivo servido pelo fileHandler
└── FUNDAMENTOS_HTTP.md         # Este arquivo
```

---

## Referências

| RFC | Título | Relevância |
|---|---|---|
| **RFC 9110** | HTTP Semantics | Semântica de headers, métodos, status codes |
| **RFC 9112** | HTTP/1.1 Message Syntax and Routing | Formato exato de request/response lines, chunked encoding |
| **RFC 7231** | HTTP/1.1 Semantics and Content (obsoleta pela 9110) | Definições originais de métodos e status |
| **RFC 3986** | URI Syntax | Estrutura de URIs (scheme, authority, path, query) |