# certs

`certs` manages a product-local mTLS hierarchy with one supported shape:

```text
root CA -> issuer CA -> leaf certificate
```

Authority changes are explicit. `Ensure` provisions missing material or stages
due rotations. `PromotePending` activates the entire pending transaction, while
`DiscardPending` removes material that was never active. Root rotation stages a
successor for every known issuer and `TrustBundlePEM` exposes both roots before
promotion.

Root keys are encrypted PKCS#8 using PBES2, scrypt, and AES-256-CBC. A generated
unlock secret stays in its manager until it is exported, deliberately stored in
the owner-only local unlock file, or cleared by `Close`. Leaf private keys are
never persisted by authority signing or the issuance ledger.

FileStore requires an existing `0750` authority directory with an explicit
owner and issuer group. SQLiteStore supports the same split-principal layout or
a private `0600` single-principal deployment. SQLite uses rollback journals and
a separate writable ledger database. Both stores reject symbolic links and
unsafe file types at their security boundaries.

FileStore and SQLiteStore do not migrate data between one another. Applications
must perform an explicit, audited export/import workflow if they change
backends; automatic migration is intentionally outside v1.
