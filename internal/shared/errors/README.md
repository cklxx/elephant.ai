# Error Handling System

Production-grade error recovery with retry logic and circuit breakers for ALEX.

## Overview

The error handling system provides:

1. **Error Classification** - Automatic detection of transient vs permanent errors
2. **Retry Logic** - Exponential backoff with jitter for transient failures
3. **Circuit Breakers** - Prevent cascading failures in distributed systems
4. **LLM-Friendly Errors** - Convert technical errors to actionable messages

## Components

### 1. Error Classification (`types.go`)

Classifies errors into three types:

- **TransientError** - Retry-able (network timeout, 429 rate limit, 500 server error)
- **PermanentError** - Non-retry-able (401 unauthorized, 400 bad request, invalid input)
- **DegradedError** - Can continue with reduced functionality

```go
// Check if error is transient (retry-able)
if errors.IsTransient(err) {
    // Retry operation
}

// Check if error is permanent (don't retry)
if errors.IsPermanent(err) {
    // Report error and stop
}

// Get error type
errorType := errors.GetErrorType(err)

// Format error for LLM (human-friendly message)
llmMessage := errors.FormatForLLM(err)
```

#### Automatic Detection

Transient errors are automatically detected:
- HTTP 429 (rate limit), 500, 502, 503, 504
- Network timeouts (`context deadline exceeded`)
- Connection errors (`connection refused`, `connection reset`)
- DNS errors
- Syscall errors (ECONNREFUSED, ETIMEDOUT, etc.)

Permanent errors are automatically detected:
- HTTP 400, 401, 403, 404
- "not found", "permission denied", "invalid"
- Tool/resource not found

#### Custom Error Wrapping

```go
// Create custom transient error
err := errors.NewTransientError(
    originalErr,
    "API rate limit reached. Retrying with backoff.",
)

// Create custom permanent error
err := errors.NewPermanentError(
    originalErr,
    "Authentication failed. Please check your API key.",
)

// Create degraded error with fallback
err := errors.NewDegradedError(
    originalErr,
    "Search API unavailable. Using cached results.",
    cachedResults,
)
```

### 2. Retry Logic (`retry.go`)

Exponential backoff retry with configurable parameters:

```go
config := errors.RetryConfig{
    MaxAttempts:  3,                    // Total attempts: 4 (1 initial + 3 retries)
    BaseDelay:    1 * time.Second,      // Base delay
    MaxDelay:     30 * time.Second,     // Cap maximum delay
    JitterFactor: 0.25,                 // ±25% randomization
}

// Retry a function
err := errors.Retry(ctx, config, func(ctx context.Context) error {
    return doSomething()
})

// Retry with result
result, err := errors.RetryWithResult(ctx, config, func(ctx context.Context) (string, error) {
    return fetchData()
})
```

#### Backoff Schedule

With default config (base=1s, max=30s):
- Attempt 1: immediate
- Attempt 2: ~1s delay (with jitter)
- Attempt 3: ~2s delay (with jitter)
- Attempt 4: ~4s delay (with jitter)
- Maximum: 30s delay

#### Context Cancellation

Retry respects context cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

err := errors.Retry(ctx, config, fn)
// Will stop retrying if context is cancelled
```

### 3. Circuit Breaker (`circuit_breaker.go`)

Prevents cascading failures with state machine:

```
     5 failures          30s timeout         2 successes
CLOSED ─────────────▶ OPEN ─────────────▶ HALF-OPEN ─────────────▶ CLOSED
                        │                      │
                        │                      │ any failure
                        │                      ▼
                        └─────────────────── OPEN
```

#### States

- **Closed** - Normal operation, requests allowed
- **Open** - Too many failures, requests blocked
- **Half-Open** - Testing if service recovered

#### Configuration

```go
config := errors.CircuitBreakerConfig{
    FailureThreshold: 5,                // Open after 5 consecutive failures
    SuccessThreshold: 2,                // Close after 2 consecutive successes
    Timeout:          30 * time.Second, // Wait 30s before trying half-open
}

cb := errors.NewCircuitBreaker("my-service", config)
```

#### Usage

```go
// Simple execution
err := cb.Execute(ctx, func(ctx context.Context) error {
    return callExternalAPI()
})

// With result (using helper function)
result, err := errors.ExecuteFunc(cb, ctx, func(ctx context.Context) (string, error) {
    return fetchFromAPI()
})
```

#### Circuit Breaker Manager

Manage multiple circuit breakers:

```go
manager := errors.NewCircuitBreakerManager(config)

// Get circuit breaker for service (creates if not exists)
cb := manager.Get("external-api")

// Get all metrics
metrics := manager.GetMetrics()

// Reset all circuit breakers
manager.ResetAll()
```

### 4. LLM Error Formatting

Convert technical errors to actionable messages:

```go
// Technical: "dial tcp 127.0.0.1:11434: connect: connection refused"
// LLM-friendly: "Ollama server is not running. Please start it with: ollama serve"

// Technical: "429 Too Many Requests: Rate limit exceeded"
// LLM-friendly: "API rate limit reached. Waiting 60s before retry. Consider using a cheaper model."

// Technical: "context deadline exceeded"
// LLM-friendly: "Request timed out after 60s. Try breaking it into smaller steps."

message := errors.FormatForLLM(err)
```

## Integration with ALEX

### LLM Client Wrapper

All LLM clients are automatically wrapped with retry and circuit breaker:

```go
// In internal/llm/factory.go
factory := llm.NewFactory()  // Retry enabled by default

client, err := factory.GetClient("openrouter", "deepseek/deepseek-chat", config)
// Client automatically has:
// - Retry logic with exponential backoff
// - Circuit breaker protection
// - LLM-friendly error messages
```

Configuration can be customized:

```go
retryConfig := errors.RetryConfig{
    MaxAttempts:  5,
    BaseDelay:    2 * time.Second,
    MaxDelay:     60 * time.Second,
    JitterFactor: 0.25,
}

circuitConfig := errors.CircuitBreakerConfig{
    FailureThreshold: 10,
    SuccessThreshold: 3,
    Timeout:          60 * time.Second,
}

factory := llm.NewFactoryWithRetryConfig(retryConfig, circuitConfig)
```

### Error Flow

```
User Request
    ↓
ReAct Engine
    ↓
LLM Client (with retry wrapper)
    ↓
Circuit Breaker Check
    ↓
Retry Logic (exponential backoff)
    ↓
Underlying LLM Client (OpenAI, DeepSeek, Ollama)
    ↓
Error Classification
    ↓
LLM-Friendly Error Message
    ↓
Back to User
```

## Best Practices

### 1. Error Classification

Always classify errors explicitly when creating them:

```go
// Good
return errors.NewTransientError(err, "Service temporarily unavailable")

// Bad (relying on automatic detection)
return fmt.Errorf("error: %w", err)
```

### 2. Context Usage

Always pass context for timeout and cancellation:

```go
// Good
err := errors.Retry(ctx, config, fn)

// Bad
err := errors.Retry(context.Background(), config, fn)
```

### 3. Circuit Breaker Naming

Use consistent, descriptive names:

```go
// Good
cb := manager.Get("external-api-tavily")
cb := manager.Get("llm-openrouter")
cb := manager.Get("database-postgres")

// Bad
cb := manager.Get("api1")
cb := manager.Get("service")
```

### 4. Retry Configuration

Match retry configuration to use case:

```go
// Fast operations (file I/O)
config := errors.RetryConfig{
    MaxAttempts:  2,
    BaseDelay:    100 * time.Millisecond,
    MaxDelay:     1 * time.Second,
}

// Slow operations (LLM API)
config := errors.RetryConfig{
    MaxAttempts:  3,
    BaseDelay:    1 * time.Second,
    MaxDelay:     30 * time.Second,
}

// Critical operations (database)
config := errors.RetryConfig{
    MaxAttempts:  5,
    BaseDelay:    2 * time.Second,
    MaxDelay:     60 * time.Second,
}
```

## Testing

Run all error handling tests:

```bash
go test ./internal/errors/ -v
```

Run specific test:

```bash
go test ./internal/errors/ -v -run TestRetry
```

Run with coverage:

```bash
go test ./internal/errors/ -cover
```

## Observability

The error handling system logs important events:

- **DEBUG**: Each retry attempt with delay
- **INFO**: Successful retry after failures
- **WARN**: Circuit breaker opened, max retries exhausted
- **ERROR**: Permanent errors, circuit breaker failures

Circuit breaker state changes can be monitored:

```go
config := errors.CircuitBreakerConfig{
    OnStateChange: func(from, to errors.CircuitState, name string) {
        log.Printf("Circuit %s: %s -> %s", name, from, to)
        // Send to metrics/monitoring system
    },
}
```

## Examples

### Example 1: Retry Network Request

```go
result, err := errors.RetryWithResult(ctx, errors.DefaultRetryConfig(),
    func(ctx context.Context) (string, error) {
        resp, err := http.Get("https://api.example.com/data")
        if err != nil {
            return "", err
        }
        defer resp.Body.Close()

        if resp.StatusCode == 429 {
            return "", errors.NewTransientError(
                fmt.Errorf("rate limited"),
                "API rate limit reached",
            )
        }

        body, err := io.ReadAll(resp.Body)
        return string(body), err
    },
)
```

### Example 2: Circuit Breaker for External API

```go
manager := errors.NewCircuitBreakerManager(errors.DefaultCircuitBreakerConfig())
cb := manager.Get("external-api")

result, err := errors.ExecuteFunc(cb, ctx,
    func(ctx context.Context) (Response, error) {
        return callExternalAPI()
    },
)

if errors.IsDegraded(err) {
    // Circuit is open, use fallback
    result = getCachedResponse()
}
```

### Example 3: Combined Retry and Circuit Breaker

```go
cb := errors.NewCircuitBreaker("service", errors.DefaultCircuitBreakerConfig())
retryConfig := errors.DefaultRetryConfig()

result, err := errors.RetryWithResult(ctx, retryConfig,
    func(ctx context.Context) (string, error) {
        return errors.ExecuteFunc(cb, ctx,
            func(ctx context.Context) (string, error) {
                return fetchData()
            },
        )
    },
)
```

## Philosophy

The error handling system follows these principles:

1. **Fail Fast for Permanent Errors** - Don't waste time retrying unrecoverable errors
2. **Retry Transient Errors** - Network blips happen, retry with backoff
3. **Protect the System** - Circuit breakers prevent cascading failures
4. **User-Friendly Messages** - Convert technical errors to actionable guidance
5. **Respect Context** - Always honor cancellation and timeouts
6. **Observable** - Log and expose metrics for monitoring

## Future Enhancements

Planned improvements:

1. Adaptive retry delays based on Retry-After headers
2. Bulkhead pattern for resource isolation
3. Fallback strategies (cache, default values)
4. Metrics integration (Prometheus, OpenTelemetry)
5. Distributed tracing with error context
6. Configuration from YAML/environment variables

## References

- [Microsoft - Retry Pattern](https://docs.microsoft.com/en-us/azure/architecture/patterns/retry)
- [Microsoft - Circuit Breaker Pattern](https://docs.microsoft.com/en-us/azure/architecture/patterns/circuit-breaker)
- [AWS - Exponential Backoff and Jitter](https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/)
- [Google SRE - Handling Overload](https://sre.google/sre-book/handling-overload/)
