# VolcAPI Usage

This guide documents the current behavior of VolcAPI based on the code in this repository.

## Run

```bash
./volcapi run volcapi_local.yml -o openapi.yml
```

`volcapi_local.yml`

```yaml
host: http://localhost:3000
env:
  TOKEN: your_token_here
```

`openapi.yml`

```yaml
openapi: 3.0.3
info:
  title: Demo API
  version: 1.0.0

scenarios:
  health_check:
    response:
      status: 200

paths:
  /health:
    get:
      v-functional-test:
        scenarios: ["health_check"]
```

## Scenario Fields

Each scenario can use these fields:

```yaml
scenarios:
  example:
    params:
      id: "42"
    query:
      page: "1"
    headers:
      Authorization: Bearer $TOKEN
    request:
      json:
        name: ali
      # or:
      # text: raw request body
    response:
      status: 200
      body:
        contains: ["data.id", "data.name", "users[0].email"]
        text: success
        data:
          id:
            value: "42"
          name:
            value: ali
```

## Cases

### 1. Status Code Only

```yaml
scenarios:
  list_users:
    response:
      status: 200
```

### 2. Path Params

```yaml
scenarios:
  get_user:
    params:
      id: "42"
    response:
      status: 200

paths:
  /users/{id}:
    get:
      v-functional-test:
        scenarios: ["get_user"]
```

### 3. Query Params

```yaml
scenarios:
  search_users:
    query:
      page: "1"
      role: admin
    response:
      status: 200
```

### 4. Headers

```yaml
scenarios:
  auth_check:
    headers:
      Accept: application/json
      Authorization: Bearer $TOKEN
    response:
      status: 200
```

Environment values support both `$TOKEN` and `${TOKEN}`.

### 5. JSON Request Body

```yaml
scenarios:
  create_user:
    headers:
      Content-Type: application/json
    request:
      json:
        name: Alice
        email: alice@example.com
    response:
      status: 201
```

### 6. Raw Text Request Body

```yaml
scenarios:
  send_plain_text:
    headers:
      Content-Type: text/plain
    request:
      text: hello from volcapi
    response:
      status: 200
```

### 7. `body.contains`

`body.contains` checks that a JSON path exists.
It is a JSON assertion, so it only runs when the response `Content-Type` includes `application/json`.

```yaml
scenarios:
  profile_loaded:
    response:
      status: 200
      body:
        contains: ["user.id", "user.email", "roles[0]"]
```

### 8. `body.text`

`body.text` checks that the raw response body contains a substring.
It works regardless of `Content-Type`.

```yaml
scenarios:
  html_contains_title:
    response:
      status: 200
      body:
        text: "<title>Dashboard</title>"
```

### 9. Scalar `body`

Use a scalar `body` when the whole response body should match exactly.
This is a raw body comparison, not a JSON field assertion.

```yaml
scenarios:
  get_text_body:
    response:
      status: 200
      body: "hello world"

  get_boolean:
    response:
      status: 200
      body: true

  get_number:
    response:
      status: 200
      body: 123
```

This also works when the body is JSON text but you want to compare the raw payload exactly:

```yaml
scenarios:
  get_broken_json:
    response:
      status: 200
      body: "{\"message\":\"This is JSON but content-type is wrong\"}"
```

### 10. Body Field Checks

Top-level `body` defaults to an object schema.

```yaml
scenarios:
  current_user:
    response:
      status: 200
      body:
        id:
          value: "42"
        active:
          value: true
```

Use bare `true` when you only want to assert that a field exists:

```yaml
scenarios:
  current_user:
    response:
      status: 200
      body:
        id: true
        email: true
```

Use `value: true` when you want an exact boolean value:

```yaml
scenarios:
  current_user:
    response:
      status: 200
      body:
        active:
          value: true
```

### 11. Nested Objects

Top-level `body` defaults to an object schema. Use `object` only for nested JSON objects.

```yaml
scenarios:
  nested_user:
    response:
      status: 200
      body:
        user:
          object:
            profile:
              object:
                email:
                  value: ali@example.com
```

### 12. Arrays

Use `body.list` for a top-level array. A plain mapping inside `list` means "array of objects" by default.

```yaml
scenarios:
  users_list:
    response:
      status: 200
      body:
        list:
          id: true
          name: true
```

For nested arrays, keep using `list` on the field:

```yaml
scenarios:
  users_in_data:
    response:
      status: 200
      body:
        data:
          list:
            id: true
            email: true
```

## Notes

- Request methods come from your OpenAPI paths: `get`, `post`, `put`, `delete`, `patch`, etc.
- Scenarios can live in the main config file, the OpenAPI file, or both.
- If a scenario exists in both places, the OpenAPI scenario overwrites the config one.
- Scalar `body` matches the full raw response body exactly after trimming surrounding whitespace.
- JSON assertions (`body.contains`, body field checks, nested `object`, and `list`) require the response `Content-Type` to contain `application/json`.
- `body.contains` also works with dotted and indexed paths like `items[0].id`.

## Working Example

See [`examples/openapi.yml`](/home/ali/Projects/Volc/volcapi/examples/openapi.yml) and [`examples/volcapi_local.yml`](/home/ali/Projects/Volc/volcapi/examples/volcapi_local.yml).
