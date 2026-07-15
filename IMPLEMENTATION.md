# LocalRAGChatBot - Implementation Roadmap

## Vision

Build a modular, offline-first Local RAG platform that supports
high-quality retrieval, document intelligence, and future extensions
(GraphRAG, SQL Agents, MCP, multimodal, etc.).

## Guiding Principles

-   Pipeline-first architecture
-   Replaceable components through interfaces
-   Offline-first
-   Incremental indexing
-   Rich metadata
-   Continuous smoke testing after every milestone

## Target Pipeline

``` text
Document
  -> Loader
  -> Normalizer
  -> Metadata Extractor
  -> Chunker
  -> Summarizer
  -> Embedding
  -> Storage

User Query
  -> Planner
  -> Query Rewriter
  -> Retriever
  -> Reranker
  -> Evidence Builder
  -> Verifier
  -> Context Builder
  -> LLM
  -> Response
```

## Milestones

### Phase 0 - Architecture Baseline

-   [ ] Finalize interfaces
-   [ ] Map temp implementations
-   [ ] Smoke test existing project

### Phase 1 - Document Ingestion

-   [ ] Loader abstraction
-   [ ] Metadata extraction
-   [ ] Normalization
-   [ ] Incremental indexing (hash based)
-   [ ] Background ingestion queue

### Phase 2 - Chunking

-   [ ] Hierarchical chunking
-   [ ] Parent/child links
-   [ ] Neighbor links
-   [ ] Chunk summaries

### Phase 3 - Embeddings & Storage

-   [ ] Embedding abstraction
-   [ ] Rich chunk schema
-   [ ] Collection management
-   [ ] Vector storage

### Phase 4 - Retrieval

-   [ ] Planner
-   [ ] Hybrid retrieval
-   [ ] Metadata search
-   [ ] Cross-collection support
-   [ ] Workspace retrieval

### Phase 5 - Evidence

-   [ ] Evidence graph
-   [ ] Context builder
-   [ ] Verification
-   [ ] Confidence estimation

### Phase 6 - Conversation

-   [ ] Workspace memory
-   [ ] Session management
-   [ ] Streaming
-   [ ] Citations

### Phase 7 - UI

-   [ ] Collection management
-   [ ] Diagnostics
-   [ ] Progress tracking
-   [ ] Advanced retrieval controls

### Phase 8 - Future Extensions

-   [ ] GraphRAG hooks
-   [ ] SQL Agent hooks
-   [ ] MCP integration points
-   [ ] OCR pipeline hooks

## Cross-cutting Improvements

-   Rich metadata
-   Adaptive context builder
-   Plugin architecture
-   Event bus
-   Diagnostics dashboard

## Verification Checklist (Run after every milestone)

-   Backend builds successfully.
-   Frontend builds successfully.
-   Python utilities execute.
-   No regressions.
-   Update this document with completed work.

## Decision Log

(To be updated as architectural decisions are finalized.)

## Change Log

(To be updated after every milestone with files changed, tests executed,
and results.)


## Progress Update — Evidence/Workspace Upgrade

### Implemented
- Rich chunk metadata in ingestion and storage.
- Document summaries stored during ingest.
- Offline extractive summarization helpers.
- Incremental schema migration for new chunk columns.
- Evidence effort control from UI to backend.
- Planner now tracks workspace-memory usage.
- Enhanced evidence bundle construction with neighbor expansion and refinement passes.
- Frontend effort selector for low/medium/high evidence gathering.
- Temp phase files excluded from builds.

### Verification Status
- Go formatting completed.
- TypeScript syntax checking attempted, but npm dependencies are not installed in this container.
- Full Go test/build remains blocked by missing external module downloads in the offline container.

### Next Checks
- Install frontend and Go dependencies in a networked environment.
- Run full backend smoke test.
- Run frontend build smoke test.
- Verify chat plan/result metadata in the UI.


## Progress Update — Phase 1 Ingestion Hardening

### Implemented in this pass
- Added encoding-aware text decoding for plain-text ingest paths.
- Added a duplicate-classification API that can return identical, moved, renamed, and partial matches.
- Added persisted ingestion job lookup by batch ID for resume/log flows.
- Added ingestion log persistence hooks during staging and completion.
- Added focused unit tests for text decoding, normalization, and chunk-overlap scoring.

### Verification Status
- Go formatting re-run after the new ingestion changes.
- Build/test execution still deferred per project instruction.

### Next Checks
- Wire duplicate-match results into the upload UI if needed.
- Extend queue UI to show job logs from persisted ingestion events.
- Continue tightening resume/retry/cancel behavior if any edge cases remain.


## Progress Update — Phase 1 Ingestion UI Completion Pass

### Implemented in this pass
- Added duplicate preflight checks in the upload modal using existing file-hash lookup support.
- Added per-file replace toggles and a batch-wide "mark all duplicates for replace" action.
- Added a visible ingestion queue panel inside the collections view.
- Added recent ingest log rendering from live backend ingest events.
- Added queue controls for resume, discard, and cancel active ingest.
- Added additional local ingest log entries for resume/discard/cancel actions.

### Verification Status
- Go files were formatted with `gofmt`.
- TypeScript static check still cannot run cleanly in this container because frontend dependencies are not installed.
- No Go build was attempted, per the current project instruction.

### Next Checks
- Install frontend dependencies in a networked environment and run the frontend build.
- Verify the generated Wails bindings against the updated frontend imports.
- Smoke-test duplicate preflight, replace toggles, resume, discard, and cancel actions in the UI.


## Progress Update — Phase 2 Hierarchical Chunking Starter

### Implemented in this pass
- Added hierarchical chunk planning in the ingest package.
- Added summary parent chunks and leaf child chunks with explicit ord-based parent/prev/next links.
- Added chunk summaries and heading-path metadata to chunk records.
- Updated ingestion to persist the hierarchical chunk plan instead of flat-only chunks.
- Added duplicate-hash filtering so summary chunks do not affect duplicate detection.
- Added focused tests for hierarchical chunk planning and summary filtering.
- Added a migration for chunk hierarchy columns and indexes.

### Verification Status
- Go formatting re-run on the touched files.
- Full backend build/test still not attempted per current instruction.

### Next Checks
- Expand retrieval to use parent/neighbor chunk context.
- Add UI affordances for chunk hierarchy inspection if needed.
- Continue Phase 2 with neighbor-aware evidence and chunk browsing surfaces.


## Progress Update — Phase 2 Retrieval Context Expansion

### Implemented in this pass
- Added chunk-by-ID and chunk-neighborhood lookups in the store layer.
- Expanded retrieval results with parent summary chunks and adjacent neighbor chunks.
- Preserved chunk ordering while deduplicating expanded evidence hits.
- Added heading-path decoding for retrieval-time evidence decoration.
- Kept the existing chat context builder unchanged by feeding it richer scored chunks.

### Verification Status
- Go formatting will be re-run on the touched files.
- No Go build/test is being attempted per project instruction.

### Next Checks
- Add a focused unit test for retrieval expansion ordering if needed.
- Add a UI affordance for browsing expanded chunk context if desired.
- Continue Phase 2 only if more hierarchy surfaces are still required.


## Progress Update — Phase 2 Chunk Browser

### Implemented in this pass
- Added a backend chunk-context API that returns a chunk with parent/neighbor rows from the same document.
- Added a search-result "Inspect context" action in the UI.
- Added a chunk context modal that renders hierarchy metadata, summary, and nearby chunk content.
- Kept the phase ledger as the source of truth for future incremental implementation.

### Verification Status
- Go formatting should be re-run on the touched backend files.
- Frontend build remains dependent on the installed npm toolchain in the environment.
- This pass intentionally avoided a full repository rescan.

### Next Checks
- Decide whether chunk browsing should also be exposed from the document list, not only universal search.
- Continue Phase 2 only if you want additional hierarchy inspection surfaces.


## Progress Update — Phase 2 Chunk Browser in Collections View

### Implemented in this pass
- Added a backend `GetDocumentChunks` API that returns all stored chunks for a document.
- Wired the document preview pane to load both extracted text and chunk records.
- Added a chunk browser to the collections view so document-list selection exposes hierarchy-aware chunk inspection.
- Added per-chunk metadata display: role, ord, level, parent/prev/next links, heading path, summary, and content excerpt.
- Added an inspect-context action on each chunk card that reuses the existing chunk context modal.

### Verification Status
- Targeted formatting applied to touched Go/TypeScript files.
- No full Go build was attempted per project instruction.

### Next Checks
- Consider adding chunk filtering/sorting if the browser becomes too dense for large documents.
- Continue Phase 2 only if additional hierarchy visualization surfaces are still desired.

## Progress Update — Phase 3 Embeddings & Storage Starter

### Implemented in this pass
- Added an embedding-provider abstraction on the engine so higher layers can read model metadata without coupling to llama-go internals.
- Added collection-level embedding/vector profile fields: embedding model, embedding dimensions, vector backend, and updated timestamp.
- Added collection creation/update APIs that persist the active embedding/vector profile.
- Added a vector-store adapter for SQLite/sqlite-vec so chunk insert/search/delete code no longer reaches into the database directly.
- Added a collection profile modal in the UI so the active collection’s embedding metadata can be viewed and edited.
- Added collection profile display chips in the collections view.

### Verification Status
- Go formatting re-run on the touched backend files.
- Frontend build still cannot be fully executed in this container because npm dependencies are not installed.
- No Go build was attempted, per the current project instruction.

### Next Checks
- Decide whether the collection profile should expose more backend-specific settings later (batch size, chunk dimensions, reranker backend).
- Continue Phase 3 with richer vector storage diagnostics or embedding model selection if needed.

## Progress Update — Phase 4 Retrieval Modes

### Implemented in this pass
- Added a document-level metadata search path over filenames, titles, summaries, content, and collection names.
- Added a workspace search path that merges document retrieval with the current session's cited source chunks.
- Added backend helpers to score metadata and workspace hits.
- Added a retrieval-mode selector in the search UI for collection, all collections, workspace, and metadata scopes.
- Added UI filtering for metadata and workspace result types.
- Added focused tests for metadata scoring and workspace scoring helpers.

### Verification Status
- Go formatting completed on the touched backend files.
- Frontend build still depends on the installed npm toolchain in the environment.
- No full Go build was attempted, per project instruction.

### Next Checks
- Verify the search UI wiring against the current Wails bindings in a networked environment.
- Expand metadata search if you want author/date/tag columns surfaced later.
- Continue into Phase 5 evidence construction once retrieval behavior is stable.

## Progress Update — Phase 5 Evidence Layer

### Implemented in this pass
- Added a formal evidence bundle that expands retrieved chunks into a node-and-edge graph.
- Added evidence node classification for primary, summary, previous, next, and context rows.
- Added evidence scoring, coverage estimation, verification, and confidence calculation.
- Added evidence context rendering for prompt assembly with explicit evidence verdicts and graph relationships.
- Added backend chat metadata for evidence count, confidence, verification, and evidence gaps.
- Added a `chat:evidence` emit path so the UI and diagnostics can react to evidence-building results later.
- Added frontend badges for evidence count, confidence, and verification state on assistant messages.
- Added focused tests for the evidence heuristics.

### Verification Status
- Go formatting will be re-run on the touched backend files.
- Frontend build remains dependent on installed npm dependencies in the environment.
- No Go build was attempted, per the current project instruction.

### Next Checks
- Expand the evidence graph if you want contradiction tracking or multi-document support.
- Add a dedicated evidence inspector panel later if diagnostics need more depth.
- Continue into Phase 6 conversation memory and streaming once the evidence layer is stable.


## Progress Update — Phase 6 Conversation Memory and Citations

### Implemented in this pass
- Added persistent rolling conversation memory rows with session/collection scoping.
- Added backend helpers to load and upsert workspace memory summaries per chat session.
- Routed workspace memory into planner decisions so follow-up turns can be classified as memory-driven even when the visible chat history is short.
- Included workspace memory in the system prompt alongside retrieved evidence.
- Rolled a compact memory summary forward after successful assistant turns.
- Added a lightweight citation chip row under assistant messages for visible source references.
- Added a session-memory API for future inspection surfaces.
- Added focused unit tests for conversation-memory summary behavior.

### Verification Status
- Go formatting re-run on the touched backend files.
- Full backend build/test still deferred per project instruction.
- Frontend build still depends on the installed npm toolchain in the environment.

### Next Checks
- Add a dedicated memory inspector panel later if you want a visible session-summary surface.
- Continue into Phase 7 UI polish and diagnostics when ready.

## Progress Update — Phase 7 UI Diagnostics Dashboard

### Implemented in this pass
- Added a dedicated diagnostics tab to the sidebar and main app shell.
- Added a live diagnostics dashboard summarizing collections, documents, chunks, chat activity, and ingest queue state.
- Added progress cards for queue health, document readiness, and evidence-confidence signals.
- Added recent ingest-log rendering and queue action shortcuts from the diagnostics view.
- Kept the diagnostics view fully driven by existing workspace state so it does not introduce a second backend path.

### Verification Status
- Touched frontend files were checked for obvious JSX/type issues.
- No full frontend build was attempted in this offline container.
- No Go build was attempted, per project instruction.

### Next Checks
- Verify the diagnostics tab against the current Wails bindings in a networked environment.
- Consider whether more backend-driven system stats are needed later (memory, disk, GPU, uptime).
- Continue to the next phase only if any further UI/diagnostics surfaces are still desired.


## Progress Update — Phase 8 Future Extension Hooks

### Implemented in this pass
- Added a persisted extension-hook registry for future GraphRAG, SQL agent, MCP, and OCR surfaces.
- Added default hook descriptors and seed rows for the canonical phase-8 integration points.
- Added backend APIs to list, update, and reset extension-hook records.
- Added a frontend Extensions tab with per-hook enable toggles, state editing, config JSON editing, and reset support.
- Added a sidebar entry for the Extensions tab so the future integration surfaces are discoverable.

### Verification Status
- Go formatting will be re-run on the touched backend files.
- Frontend build remains dependent on installed npm dependencies in the environment.
- No Go build was attempted, per project instruction.

### Next Checks
- Add real GraphRAG / SQL / MCP / OCR implementations behind these hook surfaces when the corresponding phase is active.
- Decide whether any hook-specific runtime events should be emitted later.

## Progress Update — Phase 9 Workspace Audit Trail

### Implemented in this pass
- Added a durable workspace event log table for diagnostics and extension activity.
- Added backend APIs to write and read recent workspace events.
- Logged collection, chat, ingest, and extension-hook mutations into the event trail.
- Added a frontend event log loader and a diagnostics panel section for recent workspace events.
- Updated the bindings and shared UI types so event records flow from backend to diagnostics.

### Verification Status
- Go formatting still needs to be re-run on the touched backend files after this pass.
- Frontend build remains dependent on installed npm dependencies in the environment.
- No Go build was attempted, per project instruction.

### Next Checks
- Decide whether event filtering by scope or severity is needed later.
- Continue with any additional diagnostics or extension runtime work if desired.

## Progress Update — Phase 10 Advanced Retrieval Controls

### Implemented in this pass
- Added a user-controlled max-results setting to the universal search UI.
- Added a minimum-score filter to search results so users can trim weak matches before inspection.
- Extended backend search APIs to accept caller-supplied result limits for collection, metadata, and workspace scopes.
- Kept search limits capped server-side so high values remain bounded.
- Updated the frontend bindings and search panel to pass the new limit through end to end.

### Verification Status
- Go formatting still needs to be re-run on the touched backend files after this pass.
- Frontend build remains dependent on installed npm dependencies in the environment.
- No Go build was attempted, per project instruction.

### Next Checks
- Decide whether search sorting or a saved search preset UI is needed later.
- Expand retrieval controls further only if users need more than max-results and minimum-score filtering.

