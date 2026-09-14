# Plano de Validação e Roadmap de Desenvolvimento — ngonx

> **Diretriz deste documento:** Este material atua estritamente como um guia conceitual e técnico. Nenhum trecho de solução em código é fornecido aqui. Você encontrará a citação do trecho problemático atual, a explicação técnica detalhada do erro e os conceitos necessários para que você mesmo projete e programe cada solução.

---

## 1. Validação do que já foi feito

Uma auditoria no código-fonte atual revelou que vários itens planejados anteriormente já foram implementados ou avançaram significativamente. Abaixo está o diagnóstico do estado real da base de código:

### 1.1 Condição de parada do loop em `RequestFromReader`
* **Local:** `internal/request/request.go`
* **Status:** **Resolvido**.
* **Observação:** O encerramento prematuro que ocorria quando os headers terminavam não existe mais. A condição do laço foi simplificada para depender apenas do término ou de erros no corpo ou na linha de requisição. Requisições com `Content-Length` agora continuam no laço até que todo o corpo seja consumido.

### 1.2 Máquina de estados para parsing de requisições Chunked
* **Local:** `internal/request/request.go` (métodos `parse` e `parseChunked`)
* **Status:** **Implementado no parser, mas pendente de validação por testes**.
* **Observação:** A máquina de estados (`ChunkStateAwaitingSize`, `ChunkStateAwaitingData`, `ChunkStateAwaitingCRLF`, `ChunkStateTrailer`, `ChunkStateDone`) foi construída e já consome tamanhos em hexadecimal e trata trailers. No entanto, o arquivo `request_test.go` não possui nenhum teste unitário que exercite esse fluxo de chunked.

### 1.3 Suporte a HTTP/1.0 na linha de requisição
* **Local:** `internal/request/request.go` (função `parseRequestLine`)
* **Status:** **Resolvido**.
* **Observação:** A verificação da versão já aceita tanto `"1.1"` quanto `"1.0"`.

### 1.4 Resposta 404 no roteador
* **Local:** `cmd/httpserver/main.go` (função `fakeRouterHandler`)
* **Status:** **Resolvido**.
* **Observação:** Quando a URI requisitada não coincide com nenhum caso, o handler agora retorna explicitamente uma estrutura com status 404 em vez de `nil`.

### 1.5 Prevenção básica contra Path Traversal
* **Local:** `cmd/httpserver/main.go` (função `fileHandler`)
* **Status:** **Resolvido parcialmente**.
* **Observação:** A utilização de isolamento no caminho impede ataques como `../../etc/passwd`. Porém, a estratégia adotada trouxe um efeito colateral na navegação de subdiretórios (detalhado na seção de problemas).

---

## 2. Diagnóstico Técnico dos Problemas e Limitações

Abaixo estão os pontos que requerem atenção, correção ou implementação, ordenados pela natureza do problema.

---

### 2.1 Poluição do buffer de resposta em erros de handlers

**Arquivo:** `internal/server/server.go` (linhas 93–105)

**Trecho atual:**
```go
buf := new(bytes.Buffer)

handlerErr := s.handler(buf, req)
if handlerErr != nil {
    writeHandlerError(buf, handlerErr)
    body := buf.Bytes()
    _, err = conn.Write(body)
    if err != nil {
        log.Printf("%v: %v", response.ErrWritingBody, err.Error())
        return
    }
    return
}
```

* **Erro técnico:** O handler recebe a referência direta do buffer `buf`. Se o handler escrever qualquer dado parcial no buffer antes de encontrar um erro e retornar um `*HandlerError`, o buffer já conterá esses bytes residuais. Ao chamar a função auxiliar de erro passando esse mesmo buffer, os cabeçalhos de erro HTTP e a mensagem de erro serão anexados *depois* dos dados corrompidos preexistentes. O cliente receberá um payload quebrado violando a especificação HTTP. Além disso, existe uma inconsistência arquitetural: a função de erro serializa status, cabeçalhos e corpo para dentro do buffer, enquanto o caminho de sucesso escreve status e cabeçalhos diretamente no socket de rede e utiliza o buffer exclusivamente para o corpo.
* **Conceito para correção:** Garanta que a serialização da resposta de erro não compartilhe bytes acumulados anteriormente por uma tentativa fracassada do handler. O buffer deve ser limpo/reiniciado antes da escrita do erro, ou a rotina de erro deve escrever diretamente no socket de rede ou em um canal isolado, mantendo a simetria com a forma como as respostas de sucesso são transmitidas.

---

### 2.2 Conexão encerrada abruptamente sem resposta HTTP em erros de parsing

**Arquivo:** `internal/server/server.go` (linhas 86–90)

**Trecho atual:**
```go
req, err := request.RequestFromReader(conn)
if err != nil {
    log.Printf("error at parsing request, err: %v", err.Error())
    return
}
```

* **Erro técnico:** Se o cliente enviar uma requisição malformada, com versão não suportada, método inválido ou headers corrompidos, a função apenas registra o erro no log do servidor e executa `return`. Como a função possui `defer conn.Close()`, a conexão TCP é finalizada silenciosamente (enviando um pacote TCP FIN/RST ao cliente). Segundo os padrões HTTP, o servidor deve sempre notificar o cliente do motivo da falha através de um status apropriado (como 400 Bad Request ou 505 HTTP Version Not Supported) antes de fechar o canal.
* **Conceito para correção:** Analise o erro retornado pelo leitor de requisição. Se for decorrente de sintaxe inválida ou violação de protocolo, envie uma resposta HTTP formatada com a linha de status adequada e corpo explicativo para o socket antes de abortar a conexão.

---

### 2.3 Perda de hierarquia de subdiretórios no serviço de arquivos

**Arquivo:** `cmd/httpserver/main.go` (linhas 61–65)

**Trecho atual:**
```go
func fileHandler(w io.Writer, req *request.Request) *server.HandlerError {
	baseDir := "static"
	safePath := filepath.Base(req.RequestLine.RequestURI)
	filePath := filepath.Join(baseDir, safePath)
```

* **Erro técnico:** A função utilizada para extrair o nome seguro do arquivo descarta todo o prefixo de diretórios, retendo unicamente o último elemento do caminho. Se o cliente solicitar um recurso localizado em um subdiretório (por exemplo, `/static/css/main.css` ou `/static/images/logo.png`), o caminho é achatado e o arquivo é procurado diretamente na raiz da pasta estática, falhando em localizar o arquivo ou servindo o arquivo incorreto caso existam arquivos com nomes idênticos em pastas diferentes.
* **Conceito para correção:** Em vez de descartar os subdiretórios, a técnica recomendada consiste em normalizar e limpar o caminho relativo, verificar explicitamente se o caminho resultante não tenta escapar do diretório raiz permitido (assegurando que o caminho limpo permaneça confinado dentro da fronteira do diretório base) e apenas então resolver o caminho final no sistema operacional.

---

### 2.4 Ausência de verificação de método HTTP no roteamento

**Arquivo:** `cmd/httpserver/main.go` (linhas 33–44)

**Trecho atual:**
```go
func fakeRouterHandler(w io.Writer, req *request.Request) *server.HandlerError {
	slog.Info("req line ->", "method", req.RequestLine.Method, "uri", req.RequestLine.RequestURI)
	switch req.RequestLine.RequestURI {
	case "/landing.html":
		handlerError := fileHandler(w, req)
		return handlerError

	case "/":
		handlerError := defaultHandler(w, req)
		return handlerError
	}
```

* **Erro técnico:** O roteamento toma decisões baseando-se unicamente na URI. Qualquer método HTTP (seja `POST`, `DELETE`, `PUT` ou qualquer outro) enviado para `/` acionará o `defaultHandler` e devolverá uma resposta de sucesso como se fosse uma consulta válida.
* **Conceito para correção:** A estrutura de roteamento deve associar tanto o método HTTP quanto o caminho da rota. Caso uma rota exista para determinado método (por exemplo, apenas `GET`), mas seja acessada por outro verbo HTTP, a resposta correta segundo a RFC deve informar que o método não é permitido (status 405 Method Not Allowed), preferencialmente informando quais métodos são aceitos.

---

### 2.5 Ausência de cobertura de testes para parsing de Chunked Requests

**Arquivo:** `internal/request/request_test.go`

* **Erro técnico:** Embora a lógica de `parseChunked` tenha sido introduzida em `request.go`, não há testes automatizados que validem sua execução sob condições reais de rede TCP.
* **Conceito para correção:** Adicione casos de teste em `request_test.go` simulando fluxos com o leitor fragmentado (`chunkReader`). Teste cenários fundamentais:
  - Requisição com corpo dividido em múltiplos pedaços de tamanhos hexadecimais diferentes.
  - Chunk finalizador de tamanho zero (`0\r\n\r\n`).
  - Leitura fragmentada de 1 em 1 byte para verificar se a máquina de estados não quebra com fragmentação em limites de CRLF.
  - Leitura de trailers opcionais após o chunk final.
  - Chunks com tamanho inválido ou ausência do CRLF delimitador (esperando os erros já definidos no pacote).

---

### 2.6 Vazamento de conexões (File Descriptors) no `tcplistener`

**Arquivo:** `cmd/tcplistener/main.go` (linha 33)

**Trecho atual:**
```go
		if request.Body != nil {
			fmt.Printf("Body:\n- %s", string(request.Body))
		}
		// conn.Close()
	}
```

* **Erro técnico:** A chamada de fechamento do socket está comentada dentro de um laço contínuo de escuta. Cada nova conexão TCP aceita pelo listener permanece aberta indefinidamente no sistema operacional, esgotando recursos e impedindo que o cliente finalize sua ponta do handshake de encerramento.
* **Conceito para correção:** Certifique-se de que toda conexão aberta seja fechada ao final do seu ciclo de tratamento, utilizando o encerramento postergado ou explícito logo após o término do processamento.

---

### 2.7 Divergência no suporte a conexões persistentes (Keep-Alive)

**Arquivos:** `internal/server/server.go` e `internal/response/response.go`

* **Erro técnico:** O servidor atualmente injeta por padrão o cabeçalho `Connection: keep-alive` na resposta gerada em `response.go`. Entretanto, a função `handle` em `server.go` processa rigorosamente apenas uma requisição e fecha a conexão no `defer conn.Close()`. Há uma incoerência entre o que o servidor promete nos cabeçalhos HTTP e o que ele faz na camada de transporte TCP. Em HTTP/1.1, a conexão persistente é o padrão e exige que o servidor processe múltiplas requisições sequenciais sobre o mesmo socket até que um cabeçalho `Connection: close` seja recebido ou que ocorra um timeout de ociosidade.
* **Conceito para correção:** A função de tratamento de conexão deve operar em laço contínuo para a mesma conexão de rede. Esse laço deve continuar lendo e despachando requisições enquanto o cliente não sinalizar encerramento e enquanto a conexão estiver ativa. Deve-se configurar prazos de leitura (deadlines) para liberar conexões ociosas que parem de enviar dados.

---

### 2.8 Falta de suporte para respostas em streaming com `Transfer-Encoding: chunked`

**Arquivo:** `internal/response/response.go`

* **Erro técnico:** O servidor é incapaz de transmitir respostas cujo tamanho final não seja previamente conhecido (como no caso de arquivos gerados em tempo real ou transmissões contínuas). Atualmente, todas as respostas exigem o cálculo do comprimento exato (`Content-Length`) antes do envio dos cabeçalhos.
* **Conceito para correção:** Implemente um mecanismo na camada de resposta que omita o `Content-Length` e envie `Transfer-Encoding: chunked`. Crie funções auxiliares ou um encapsulador do escritor de rede que formate cada bloco de dados no formato de tamanho hexadecimal seguido do payload e delimitado por CRLF, concluindo com o bloco terminador de tamanho zero.

---

### 2.9 Estrutura de cabeçalhos engessada e prolixa

**Arquivo:** `internal/response/response.go` (linhas 77–113)

**Trecho atual:**
```go
func GetDefaultHeaders(contentLen int, contentType *string, connection *string, currDate *string, contentEncoding *string, cacheControl *string, server *string) headers.Headers
```

* **Erro técnico:** O uso de múltiplos ponteiros posicionais para tipos primitivos (`*string`) torna as chamadas verbosas, confusas e extremamente suscetíveis a erros de posicionamento de argumentos.
* **Conceito para correção:** Utilize técnicas canônicas da linguagem Go para lidar com parâmetros opcionais e configurações padrão. Duas abordagens adequadas são:
  1. Uma struct de configurações com campos opcionais, onde valores vazios assumem os padrões do sistema.
  2. O padrão de funções de opção (*functional options pattern*), permitindo chamadas declarativas e extensíveis sem poluir a assinatura da função.

---

### 2.10 Acoplamento de responsabilidades no pacote `main`

**Arquivo:** `cmd/httpserver/main.go`

* **Erro técnico:** O ponto de entrada da aplicação contém diretamente a implementação do roteamento, manipulação do sistema de arquivos e o handler padrão. Isso impede o reaproveitamento e dificulta a criação de testes automatizados para as regras de negócio de rotas sem inicializar o binário.
* **Conceito para correção:** Separe os componentes de infraestrutura dos de aplicação. Crie pacotes internos dedicados para os handlers e para o mecanismo de roteamento, mantendo no `main.go` apenas a composição das dependências, a configuração de portas e a captura de sinais do sistema operacional.

---

### 2.11 Resíduos e código sem uso

**Arquivo:** `internal/request/request.go`

* **Erro técnico:** Existem variáveis globais de compilação de expressões regulares e definições de erros que não são referenciadas em nenhum ponto do código (por exemplo, `isRequestLineValid` e `ErrMalformedMsg`). Expressões regulares compiladas globalmente sem utilização representam desperdício de inicialização e criam ruído na leitura do código.
* **Conceito para correção:** Remova declarações mortas ou, caso fossem destinadas a validações que ficaram incompletas, integre-as nas rotinas cabíveis.

---

## 3. Roadmap Priorizado de Implementação

Para otimizar o seu aprendizado sobre o funcionamento interno do protocolo HTTP sem ficar bloqueado por dependências entre componentes, siga as fases nesta ordem:

```
┌─────────────────────────────────────────────────────────────┐
│ FASE 1: Integridade Operacional e Correções Imediatas        │
│ (Buffer limpo em erros, resposta para parsing inválido)     │
└──────────────────────────────┬──────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────┐
│ FASE 2: Confiabilidade do Parser via Testes Unitários        │
│ (Casos de teste exaustivos para a máquina de estados chunked)│
└──────────────────────────────┬──────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────┐
│ FASE 3: Evolução de Roteamento e Arquivos Estáticos         │
│ (Métodos HTTP, status 405, sanitização de subdiretórios)    │
└──────────────────────────────┬──────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────┐
│ FASE 4: Recursos Avançados do Protocolo HTTP/1.1            │
│ (Respostas em streaming Chunked e laço de Keep-Alive)       │
└──────────────────────────────┬──────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────┐
│ FASE 5: Refatoração Estrutural e Boas Práticas em Go        │
│ (Organização de pacotes, structs de opções e limpeza)       │
└─────────────────────────────────────────────────────────────┘
```

---

### Fase 1: Integridade Operacional e Tratamento de Erros
*Prioridade: Máxima. O servidor não pode falhar silenciosamente nem enviar respostas corrompidas.*

1. **Isolar/Limpar buffer de erro no servidor:**
   - Garantir que erros de handlers nunca enviem bytes residuais que tenham sido gravados antes do erro ocorrer.
   - Manter consistência na serialização da resposta de erro em relação à de sucesso.
2. **Responder ao cliente em falhas de parsing da requisição:**
   - Em vez de fechar a conexão de imediato no `RequestFromReader`, mapear os erros de parsing e emitir uma resposta HTTP adequada (ex: 400 Bad Request) antes do encerramento.
3. **Corrigir fechamento de socket no listener de depuração:**
   - Descomentar/garantir o fechamento das conexões tratadas em `cmd/tcplistener/main.go`.

---

### Fase 2: Bateria de Testes do Parser (Consolidação da Máquina de Estados)
*Prioridade: Alta. O código de decodificação chunked já foi escrito, mas precisa ser comprovado antes de avançar para novas features.*

1. **Testes de decodificação chunked padrão:**
   - Validar decodificação com chunks de diferentes tamanhos e chunk final `0\r\n\r\n`.
2. **Testes de fragmentação extrema:**
   - Testar o comportamento da máquina de estados alimentada byte a byte via stream.
3. **Testes de validação de erro em chunked:**
   - Testar chunks corrompidos, ausência de terminadores CRLF e hexadecimais inválidos.
4. **Testes de cabeçalhos trailer:**
   - Validar a extração correta de headers enviados após o corpo chunked.

---

### Fase 3: Roteamento Semântico e Arquivos Estáticos
*Prioridade: Média-Alta. Prepara a camada de aplicação para comportar múltiplos métodos com conformidade às normas HTTP.*

1. **Roteamento ciente do método HTTP:**
   - Estruturar o roteador para validar a tupla (Método, URI).
   - Retornar 405 Method Not Allowed quando a rota existir para outro método HTTP.
2. **Sanitização segura de caminhos preservando subpastas:**
   - Substituir o achatamento de caminho por uma validação que permita subdiretórios legítimos sob a pasta base (ex: `static/`), rejeitando tentativas de subida de diretório (`..`).

---

### Fase 4: Recursos Avançados do Protocolo HTTP/1.1
*Prioridade: Média. Consolida o aprendizado prático de HTTP/1.1.*

1. **Escrita de resposta com Transfer-Encoding: chunked:**
   - Criar utilitário para envio de blocos hexadecimais e finalizadores para respostas em streaming onde o `Content-Length` não é conhecido.
2. **Ciclo de vida persistente (Keep-Alive):**
   - Implementar laço de persistência por conexão TCP no servidor.
   - Definir timeouts para requisições ociosas e respeitar o sinal de fechamento (`Connection: close`).

---

### Fase 5: Refatoração, Ergonomia e Código Limpo
*Prioridade: Baixa (Melhoria contínua de código).*

1. **Refatoração de cabeçalhos de resposta:**
   - Simplificar a função `GetDefaultHeaders` substituindo a sequência de ponteiros por uma struct dedicada ou padrão funcional.
2. **Modularização de pacotes:**
   - Mover a lógica de roteamento e os handlers específicos de `cmd/httpserver` para pacotes independentes sob o diretório `internal/`.
3. **Limpeza de código morto e logging consistente:**
   - Eliminar expressões regulares e erros sem uso em `request.go`.
   - Padronizar o uso de logging estruturado com a biblioteca padrão nos pontos onde ainda há uso de `log.Printf`.

---

## 4. Checklist Consolidado de Execução

Marque os itens conforme for implementando e testando na sua rotina de desenvolvimento:

### Fase 1: Integridade e Tratamento de Erros
- [ ] Tratar isolamento do buffer em `writeHandlerError` para evitar envio de dados parciais residuais
- [ ] Enviar resposta HTTP de erro (ex: 400 Bad Request) quando `RequestFromReader` falhar
- [ ] Garantir o encerramento do socket `conn` no laço de `cmd/tcplistener/main.go`

### Fase 2: Validação da Máquina de Estados (Testes)
- [ ] Criar teste de requisição chunked válida com múltiplos chunks em `request_test.go`
- [ ] Criar teste de requisição chunked com leitura fragmentada (1 byte por leitura)
- [ ] Criar teste de requisição chunked com trailer headers
- [ ] Criar testes para chunks inválidos (hexadecimal corrompido, CRLF faltando)

### Fase 3: Roteamento e Arquivos
- [ ] Implementar verificação de método HTTP no roteador
- [ ] Implementar resposta com status 405 Method Not Allowed
- [ ] Corrigir resolução de caminho em `fileHandler` para aceitar subdiretórios com segurança

### Fase 4: Features de Protocolo HTTP/1.1
- [ ] Criar mecanismo para envio de resposta com cabeçalho `Transfer-Encoding: chunked`
- [ ] Criar rotina para envio de chunks individuais e chunk terminador na resposta
- [ ] Implementar laço de leitura contínua na mesma conexão TCP para suporte a Keep-Alive
- [ ] Configurar timeout de inatividade para conexões persistentes

### Fase 5: Ergonomia e Organização
- [ ] Substituir parâmetros posicionais de `GetDefaultHeaders` por struct de configuração
- [ ] Extrair roteador e handlers de `cmd/httpserver/main.go` para pacotes em `internal/`
- [ ] Remover regex e erros não utilizados em `request.go`
- [ ] Padronizar logging com `slog` em `server.go` e listeners