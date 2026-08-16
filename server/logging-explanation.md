# Logging Strategy Keynote

## Core Philosophy: "Log Once, Log Right"

Each error is logged **once** at the layer where it originates or has the most context. This eliminates redundant logging while maintaining excellent observability.

## Layer-Specific Logging Responsibilities

### 🗄️ **Storage Layer** - Database & Infrastructure Concerns
**What we log:**
- Database connection errors
- Query execution failures
- Transaction errors
- Constraint violations (unexpected ones)

**What we DON'T log:**
- `sql.ErrNoRows` (expected behavior)
- Unique constraint violations for business logic (like duplicate users)

**Key logging details:**
- **Query text**: Helps debug slow/failing queries
- **Operation name**: Categorizes the type of database operation
- **Parameters**: User ID, username, email (for context)
- **PostgreSQL error codes**: Helps identify specific database issues

### 🔧 **Service Layer** - Business Logic & Coordination
**What we log:**
- Transaction management errors (begin/commit failures)
- Business rule violations (accessing soft-deleted users)
- Successful business operations
- Duplicate creation attempts

**What we DON'T log:**
- Database errors (already logged in storage layer)
- Simple pass-through operations

**Key logging details:**
- **Business context**: User ID, username, email, display name
- **Operation type**: create_user, update_user_profile, etc.
- **Success events**: User created, profile updated (for audit trails)

### 🌐 **Handler Layer** - HTTP & Request/Response Concerns
**What we log:**
- Authentication/middleware failures
- Request parsing errors (JSON, validation)
- HTTP response writing failures
- Successful operations (optional, for request tracing)

**What we DON'T log:**
- Service/business logic errors (already logged in lower layers)
- Expected HTTP errors (404s from business logic)

**Key logging details:**
- **HTTP context**: Path, method, status code, content type
- **Request details**: User ID from auth, request parameters
- **Response issues**: Failed to write response, serialization errors

## Log Level Strategy

### 🔴 **ERROR** - System/Infrastructure Problems
- Database connection failures
- Transaction commit/rollback failures
- HTTP response writing failures
- Authentication middleware errors
- Unexpected constraint violations

### 🟡 **WARN** - Expected Problems & Business Rules
- Invalid JSON payloads
- Validation failures
- No rows affected in updates
- Malformed requests

### 🔵 **INFO** - Important Business Events
- User created successfully
- Profile updated successfully
- Audit trail events

### 🟢 **DEBUG** - Development & Tracing
- Request received
- Business rule checks (soft deleted users)
- Request/response tracing

## Detailed Logging Examples

### Storage Layer Example
```go
store.logger.Error("database query failed", 
    "operation", "get_user_by_id",      // What operation failed
    "userID", id,                       // Business context
    "query", query,                     // Exact SQL for debugging
    "error", err,                       // Original error
)
```

### Service Layer Example  
```go
s.logger.Error("failed to begin transaction", 
    "operation", "create_user",         // Business operation
    "username", user.Username,          // Business context
    "email", user.Email,               // More business context
    "error", err,                      // Original error
)
```

### Handler Layer Example
```go
h.logger.Error("authentication middleware error", 
    "error", err,                      // Original error
    "path", r.URL.Path,               // HTTP context
    "method", r.Method,               // HTTP context
)
```

## Benefits of This Approach

### ✅ **Reduced Noise**
- No duplicate error messages
- Only relevant information at each layer
- Clear separation of concerns

### ✅ **Better Debugging**
- SQL queries logged when they fail
- Business context preserved
- HTTP context available for request tracing

### ✅ **Improved Monitoring**
- Clear distinction between expected vs unexpected errors
- Structured logging for easy parsing
- Audit trail for business events

### ✅ **Performance**
- Less logging overhead
- Reduced log storage requirements
- Faster log processing

## Error Flow Example

```
1. Storage Layer: Database query fails
   → LOG: "database query failed" with query, params, error
   → RETURN: apperrors.ErrInternalServer

2. Service Layer: Receives ErrInternalServer
   → NO LOG: Error already logged
   → RETURN: