# Transfer-Encoding: Chunked — Guia Completo

## Resumo

Chunked encoding é um mecanismo do HTTP/1.1 que permite enviar o **message-body** em uma série de pedaços (chunks) de tamanho conhecido individualmente, sem precisar saber o tamanho total **antes** de começar a transmissão.

| Aspecto | Content-Length | Transfer-Encoding: chunked |
|---|---|---|
| Conhecimento do tamanho | Deve saber o tamanho **antes** de escrever os headers | Não precisa saber — calcula-se durante o envio |
| Delimitação do body | Valor do header `Content-Length` | Terminador `0\r\n\r\n` |
| Streaming | Não suporta (precisa do body completo primeiro) | Suporta nativamente |
| Uso típico | Respostas pequenas ou com tamanho conhecido | Streaming, arquivos grandes, dados dinâmicos |
| Regra RFC | **Mutualmente exclusivo** com chunked (RFC 9112 §6.1) | **NÃO** deve ter `Content-Length` |

---

## 1. Formato Wire (On The Wire)

### Formato genérico (RFC 9112 §7.1)

```
chunked-body = *chunk
               last-chunk
               trailer-section
               CRLF

chunk         = chunk-size [ chunk-ext ] CRLF
                chunk-data CRLF

chunk-size    = 1*HEXDIG
last-chunk    = 1*("0") [ chunk-ext ] CRLF

chunk-data    = 1*OCTET ; a sequência de octetos de tamanho chunk-size

trailer-section = *( field-line CRLF )
```

### Exemplo concreto

```
HTTP/1.1 200 OK
Content-Type: text/plain
Transfer-Encoding: chunked

1E
I could go for a cup of coffee
C
But not Java
12
Never go full Java
0

```

O que acontece byte a byte:

```
"1E\r\n"                       → chunk-size: 0x1E = 30 bytes
"I could go for a cup of coffee\r\n"  → 30 bytes de dados + CRLF
"C\r\n"                        → chunk-size: 0x0C = 12 bytes
"But not Java\r\n"            → 12 bytes de dados + CRLF
"12\r\n"                       → chunk-size: 0x12 = 18 bytes
"Never go full Java\r\n"      → 18 bytes de dados + CRLF
"0\r\n"                        → last-chunk: tamanho zero
"\r\n"                         → trailer-section vazio (só o CRLF)
```

### Regras importantes

1. **`chunk-size` é hexadecimal** em ASCII (ex: `1E` = 30, `0` = zero)
2. Cada chunk é **autodelimitado**: você sabe exatamente quantos bytes ler porque o tamanho vem antes
3. O **last-chunk** é `0\r\n` seguido do trailer e CRLF final
4. **Trailer** permite headers opcionais após o body (ex: `Content-MD5`, `Trailer: X-Process-Time`)
5. **Chunk extensions** (`chunk-ext`) são opcionais: `chunk-size;extension=value\r\n`

---

## 2. Content-Length vs Chunked

A RFC 9112 §6.1 é clara: **as duas formas são mutuamente exclusivas**.

```
HTTP/1.1 200 OK
Content-Type: text/plain
Content-Length: 40
Transfer-Encoding: chunked

28
Este é um body com tamanho conhecido
0

```

Isso é **inválido** — o receptor deve priorizar `Transfer-Encoding` e ignorar `Content-Length` (RFC 9110 §8.6).

### Quando usar cada um?

| Situação | Escolha |
|---|---|
| Resposta pequena (< 1 KB) e tamanho conhecido | `Content-Length` |
| Arquivo grande (ex: 2 GB) | `Transfer-Encoding: chunked` |
| Resposta gerada dinamicamente (streaming de LLM, live feed) | `Transfer-Encoding: chunked` |
| Resposta de uma consulta de banco que você sabe o tamanho | `Content-Length` |
| Você não sabe o tamanho até o último byte | `Transfer-Encoding: chunked` |

---

## 3. Leitura (Parsing) de Chunked — Request

### Máquina de estados para parsear chunked

O parser atual (`request.go`) só trata `Content-Length`. Para suportar chunked no **recebimento**, seria necessário uma máquina de estados adicional dentro de `StateBodyInitialized`:

```
StateChunkSize
    ↓ lê linha até CRLF, converte hex para int
    ↓ se n == 0 → StateChunkTrailer
StateChunkData
    ↓ lê exatos `chunk-size` bytes
    ↓ append no Body
    ↓ lê CRLF (esperado após chunk-data)
    → volta para StateChunkSize
StateChunkTrailer
    ↓ lê headers do trailer até CRLF vazio
    → StateBodyDone
```

### O que muda no `Request` atual

```go
type Request struct {
    RequestLine
    headers.Headers
    Body            []byte
    contentLength   int
    bodyWritten     int
    ParserState

    // Novo: estado interno do chunked parser
    chunkedBody     bool
    chunkRemaining  int      // bytes restantes do chunk atual
    chunkState      ChunkState
}

type ChunkState int
const (
    ChunkStateAwaitingSize ChunkState = iota
    ChunkStateAwaitingData
    ChunkStateAwaitingCRLF
    ChunkStateTrailer
    ChunkStateDone
)
```

### Lógica do parse de chunk

```go
// Pseudo-código para o estado StateBodyInitialized com chunked
case StateBodyInitialized:
    if r.chunkedBody {
        // --- parse chunked ---
        n, err := r.parseChunked(data[read:])
        read += n
        if err != nil {
            r.ParserState = StateBodyError
            return 0, err
        }
        if r.chunkState == ChunkStateDone {
            r.ParserState = StateBodyDone
        }
    } else {
        // lógica atual com content-length
    }
```

### Função `parseChunked`

```go
func (r *Request) parseChunked(data []byte) (int, error) {
    read := 0
    for r.chunkState != ChunkStateDone {
        switch r.chunkState {
        case ChunkStateAwaitingSize:
            // Procura CRLF na linha do chunk-size
            idx := bytes.Index(data[read:], CRLFByte)
            if idx == -1 {
                return read, nil // precisa de mais dados
            }
            sizeLine := data[read : read+idx]
            // Converte hex para int (ex: "1E" → 30)
            size, err := strconv.ParseInt(string(sizeLine), 16, 64)
            if err != nil {
                return read, ErrInvalidChunkSize
            }
            read += idx + len(CRLFByte)
            r.chunkRemaining = int(size)
            if r.chunkRemaining == 0 {
                r.chunkState = ChunkStateTrailer
            } else {
                r.chunkState = ChunkStateAwaitingData
            }

        case ChunkStateAwaitingData:
            available := len(data[read:])
            toCopy := min(r.chunkRemaining, available)
            r.Body = append(r.Body, data[read:read+toCopy]...)
            read += toCopy
            r.chunkRemaining -= toCopy
            if r.chunkRemaining == 0 {
                r.chunkState = ChunkStateAwaitingCRLF
            } else {
                return read, nil
            }

        case ChunkStateAwaitingCRLF:
            if len(data[read:]) < len(CRLFByte) {
                return read, nil
            }
            if !bytes.HasPrefix(data[read:], CRLFByte) {
                return read, ErrMissingChunkCRLF
            }
            read += len(CRLFByte)
            r.chunkState = ChunkStateAwaitingSize

        case ChunkStateTrailer:
            // Reusa o parser de headers já existente
            n, done, err := r.Headers.Parse(data[read:])
            read += n
            if err != nil {
                return read, err
            }
            if done {
                r.chunkState = ChunkStateDone
            } else {
                return read, nil
            }
        }
    }
    return read, nil
}
```

---

## 4. Escrita (Serialização) de Chunked — Response

### O que muda no `response.go`

Atualmente `WriteResponse` e `WriteHeaders` sempre escrevem `Content-Length`. Para suportar chunked **na resposta**, seriam necessárias duas novas funções:

```go
// Em vez de WriteHeaders, chama-se WriteChunkedHeaders
func WriteChunkedHeaders(writer io.Writer, contentType string) error {
    writer.Write([]byte("HTTP/1.1 200 OK\r\n"))
    writer.Write([]byte("Content-Type: " + contentType + "\r\n"))
    writer.Write([]byte("Transfer-Encoding: chunked\r\n"))
    // NÃO escreve Content-Length!
    writer.Write([]byte("Connection: close\r\n"))
    writer.Write([]byte("Date: " + getCurrentUTCDate() + "\r\n"))
    writer.Write([]byte("\r\n"))
    return nil
}

func WriteChunk(writer io.Writer, data []byte) error {
    if len(data) == 0 {
        return nil
    }
    // Tamanho em hexadecimal + CRLF
    _, err := fmt.Fprintf(writer, "%x\r\n", len(data))
    if err != nil {
        return err
    }
    // Dados + CRLF
    _, err = writer.Write(data)
    if err != nil {
        return err
    }
    _, err = writer.Write(CRLFByte)
    return err
}

func WriteChunkedTrailer(writer io.Writer) error {
    // Last-chunk: "0\r\n" + trailer vazio "\r\n"
    _, err := writer.Write([]byte("0\r\n\r\n"))
    return err
}
```

### Uso no handler (streaming)

```go
func streamingHandler(w io.Writer, req *request.Request) *server.HandlerError {
    // 1. Escreve status line + headers chunked
    response.WriteStatusLine(w, response.StatusOK)
    response.WriteChunkedHeaders(w, "text/plain")

    // 2. Envia chunks um por um
    response.WriteChunk(w, []byte("Hello"))
    time.Sleep(1 * time.Second)
    response.WriteChunk(w, []byte("World"))
    time.Sleep(1 * time.Second)
    response.WriteChunk(w, []byte("!!"))

    // 3. Finaliza
    response.WriteChunkedTrailer(w)
    return nil
}
```

### Atenção: flush

Para streaming funcionar **em tempo real** (não apenas em buffers), você precisa forçar o flush a cada chunk. Em Go:

```go
// Se o writer for um net.Conn, você precisa de um *bufio.Writer
// e chamar Flush() após cada WriteChunk

bw := bufio.NewWriter(w)
// ... escreve status line, headers ...
bw.Flush()

// A cada chunk:
response.WriteChunk(bw, data)
bw.Flush()  // ← essencial para o cliente receber imediatamente

response.WriteChunkedTrailer(bw)
bw.Flush()
```

---

## 5. Análise no Seu Código Atual

### O que existe hoje

**`response.go`** — `GetDefaultHeaders` e `WriteHeaders`:

```go
func GetDefaultHeaders(contentLen int, ...) headers.Headers {
    h := headers.NewHeaders()
    h["content-length"] = strconv.Itoa(contentLen)  // ← sempre presente
    // ...
}

var defaultHeaders []string = []string{
    "content-length",  // ← obrigatório
    "content-type",
    "connection",
    "date",
    "content-encoding",
    "cache-control",
    "server",
}
```

**`request.go`** — transição `StateHeadersDone`:

```go
case StateHeadersDone:
    contentLengthStr, exists := r.Headers["content-length"]
    if !exists {
        r.ParserState = StateBodyDone  // ← sem Content-Length = sem body
        continue
    }
    // ...
    r.contentLength, err = strconv.Atoi(contentLengthStr)
```

### O que falta para suportar chunked

| Funcionalidade | Status | Onde implementar |
|---|---|---|
| Parse de chunk-size (hex) | ❌ Não existe | `request.go` — novo `parseChunked()` |
| Append de chunks no Body | ❌ Não existe | `request.go` — `StateBodyInitialized` |
| Detecção de `Transfer-Encoding: chunked` nos headers | ❌ Não existe | `request.go` — `StateHeadersDone` |
| Escrita de `Transfer-Encoding: chunked` na resposta | ❌ Não existe | `response.go` — nova `WriteChunkedHeaders()` |
| Função `WriteChunk` | ❌ Não existe | `response.go` — nova função |
| Função `WriteChunkedTrailer` | ❌ Não existe | `response.go` — nova função |
| Flush explícito para streaming | ❌ Não tratado | `server.go` — `handle()` com `bufio.Writer` |

### O que precisa mudar no `StateHeadersDone`

```go
case StateHeadersDone:
    // Verifica se é chunked
    transferEncoding, teExists := r.Headers["transfer-encoding"]
    if teExists && strings.Contains(transferEncoding, "chunked") {
        r.chunkedBody = true
        r.ParserState = StateBodyInitialized
        r.Body = make([]byte, 0) // slice vazio, vai dar append
        continue
    }

    // Lógica atual (Content-Length)
    contentLengthStr, exists := r.Headers["content-length"]
    if !exists {
        r.ParserState = StateBodyDone
        continue
    }
    // ...
```

### Mapa de mudanças

```
request.go:
  - Adicionar ChunkState enum
  - Adicionar campos chunkedBody, chunkRemaining, chunkState no Request
  - Novo método parseChunked()
  - Modificar StateHeadersDone para detectar/chunked
  - Modificar StateBodyInitialized para bifurcar entre Content-Length e chunked

response.go:
  - Nova função WriteChunkedHeaders()
  - Nova função WriteChunk()
  - Nova função WriteChunkedTrailer()

server.go:
  - Opcional: usar bufio.Writer + Flush() no handle()
```

---

## 6. Casos de Borda (Edge Cases)

### Chunk extension

```
1E;chunk-id=42\r\n
dados aqui\r\n
```

O parser atual quebra se encontrar algo após o tamanho. Solução: ignorar tudo entre `;` e `\r\n` na linha de chunk-size.

### Trailer com headers

```
0\r\n
X-Process-Time: 1.2s\r\n
X-Final-Checksum: abc123\r\n
\r\n
```

O trailer é opcional. Se existir, contém headers como `Trailer` foi declarado nos headers iniciais (RFC 9110 §6.5.2).

### Chunk de tamanho zero no meio do stream

Se o servidor enviar `0\r\n` antes de terminar os dados, o cliente deve aceitar como fim de stream — isso é um comportamento válido.

### Conexão cai no meio de um chunk

Se a conexão TCP for fechada enquanto `chunkRemaining > 0`, o parser deve retornar `ErrIncompleteChunkedBody` (análogo ao `ErrIncompleteBody` já existente).

---

## 7. RFCs e Referências

| RFC | Título | Seção relevante |
|---|---|---|
| **RFC 9112** | HTTP/1.1 Message Syntax and Routing | §6.1 (Transfer-Encoding), §7.1 (Chunked Body), §7.1.1 (Chunk Extensions) |
| **RFC 9110** | HTTP Semantics | §8.6 (Content-Length vs Transfer-Encoding), §6.5.2 (Trailer) |
| **RFC 7230** | HTTP/1.1 Message Syntax (original, obsoleta pela 9112) | §4.1 (Chunked Transfer Coding) |

---

## 8. Exercícios Práticos

1. **Parser**: Escreva um teste que envia `"1E\r\n...0\r\n\r\n"` e verifica se `Request.Body` contém os dados concatenados.
2. **Escrita**: Escreva `WriteChunk` e verifique com `bytes.Buffer` se a saída é `"3\r\nabc\r\n"`.
3. **Integração**: Crie um handler que faz streaming de 3 chunks com 1 segundo de intervalo entre eles. Teste com `curl --raw`.
4. **Edge case**: Envie um chunked body com chunk extension e veja se o parser ignora a extension.
5. **Mutual exclusivity**: Envie uma resposta com ambos `Content-Length` e `Transfer-Encoding: chunked`. O parser deve priorizar chunked.

```
curl -N http://localhost:4002/stream
```

A flag `-N` (ou `--no-buffer`) faz o `curl` mostrar os dados conforme chegam, ideal para testar streaming.