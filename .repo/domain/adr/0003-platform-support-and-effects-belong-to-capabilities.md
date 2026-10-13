# Keep platform support and mutations with each capability

Each capability declares its own platform support; the module has no single support matrix. `host` remains read-only, while capabilities that mutate operating-system state own those operations and their authorization checks. This allows broad host inspection without implying that every mutation is available on every platform.
