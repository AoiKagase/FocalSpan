# C++ extraction

C++ is parsed statically from source tokens. No compiler, template instantiation, macro execution, overload type checker, or runtime dispatch is used.

The CP01–CP20 regression suite covers numeric separators, UTF-8 BOM and CRLF offsets, line-comment splicing, commented includes, namespace directives and anonymous ownership, nested types and template headers, constructor/operator/typedef declarators, function bodies including requires-expressions and handlers, parameter-type declaration pairing, lexical shadowing, template calls, scoped cross-file calls, and named module imports.

Concrete chunks preserve original source bytes and line positions. Qualified and receiver calls never fall back to unrelated global names. Unknown receivers and ambiguous declarations remain unresolved. C++ lexical lookup rules are shared by extraction, linking, and unresolved relation expansion; other languages retain their existing rules. Anonymous namespaces have file-specific identities and same-file implicit lookup.

Named module declarations keep translation-unit owners and add logical module symbols. Unique interface/partition providers resolve imports to their file owners. Global/private module fragments do not produce imports, and duplicate or unknown providers stay unresolved. Namespace alias/using directives have correct boundaries; general alias/using-set resolution and full C++ compiler semantics remain unsupported.

Extractor version v20 causes regular indexing to refresh older extraction results once, including removal of obsolete relations. It does not require a DB schema migration. Deployment and live-index updates are separate operations.
