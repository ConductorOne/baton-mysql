# CXH-2683: sync MySQL routine-level grants from mysql.procs_priv

Branch: `connector-farm/CXH-2683-cf-26102914qb-CXH-2683-cf-26102914qb.12.4-impl` (attempt 4; PR title verbatim above; PR to be opened off `main` by the orchestrator — implementer did not push).

## What ships

Routine (procedure/function) grants are now synced **principal-side** from `mysql.procs_priv`:

- `Client.ListRoutineGrants` (`pkg/client/grants.go`) selects `User, Host, Db, Routine_name, Proc_priv` from `mysql.procs_priv` and is **version-neutral** — the 8.0-only `Routine_type` column is omitted entirely (I2), so the same query works on 5.7 and 8.x.
- `listRoutineGrants` is wired into `grantsForUserOrRole`'s host loop between `listColumnGrants` and `listProxyGrants` (users AND roles inherit it via `user.go`/`role.go` `Grants`); entitlement slugs reuse the existing `execute`, `alter_routine`, `grant` slugs via the `TableGrant.GetPrivs`-style normalization. `routineSyncer.Grants` stays an empty stub with an explanatory comment (I1).
- **Grant side (5a):** the Grant switch accepts the `grant` slug. The mapping helper `routinePrivilege` returns the `GRANT OPTION` marker, and `GrantRoutinePrivilege` ships the **R2 fallback form** (see "GRANT option form shipped" below).
- **Revoke side (5a, R1):** the Revoke path NEVER builds the forbidden bare `REVOKE GRANT OPTION ON PROCEDURE ...` (which revokes held privileges). Instead the `grant` slug routes through `Client.RevokeRoutineGrantOption`, which builds `REVOKE GRANT OPTION FOR <held privs> ON <TYPE> <schema>.<routine> FROM <user>`. `<held privs>` = the principal's held routine privileges for that schema+routine (rows UNIONed and deduped for the collapsed PROCEDURE+FUNCTION case), decoded via `RoutineGrant.GetPrivs`, uppercased with underscores restored, EXCLUDING `Grant`, sorted alphabetically for deterministic SQL. If nothing besides the grant option is held, the revoke is a no-op success.
- **Collapsed-host revoke fix (ported from sibling attempts 12.1/12.3):** `revokeRoutineGrantOption` splits the collapsed `user@host1,host2` host on the comma and queries `mysql.procs_priv` per host, UNIONing rows — the same host loop as `grantsForUserOrRole`. Previously the comma-joined host was passed verbatim, matched no row, and turned a needed revoke into a silent success. The decode/dedupe/sort logic is factored into a DB-free `routineHeldPrivs` helper, unit-tested by `TestRoutineHeldPrivs` (match, Grant-only, union dedupe, sorting, no-match). Single-host behavior is unchanged.
- **5b (I6):** the Grant principal gate now accepts `resourceTypeRole` alongside `resourceTypeUser`, gated on `c.IsVersion8()`; 5.x behavior (roles rejected) is unchanged. Revoke has no principal gate (minimal change).
- **5c (I7):** `GetRoutineType` is now a multi-row query: 0 rows → `baton-mysql: routine <schema>.<routine> not found` (wrapping `sql.ErrNoRows`); 1 DISTINCT type → unchanged behavior; >1 DISTINCT type → a loud ambiguity error refusing to grant/revoke the wrong object. No `ROUTINE_TYPE` in resource IDs.
- **Idempotency (I8):** Revoke-side, MySQL "no such grant defined" errors → success. The error number is level-specific: routine-level REVOKE raises 1403 (`ER_NONEXISTING_PROC_GRANT`), and 1147 (`ER_NONEXISTING_TABLE_GRANT`) is accepted alongside it for parity with the table-level shape; both map to success. Grant-side, string-based "already exists" detection → `GrantAlreadyExists` annotation.
- Every new-path query failure returns a wrapped error with the `baton-mysql: ` prefix and the remediation string `GRANT SELECT (Host, User, Db, Routine_name, Proc_priv) ON mysql.procs_priv TO <account>` in `ListRoutineGrants` failures (I4/R4). Pre-existing swallows in `listGlobalGrants`/`listProxyGrants` are untouched.

## (a) ROUTINE_TYPE-in-resource-ID full fix: reported out of scope

The complete 5c fix would encode `ROUTINE_TYPE` into routine resource IDs so colliding PROCEDURE/FUNCTION pairs get distinct IDs. This is deliberately **out of scope**: changing the resource-ID format is a breaking change (existing downstream c1z ledgers, saved views, and grants recorded under the current `routine:<db>.<name>` ID shape would be orphaned), which the batch's acceptance criteria forbid (BUGBOT breaking-change reason). Instead, the collapsed-ID behavior is kept and the collision is handled loudly at grant/revoke time via the `GetRoutineType` ambiguity error.

## (b) Collapsed-ID union overstatement

Because routine resource IDs do not encode the routine type, a schema containing both a PROCEDURE and a FUNCTION with the same name collapses both objects into one resource/entitlement ID. Both objects' privileges are UNIONed under the single ID, so the connector may **overstate** the privileges granted on the collapsed pair (you see the union of what is actually held across the two objects). The Revoke-side held-privs lookup applies the same union (dedupe) consistently, mirroring the collapsed-ID semantics. This is reported as a known limitation, not fixed here.

## (c) Parallel defect reported, NOT fixed (I10)

`pkg/connector/database.go` has an analogous defect: `databaseSyncer.Grant` (use-site at `pkg/connector/database.go:89` calling `GrantDatabasePrivilege`) routes the `grant` privilege through `extractDatabasePrivilegeAndDb` (database.go:113-124; the grant→GRANT mis-normalization is the `strings.ToUpper(strings.ReplaceAll(...))` statement at database.go:121), producing the invalid statement `GRANT GRANT ON db.* ...` instead of `WITH GRANT OPTION`. Per the batch plan this defect is **reported only** — this PR touches no code in `database.go` (verified: it is absent from the diff).

## (d) D9 live-verification status and the GRANT/REVOKE grant-option form that shipped

**Status: NOT live-verified.** No MySQL 8.0 validation environment was available in this run (no `mysql`/`mysqld`/`docker`/`podman` binary in the environment), so neither statement was executed against a live server. Both forms are therefore reported unverified, never silently shipped as verified:

- **GRANT form shipped (R2 fallback):** the primary form `GRANT GRANT OPTION ON PROCEDURE <schema>.<routine> TO <user>` could not be live-verified on MySQL 8.0, so per locked rule R2 the connector ships the fallback form, implemented as a branch inside `GrantRoutinePrivilege`:

  `GRANT USAGE ON PROCEDURE <schema>.<routine> TO <user> WITH GRANT OPTION`

  (the same `WITH GRANT OPTION` form used by `extractServerPrivilege` in `pkg/connector/server.go`). The non-grant templates (`GRANT %s ON %s %s.%s TO %s`, `REVOKE %s ON %s %s.%s FROM %s`) remain byte-identical to baseline.

  **Doc-grounding of the fallback form (static, source-level — still not live-verified):** the MySQL 8.0 server grammar maps `USAGE` to zero privilege bits (`PT_static_privilege(@1, 0)` in `sql/sql_yacc.yy`), and the routine-level GRANT/REVOKE check `mysql_routine_grant` accepts a privilege set whose bits are a subset of `PROC_ACLS = (ALTER_PROC_ACL | EXECUTE_ACL | GRANT_ACL)` (`sql/auth/sql_authorization.cc`, `auth_acls.h`); with `USAGE` contributing 0 bits and `WITH GRANT OPTION` contributing `GRANT_ACL`, the statement passes the routine-level check and stores a `mysql.procs_priv` row whose `Proc_priv` is `Grant` alone (the R3 "only Grant member" case). The same holds on 5.7 (same grammar shape and same `rights & ~PROC_ACLS` check). This is static evidence from the server source, not an execution against a live server.
- **REVOKE form shipped (R1):** `REVOKE GRANT OPTION FOR <held privs> ON PROCEDURE <schema>.<routine> FROM <user>` via `Client.RevokeRoutineGrantOption`. The literal `GRANT OPTION` never reaches the `REVOKE %s ON ...` template (asserted by the code layout and the unit tests around the branch).

Recommended follow-up for the orchestrator/verifier: run both statements once against a real MySQL 8.0 instance; if the primary `GRANT GRANT OPTION` form turns out to be valid and preferred, flipping `GrantRoutinePrivilege`'s branch is a one-line change.

## (e) Kind-rule compliance notes (R5)

`bash scripts/check-kind-rules.sh` on the branch: **KIND RULES CLEAN — 0 violations, 0 hints.**

- GO-P-020 (no new syncer): routine grants ride the existing routineSyncer/principal-side path; `routineSyncer.Grants` remains an empty stub.
- GO-P-021 (stateless): all new helpers are pure/DB-free where possible; no caching.
- GO-P-040/041/046 (errors returned and wrapped): every new-path failure returns wrapped `baton-mysql: ...` errors; no new log-and-continue.
- GO-A-014/024 (no swallow in new path): `listRoutineGrants` returns errors untouched; only pre-existing swallows remain.
- GO-P-060 (principal-side entity source): grants collected in `grantsForUserOrRole`.
- GO-P-100/GO-A-003 (no breaking change): resource types, IDs, slugs, config unchanged.
- GO-P-091 (docs): README.md + docs/connector.mdx gained the `mysql.procs_priv` provisioning line in BOTH the 5.7 and 8+ blocks (two per file).
- GO-P-092/093 (targeted validation): `scripts/validate-changes.sh` (build+vet+test of `./pkg/client` and `./pkg/connector`) passed; no repo-wide builds.

## Verification run

- `go vet ./pkg/client ./pkg/connector` — clean.
- `go test ./pkg/client ./pkg/connector` — ok (new tests: `routineTypesFromRows`, `RoutineGrant.GetPrivs`/ID shape, `isNoSuchGrantErr`, `routinePrivilege`, `routinePrincipalAllowed`, `routineGrantKeys`).
- `bash scripts/validate-changes.sh --repo <worktree>` — validation passed.
- `bash scripts/check-kind-rules.sh <worktree>` — clean.
- D9 live MySQL verification — **unavailable** (see section d); remainder reported, not hidden.