# AGENTS.md

`llmapi` implements a layered architecture where a unified Go interface (`Provider`) abstracts external LLM services. This core abstraction is wrapped by a networking layer that multiplexes HTTP and gRPC protocols on a single port.

## Directory Implementation Map

*   **`models.go`**: Domain Root. Contains the `Models` struct which holds the state of all active providers.
*   **`internal/providers/`**:
    *   `provider/`: Defines the strict contract (`interface`) all plugins must adhere to.
    *   `openai/`, `anthropic/`, etc.: Concrete implementations. These packages contain the translation logic from the unified internal types to the vendor-specific SDK calls.
    *   `providers.go`: The central registry (Lookup Table) linking string names to constructor functions.
*   **`internal/server/`**:
    *   `server.go`: Bootstraps the `cmux` listener and orchestrates the shutdown signal.
    *   `api/v1/`: Contains the business logic for endpoints. This is where the request validation happens before calling the SDK.
*   **`api/`**: Contains `buf` configuration and `.proto` files defining the external contract.
