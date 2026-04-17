<div align="center">
  <img src="https://github.com/user-attachments/assets/995be885-ec69-4d17-b177-8105eb7c09da" height="350" alt="VolcAPI" />
  <p><strong>OpenAPI-Native API Testing Tool Built in Go</strong></p>
</div>

**Your OpenAPI spec is already your test suite. You just can't run it yet.**
 
VolcAPI is a Go CLI that makes OpenAPI specs executable — define test scenarios directly inside your spec, run them from the terminal or CI, get pass/fail output. No separate Postman collection to maintain. No extra config files to sync.

---

```bash
volcapi run volcapi_local.yml -o openapi.yml
```
 
---
 
> ⚠️ **This is an early alpha.** Core GET/POST/PUT/DELETE testing works. CI output formats (JUnit XML, JSON) are in active development. If the problem resonates with you, star/watch the repo — that directly helps me prioritize.
 
---
## The problem
 
You maintain your API in two places:
 
1. The **OpenAPI spec** — for docs, codegen, and communication
2. A **Postman collection / test script** — for actual validation
They drift. They contradict each other. You update one and forget the other. Every developer on your team has a different version of the collection locally.
 
VolcAPI collapses this into one file. Your spec defines the API. Your spec runs the tests.
 
---


### Technical Architecture: my vision
```
┌─────────────────────────────────────────────────────────┐
│                    OpenAPI Spec                         │
│  (Single Source of Truth: API Definition + Tests)       │
└─────────────────┬───────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────┐
│               APIFlow CLI (Go)                          │
│  • Parse OpenAPI + test scenarios                       │
│  • Load environment config (local/staging/prod)         │
│  • Execute API requests                                 │
│  • Validate responses                                   │
│  • Generate reports                                     │
└─────────────────┬───────────────────────────────────────┘
                  │
      ┌───────────┼───────────┬──────────────┐
      ▼           ▼           ▼              ▼
  ┌───────┐  ┌─────────┐  ┌─────────┐   ┌──────────┐
  │  CLI  │  │  Web    │  │ Grafana │   │  Slack   │
  │Output │  │Dashboard│  │ Metrics │   │  Alerts  │
  └───────┘  └─────────┘  └─────────┘   └──────────┘
```
---

## How it works
 
Add a `v-functional-test` extension to any endpoint in your spec:
 
```yaml
paths:
  /auth/login:
    post:
      summary: User login
      responses:
        '200':
          description: Success
      v-functional-test:
        scenarios: ["valid_login", "wrong_password"]
```
 
Define scenarios at the root level of the same spec (or in a separate config):
 
```yaml
scenarios:
  valid_login:
    headers:
      Content-Type: application/json
    request:
      email: user@example.com
      password: password123
    response:
      status: 200
      body:
        contains: ["token", "user"]
 
  wrong_password:
    request:
      email: user@example.com
      password: wrong
    response:
      status: 401
```
 
Run it:
 
```bash
volcapi run volcapi_local.yml -o openapi.yml
```
 
That's it. Your spec is now a test runner.
 
---
 
## Environment config
 
Point at different hosts per environment. Use environment variables for secrets:
 
```yaml
# volcapi_local.yml
host: http://localhost:3000
env:
  API_TOKEN: your_local_token
 
# volcapi_staging.yml
host: https://staging.api.example.com
env:
  API_TOKEN: ${STAGING_TOKEN}  # from CI secrets
```
 
---
 
## Installation
 
**Requires Go 1.21+**
 
```bash
git clone https://github.com/aliamerj/volcapi.git
cd volcapi
go build -o volcapi
sudo mv volcapi /usr/local/bin/
```
 
---
 
## What works today
 
| Feature | Status |
|---|---|
| OpenAPI 3.x parsing | ✅ |
| GET / POST / PUT / DELETE | ✅ |
| Response status validation | ✅ |
| JSON body validation (exact + contains) | ✅ |
| Environment variable substitution | ✅ |
| Multiple environments (local/staging/prod) | ✅ |
| JUnit XML output (for GitHub Actions) | 🔨 In progress |
| JSON output format | 🔨 In progress |
| GitHub Actions example | 🔨 In progress |
 
---
 
## What's next (and why it matters)
 
The immediate goal is making VolcAPI a first-class CI/CD tool:
 
- **JUnit XML output** — so GitHub Actions, GitLab CI, and Jenkins can parse results natively
- **GitHub Actions workflow** — copy-paste ready, zero config
- **Contract validation** — fail CI when your API response no longer matches what the spec says it returns
Longer term: gRPC support, performance testing, eBPF-based profiling. But those come after the CI story is solid.
 
---
 
## Why Go?
 
Most API testing tooling is JavaScript or Python. Go gives you:
 
- A single static binary — no runtime, no `node_modules`, no virtualenv
- Fast startup — matters in CI where you're paying per second
- Easy to distribute — `go install` or download a binary, done
---



## 🔧 CLI Reference

### Commands

```bash
# Run tests from config file
volcapi run <config-path> [flags]

# Run with OpenAPI spec
volcapi run volcapi_local.yml -o openapi.yml

# Run from remote URL
volcapi run https://example.com/volcapi_local.yml -o openapi.yml
```

### Flags

- `-o, --openapi <path>`: Path to OpenAPI specification file
- `-h, --help`: Show help for commands

---

## 💡 Examples

### Example 1: Simple GET Request

```yaml
# openapi.yml
paths:
  /get:
    get:
      summary: Echo request
      responses:
        '200':
          description: OK
      v-functional-test:
        scenarios: ["simple_get"]

scenarios:
  simple_get:
    headers:
      Accept: application/json
    response:
      status: 200
      body:
        contains: ["headers.Host"]
```

### Example 2: GET with Query Parameters

```yaml
scenarios:
  get_with_query:
    query:
      id: 132
      filter: active
    headers:
      Accept: application/json
    response:
      status: 200
      body:
        json:
          args:
            object:
              id:
                value: "132"
              filter:
                value: "active"
```

### Example 3: POST with Authentication

```yaml
scenarios:
  create_user:
    headers:
      Content-Type: application/json
      Authorization: Bearer ${TOKEN}
    request:
      name: "Alice"
      email: "alice@example.com"
    response:
      status: 201
      body:
        contains: ["id", "name", "email"]

env:
  TOKEN: your_api_token
```

---

## 🚀 CI/CD Integration

### GitHub Actions (Coming Soon)

```yaml
name: API Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Install VolcAPI
        run: go install github.com/aliamerj/volcapi@latest
      
      - name: Run API Tests
        env:
          STAGING_TOKEN: ${{ secrets.STAGING_TOKEN }}
        run: volcapi run volcapi_staging.yml -o openapi.yml
```
---

## Contributing
 
The project is early and the roadmap is shaped by real feedback. If you hit a bug, want a feature, or just want to say "yes this problem is real" — open an issue. That feedback directly influences what gets built next.
 
```bash
git clone https://github.com/aliamerj/volcapi.git
cd volcapi
go mod download
go test ./...
go build -o volcapi
./volcapi run volcapi_local.yml -o openapi.yml
```

---

## 📝 License

MIT License - see [LICENSE](LICENSE) file for details

---

## 🌟 Show Your Support

If VolcAPI helps you, please:
- ⭐ Star this repository
- 🐦 Share it on social media
- 📝 Write about your experience
- 🗣️ Tell your team

---

## 🙏 Acknowledgments

Inspired by:
- [K6](https://k6.io/) - Performance testing excellence
- [Bruno](https://www.usebruno.com/) - Git-friendly API testing
- [OpenAPI](https://www.openapis.org/) - API specification standard

Built with ❤️ for developers who value simplicity and speed.

---

**Status**: 🚧 Early Development - v0.1.0-alpha

**Current Features**: Basic GET request testing with OpenAPI validation

Star ⭐ this repo to follow our progress!
