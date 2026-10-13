# Make capability composition explicit

Closely related capabilities may coordinate through visible, opt-in APIs, as `service.WithAccount` does. Unrelated integrations and third-party adapters belong in separate packages, and core capabilities must not depend on optional adapters. This keeps useful workflows available without making cross-package coupling implicit or unrestricted.
