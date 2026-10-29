# Keep shared foundations small and stable

Public foundational packages should expose stable concepts and mechanisms used across capabilities, not abstractions created only to remove duplication. Reuse implementation-only helpers through module-private `internal` packages, and keep contextual errors in their owning capabilities while using `errx` for optional transport-neutral classification. This prevents unrelated policy from leaking into shared APIs.
