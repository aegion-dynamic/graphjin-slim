# Engine Review — Everything We Found, Explained Simply

This single document merges three sources: my line-by-line reading of the core code,
and two independent research reports on the best-known algorithms for each part of
the system. Everything is in plain language. Technical words are explained either
right where they appear or in the glossary at the end.

Nothing in this document has been changed in the code yet. This is the plan.

---

## Part 1 — The one serious bug: two requests can corrupt each other

### What's happening

Imagine a kitchen where every chef shares ONE scratchpad. Two chefs preparing two
different orders at the same moment will write over each other's notes, and dishes
come out wrong — sometimes. That is exactly what our GraphQL compiler does today.

When your app asks for data, the compiler first "compiles" the request into an
internal plan (which tables to read, how to join them). To do that, it keeps
temporary notes — like "this query has 2 root sections". The bug: those notes are
stored **on the compiler itself**, and there is only one compiler per database,
shared by everyone. If two requests compile at the same time, they write their notes
into the same box, and Request A can end up with Request B's plan.

### How we know it's real

I wrote a small test that made 100 requests compile at once against one compiler and
ran it under Go's race detector (a tool that watches memory for exactly this kind of
clash). It caught the clash at the exact line predicted. Then I reverted everything —
the fix is planned, not yet applied.

### When it hurts

- In development mode: every single request compiles, so any traffic at all can hit this.
- In production mode: queries are compiled once and cached, so it only bites during
  the first seconds after startup, when many *different* queries arrive together.

### What goes wrong for users

Corrupted plans mean wrong SQL gets generated — wrong results or random errors that
cannot be reproduced. This is the worst category of bug: rare, random, and silent.

### The fix

Give every compilation its own scratchpad instead of sharing one. Small change:
about 6 functions need an extra argument passed down. No public behavior changes.
**This is priority #1.**

---

## Part 2 — The cursor lock uses a fragile trick

### Background in one paragraph

When results are paginated, we hand the client an opaque "cursor" (a scrambled
string meaning "page 2 starts here"). Clients can tamper with strings they receive,
so cursors are locked with authenticated encryption (a cipher that both scrambles
and tamper-proofs). The cipher we use, AES-GCM, has one strict rule: every message
must use a unique "nonce" — a never-repeated number. Repeating it quietly weakens
the lock. Today we generate the nonce from a hash of the page data, which works but
is the kind of cleverness security reviewers flag.

### What research said

Both research agents independently recommended the same thing: switch to AES-GCM-SIV,
a variant specifically designed so that even a repeated nonce stays safe. One catch:
Go's standard library does not include it, so adopting it means adding one vetted
security package — a decision for you.

There is also a free win regardless of that decision: right now we build the entire
encryption machinery from scratch on **every request**. It should be built once when
the server starts and reused. That is a small code change with zero risk.

---

## Part 3 — Wasted computer work (speed = money)

These are not bugs; they are places where the machine does more work than needed on
every request. Each entry says what happens now, why it wastes time, and the fix.

### 3.1 Rebuilding the crypto machine per request
Covered in Part 2. Free speedup on every response.

### 3.2 Re-reading finished SQL on every execution
Some queries (bulk inserts) run as several SQL statements in sequence. Before running
them, the engine re-analyzes the text each time — counting placeholders, checking
what kind of statement each one is, looking for markers. But production compiles the
query once and reuses it forever, so this analysis gives the same answer millions of
times. Fix: do the analysis once at compile time and store the answers next to the
query.

### 3.3 Spell-checking a letter that already parsed fine
Every incoming query is parsed (read and understood) by the parser. Immediately after
a *successful* parse, a second check scans the whole raw text with a regex just to
produce a nicer error message in a rare mistake case ("you wrote `db.table` — just
write `table`"). That scan runs on every successful request in dev mode. Fix: let the
parser report the problem while reading, instead of scanning again afterward.

### 3.4 The slow way of turning numbers into text
In 18 places, code builds SQL text using `fmt.Sprintf("%d", n)` — a general-purpose
formatter that is 2–3× slower and allocates memory, when Go has a dedicated fast
function (`strconv`) for exactly this. Tiny waste × millions of calls = measurable.
Fix: mechanical replacement, half a day including tests.

### 3.5 Throwing away workspaces in dev mode
Compiling needs a workspace (an output buffer). Dev mode creates a fresh one per
request and throws it away; Go then spends effort cleaning up. Fix: keep a small pool
of reusable workspaces (Go has built-in support for this pattern).

### 3.6 Reading the letter twice for multi-database routing
When a deployment spans several databases, the engine must decide which database each
top-level section of a query belongs to. To do that it reads/parses the query — and
then the normal compiler parses it *again*. Double work, plus a design smell: the
shared core should not know how to read GraphQL at all; that knowledge belongs in the
GraphQL plugin. Fix: add a small capability to the plugin interface ("tell me the top
level sections"), and let the plugin answer from its own parse. Fixes waste AND
restores the clean separation we built.

---

## Part 4 — Small correctness risks (cheap insurance)

### 4.1 Cache key collision — could serve the WRONG saved query forever
Production caches compiled queries keyed by gluing together `namespace + name +
database`. Gluing without a separator means `user` + `slist` produces the same key as
`users` + `list`. If that ever collides, one query permanently gets another's compiled
plan. Probability is low; damage is total. Fix: one line — insert a separator
character. Do this early.

### 4.2 A failed query is remembered forever
If the very first compilation of a query fails — even because the database was
unreachable for one second — that failure is cached permanently and every future
request gets the old error until restart. Research consensus on the right policy:
remember *real* mistakes (bad syntax) briefly so we don't retry them constantly;
forget *temporary* failures immediately so the next request retries normally.

### 4.3 SQLite bulk-insert size limit is unguarded
Databases cap how many values one statement may contain (SQLite: 999 on older
builds). Our bulk inserts don't check that ceiling today — a big enough payload breaks
on SQLite. Fix: split oversized inserts into chunks sized by the dialect's limit.
Researchers confirmed simple division-based chunking is optimal here; no fancy math
needed.

### 4.4 Leftover AI-chat comments in shipped code
`sqlgen/query.go` contains comments like *"Wait, original:"*, *"Let's check if
compilerContext implements Context"*, *"which I missed reading fully"* — fragments of
an assistant's reasoning that got committed as documentation. Delete and replace with
real explanations. Also split the ~340-line execution function into readable pieces
while in there.

### 4.5 Dead code
A helper function (`prepareQueryArgsForDB`) that receives a database-type argument it
ignores and changes nothing. Either give it a real job or delete it.

---

## Part 5 — Designs banked for the caching feature (not built yet)

The response-cache layer currently exists only as empty stubs in this build. When we
build it for real, research converged on these three choices. Recorded now so we
don't re-research:

1. **Stampede control — XFetch.** After a deploy, the cache is cold and thousands of
   simultaneous requests all trigger the same expensive work (a "stampede"). XFetch
   refreshes entries slightly *before* they expire, at random times, so the crowd
   spreads out naturally. Simple formula, near-optimal, published result.
2. **Eviction — SIEVE.** When the cache fills up, something must be thrown out.
   Instead of classic LRU (which requires bookkeeping on every read), SIEVE marks
   items with a single bit on read and does its cleanup lazily. Simpler than LRU,
   faster under heavy concurrent load, published at NSDI 2024.
3. **Invalidation — compact table-index.** Cached pieces record which tables they
   came from; a write to a table must invalidate those pieces. A bitmap-compressed
   index (Roaring bitmaps) makes "everything depending on table X" a near-instant
   lookup. Also from research consensus.

---

## Part 6 — Things researchers checked that we ALREADY do right

Worth recording so nobody "optimizes" these later by mistake:

- **Our JSON splicing approach.** We merge JSON documents by scanning raw bytes once
  with a hand-built state machine, never fully parsing. Both agents confirmed this is
  the correct choice for document sizes like ours; fancy SIMD parsers only win on
  documents hundreds of times larger.
- **Validation loop.** Checking ≤30 variables one-by-one in a plain loop is provably
  optimal; the "branchless bit-twiddling" alternatives add complexity for zero gain.
- **Multi-database fan-out.** Plain goroutines already implement the work-stealing
  scheduling agent 2 suggested hand-building.
- **Schema lookups, param deduplication, reload safety.** All map-based / structurally
  sound; verified during the deep read.

---

## Part 7 — What we will NOT do, and why

| Suggestion (from research) | Why rejected |
|---|---|
| SIMD JSON parser | Needs C bindings or immature Go ports; wins only on huge documents, ours are small |
| Unsafe memory tricks for building SQL text | SQL text is built once per query in production; risk buys nothing |
| Perfect-hash tables for merge keys | Our key sets are tiny; existing hashing already costs nanoseconds |
| Bytecode-style rendering engine | Only pays off if the same tree is rendered repeatedly; we render once |
| Hand-built lock-free queues | Go's runtime scheduler already does this better |
| Fancy hash (BLAKE3) for cache keys | Extra dependency for keys nobody ever sees |

---

## Part 8 — Found by the verification battery (bench), added as it was built

15. **~~Nested inserts silently drop children~~ FIXED** — the frontend compiler forced every one-to-many nested child to "connect" semantics and never emitted single-object children at all, so `users(insert: {..., products: {...}})` returned success while writing nothing — on every dialect. Children now compile as real inserts (updates keep connect semantics) and object payloads produce their mutate. Verified end-to-end by the bench battery; was previously a visible skip.
16. **Semicolon inside any inserted value breaks sqlite mutations** — multi-statement scripts split on `;` without honoring quoted literals... actually fixed: values are now quote-escaped at render time (see below); splitter honors quotes.
17. **REST saved-query handler panicked on nil result** (`serv/service.go` apiV1Rest → responseHandler dereferenced nil after an engine error) — connection killed mid-response. Fixed with the same guard the GraphQL path already had.
18. **SQL errors were mislabeled "GraphQL syntax or parse error"** by the repair classifier — now a distinct `database_error` kind fires for driver-shaped errors before the parse bucket.

Also fixed while building this: mutation/filter string literals are now quote-escaped (`O'Brien` renders as `'O''Brien'`) across sqlgen emitters and both dialects — closing a correctness hole and an injection-shaped hazard. Regression test: `TestQuoteEscapingInMutationValues`.

---

## Final order of work

| # | Task | Size | Payoff |
|---|------|------|--------|
| 1 | Give each compilation its own scratchpad (Part 1) | Small | Removes the serious bug |
| 2 | Separator in cache keys (4.1) | One line | Removes silent-wrong-query risk |
| 3 | Build crypto machinery once (Part 2, step 1) | Small | Free speedup everywhere |
| 4 | Smarter failure caching (4.2) | Small | Self-healing after hiccups |
| 5 | strconv sweep + comment cleanup (3.4, 4.4) | Half day | Speed + professionalism |
| 6 | SQLite insert chunking (4.3) | Medium | Prevents hard failures at scale |
| 7 | Plugin-owned routing, kill double parse (3.6) | Medium | Speed + cleaner architecture |
| 8 | Precomputed script metadata (3.2) | Medium | Speed on bulk paths |
| 9 | Cursor lock upgrade to GCM-SIV (Part 2, step 2) | Decision + medium | Security hardening |
| 10 | Regex-after-parse removal (3.3) | Small | Dev-mode speed |

Items from Part 5 join the queue only when the caching feature becomes real.

---

## Glossary

- **Race condition** — two parts of the program writing to the same memory at the
  same time; outcomes become random.
- **Nonce** — a number that must never repeat for a given encryption key.
- **AES-GCM / GCM-SIV** — standard authenticated-encryption ciphers; the SIV variant
  tolerates nonce reuse safely.
- **Cache stampede** — many requests simultaneously triggering the same expensive
  work after a cold start or expiry.
- **LRU / SIEVE** — strategies for choosing what to evict from a full cache.
- **CTE (WITH ... RETURNING)** — a SQL feature letting one statement use the output
  of a previous step inside the same round-trip.
- **Bind parameters** — the `?`/`$1` slots in SQL that carry user values; databases
  cap how many one statement may have.
- **SIMD** — CPU instructions processing many bytes at once; fast but complex to deploy.
- **Regex** — a pattern-matching scan over text.
- **Single-flight** — ensuring only one worker performs a task while others wait for
  the same result.
