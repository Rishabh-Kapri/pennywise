# Transport Architecture Concept

## Overview
The architecture we're using in Pennywise for inter-service communication follows the **Strategy Pattern** paired with **Dependency Injection**. It decouples *business logic* from the *underlying network implementation*.

This is the exact same concept used in a layered architecture for databases (`Handler -> Service -> Repository`).

## The Core Concept
Instead of your business logic knowing how to make HTTP requests or parse URLs, it talks to a generic "Client". That generic Client holds a generic "Engine" (the `Transport` interface). 

### The Layers
1. **The Engine (The Transport Interface)**
   - Responsible strictly for sending bytes across a network and getting bytes back.
   - It doesn't know what the bytes represent. It doesn't know what an "Embedding" is, or what a "Prediction" is.
   - **Example Implementation:** `httpTransport` (in `httpTransport.go`), which uses `*http.Client` underneath and knows about HTTP Status Codes.

2. **The Generic Client (The Steering Wheel)**
   - Responsible for taking business payloads (structs), constructing a generic `Request` object, and sending it to the Engine.
   - Responsible for taking the generic `Response` bytes from the Engine and JSON unmarshaling them into the expected struct types.
   - **Implementation:** `transport.Client` with generic `Get[T]` and `Post[T]` wrappers.

3. **The Business Client (The Driver)**
   - Responsible for business logic. It relies purely on the Generic Client.
   - **Example Implementation:** `OllamaClient` or `MLPClient`.

## Why Build It This Way? (Inversion of Control)

Think about your database code. Your `TransactionService` does not know you are using PostgreSQL. It just expects an interface that can `Create()` or `Get()`. If you switch to MySQL, you just pass a different implementation of that interface in `main.go`.

This `Transport` concept achieves the same thing for external APIs!

Your `OllamaClient` currently relies on `transport.Client`. It has **no idea** that you are using HTTP. If Ollama deprecates its HTTP API tomorrow and switches to WebSockets, you simply write a `websocketTransport`, and swap it in `main.go`:

```go
// Today: We spin up an HTTP engine
engine := transport.NewHttpTransport("http://localhost:11434")

// Tomorrow: We spin up a gRPC engine instead
// engine := transport.NewGrpcTransport("localhost:50051")

// Wraps the engine in our generic client
ollamaGenericClient := transport.NewClient("ollama", engine)

// Pass it to the business logic
ollamaBusinessClient := client.NewOllamaClient(ollamaGenericClient)
```

**Zero lines of code in your business logic (`client/ollama.go` or `handler.go`) had to change!**

## Visualizing the Flow

1. You call `err := ollamaClient.Embed(ctx, "text")`
2. `OllamaClient` says: "I need to hit `/api/embed`. Generic Client, please `Post` this payload!" (It calls `c.client.Post[[]float64](...)`)
3. `Generic Client` says: "I will marshal this payload to bytes, and hand a generic Request to whatever Engine I was given." (Calls `c.transport.Send()`)
4. `httpTransport` (The Engine) says: "I know HTTP! I will attach the Base URL, inject context headers like `X-Correlation-ID`, and do an `http.Post`."
5. `httpTransport` returns raw response bytes.
6. `Generic Client` unmarshals the response bytes into the output struct provided by `OllamaClient`.
