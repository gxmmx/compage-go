# compage-go

Terms for describing the library's public structure and the relationship between its Go packages.

## Language

**Application building block**:
A reusable capability an application can adopt without taking on a prescribed framework or runtime model.

**Capability package**:
A focused Go package that provides one application building block and can be imported independently.

**Adapter package**:
A package that connects an external library or interface to a capability package while keeping that integration optional.

**Shared foundational package**:
A package of small, reusable concepts or mechanisms that capability packages can depend on without importing higher-level application behavior.

**Capability-specific platform support**:
The set of operating systems and platform features supported by an individual capability package, independent of the support offered by other packages in the module.

**Explicit capability composition**:
A declared, opt-in relationship where one capability coordinates another to provide a combined application operation.

**Error classification**:
A shared, transport-neutral category that describes how a failure should be interpreted at a package boundary, separate from the contextual details owned by the capability that produced it.
