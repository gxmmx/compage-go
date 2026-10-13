# Keep focused packages in one module

compage-go is a framework-free set of independently adoptable capability packages in one Go module. Keep standalone capabilities at the module root, nest packages when they share a clear area, and keep the internal dependency graph acyclic; do not add a root facade that duplicates package APIs. This supports individual adoption while keeping releases and internal reuse simple.
