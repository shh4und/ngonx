# ngonx

Servidor HTTP / Listener TCP e UDP desenvolvido do zero em Go para fins educacionais, baseado no curso **"Learn the HTTP Protocol"** do **Boot.dev**.

O objetivo principal do projeto é entender o funcionamento interno de redes e do protocolo HTTP em baixo nível — manipulando sockets TCP/UDP brutos, fazendo o parse manual de requisições HTTP (Status Line, Headers, Body) e construindo respostas HTTP padronizadas sem utilizar abstrações de alto nível (como a biblioteca `net/http` do Go para roteamento e parsing).

---

## 🚀 Tecnologias e Bibliotecas

- **Linguagem:** Go (Golang)
- **Biblioteca Padrão:** `net`, `io`, `fmt`, `bytes`, etc.
- **Testes:** `github.com/stretchr/testify` (única dependência externa, utilizada apenas para asserções nos testes unitários)

---

## 📁 Estrutura do Projeto

```text
.
├── cmd/
│   ├── httpserver/       # Ponto de entrada da aplicação HTTP Server
│   │   └── main.go
│   ├── tcplistener/      # Listener TCP básico para testes de conexão de baixo nível
│   │   └── main.go
│   ├── udpreceiver/      # Receptor de pacotes UDP
│   │   └── main.go
│   └── udpsender/        # Emissor de pacotes UDP
│       └── main.go
├── internal/
│   ├── headers/          # Manipulação e parsing de HTTP Headers (key-value)
│   │   ├── headers.go
│   │   └── headers_test.go
│   ├── request/          # Parsing e representação de requisições HTTP/1.1
│   │   ├── request.go
│   │   └── request_test.go
│   ├── response/         # Construção e serialização de respostas HTTP/1.1
│   │   └── response.go
│   └── server/           # Loop de aceitação de conexões e gerenciamento do socket TCP
│       └── server.go
├── static/               # Arquivos estáticos servidos pelo HTTP Server
│   └── landing.html
├── messages.txt          # Mensagens/logs de teste
├── go.mod
└── go.sum
```
