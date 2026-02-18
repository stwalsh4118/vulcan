# Chi Router Package Guide

**Date**: 2026-02-18
**Package**: `github.com/go-chi/chi/v5`
**Docs**: https://pkg.go.dev/github.com/go-chi/chi/v5

## API Usage

### Router Creation & Middleware

```go
r := chi.NewRouter()
r.Use(middleware.RequestID)  // Injects X-Request-Id
r.Use(middleware.Recoverer)  // Catches panics, returns 500
```

### Routes

```go
r.Get("/path", handler)
r.Post("/path", handler)
r.Delete("/path", handler)

// URL params
r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
})

// Route groups
r.Route("/v1", func(r chi.Router) {
    r.Get("/items", listItems)
    r.Post("/items", createItem)
})
```

### Middleware Pattern

```go
func MyMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // before
        next.ServeHTTP(w, r)
        // after
    })
}
```

### CORS (go-chi/cors)

```go
r.Use(cors.Handler(cors.Options{
    AllowedOrigins:   []string{"*"},
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Accept", "Content-Type"},
    AllowCredentials: false,
    MaxAge:           300,
}))
```
