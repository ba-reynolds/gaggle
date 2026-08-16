# Setting Up Swagger Documentation in Go Projects

## Installation

```bash
# Install Swagger CLI tool
go install github.com/swaggo/swag/cmd/swag@latest

# Add required dependencies
go get -u github.com/swaggo/http-swagger
go get -u github.com/swaggo/swag
```

## Configuration Steps

### 1. Main Entry Point (main.go)

Add the docs import and general API annotations:

```go
package main

import (
    // This will import the .go file inside of /docs
    _ "yourproject/docs" // Import the docs
)

// @title         Your API Title
// @version       1.0
// @description   Description of your API
// @host          localhost:yourport
// @BasePath      /api/v1
// @schemes       http
func main() {
    // ... your main code
}
```

### 2. Router Configuration

```go
import (
    swagger "github.com/swaggo/http-swagger"
)

// In your router setup:
r.Get("/swagger/*", swagger.Handler(
    swagger.URL("/api/v1/swagger/doc.json"),
    swagger.DocExpansion("none"),
    swagger.DeepLinking(true),
    swagger.DomID("swagger-ui"),
))
```

### 3. Handler Annotations Example

```go
// @Summary     Health check endpoint
// @Description Get API health status
// @Tags        health
// @Accept      json
// @Produce     json
// @Success     200 {object} map[string]string
// @Router      /health [get]
func (app *Application) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
    // ... handler implementation
}
```

## Generate Documentation

```bash
swag init -g cmd/api/main.go --parseDependency --output docs
```

Make sure to add this to `.air.toml` as
```
  pre_cmd = ["make gen-docs"]
```

## Access Points
- Swagger UI: `http://localhost:yourport/api/v1/swagger/index.html`
- Swagger JSON: `http://localhost:yourport/api/v1/swagger/doc.json`

## Important Notes
1. Regenerate documentation (`swag init`) after adding new annotations
2. Restart your application after regenerating docs
3. Make sure your API's base path matches the one in Swagger configuration
4. Keep annotations up to date with your actual implementation