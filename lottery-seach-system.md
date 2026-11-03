# Lottery Search System — Design Document

## Summary

Redis keeps a stock count for every ticket number plus a 125 KB "has stock" bitmap. Each search takes its tickets on the spot as a 5-minute hold, inside one atomic Lua script, so two users can never be given the same ticket.

- **Stock count per number:** `ticket:123456 → 8` means number 123456 has 8 tickets left.
- **"Has stock" bitmap:** one bit per number (1M bits = 125 KB). Bit = 1 if that number still has tickets. It is used to skip sold-out numbers quickly, for any pattern shape.
- **Search = reserve:** a search picks matching numbers that have stock, takes one ticket from each (count − 1) and records a 5-minute hold for the user.
- **Buy or release:** buying turns the hold into a sale saved in PostgreSQL. If the user doesn't buy, the hold expires and the ticket goes back into stock (count + 1).
- **One Lua script per action** (search, buy, release), so every step is atomic and no two users can get the same ticket.
- **PostgreSQL** stores sales permanently; Redis can be rebuilt from it.

Assumption: numbers repeat across tickets (10 million tickets over 1 million possible numbers, about 10 per number).

## Architecture

&#91;embedded content: architecture · app servers, Redis, PostgreSQL\]

Users talk to stateless app servers, which do all ticket work by calling Lua scripts in Redis. Redis holds the stock counts, the has-stock bitmap and the holds. PostgreSQL stores ticket records and sales permanently, and is used to rebuild Redis if needed.

## Redis key and value design

Six kinds of keys cover search, holds, buying and limits. Together they take about 80 MB.

| Key                   | Type                      | Value                                                    | Example                                                               | Size                |
| --------------------- | ------------------------- | -------------------------------------------------------- | --------------------------------------------------------------------- | ------------------- |
| `ticket:{number}`     | String (integer)          | Tickets left for that number (not counting held or sold) | `ticket:123456` → `8`                                                 | 1M keys, \~60–80 MB |
| `avail`               | Bitmap                    | Bit _n_ = 1 if `ticket:n` > 0                            | bit 123456 = `1`                                                      | 1M bits = 125 KB    |
| `hold:{holdId}`       | Hash                      | `number`, `user`, `expires_at`, status (held / paying)   | `hold:h_9f2c` → `{number: 123456, user: u42, expires_at: 1762160400}` | one per active hold |
| `holds:expiry`        | Sorted set                | member = holdId, score = expiry time                     | `h_9f2c` @ `1762160400`                                               | one entry per hold  |
| `user:{userId}:holds` | Set                       | The user's active hold IDs                               | `user:u42:holds` → `{h_9f2c, h_77ab}`                                 | small               |
| `rate:{userId}`       | String (integer), 1 s TTL | Searches this second                                     | `rate:u42` → `3`                                                      | small               |

**`ticket:{number}`, the stock count.**

- Goes down by 1 when a ticket is _reserved_ (at search), not when it is bought.
- Goes back up by 1 when a hold expires or is released.
- Buying does not change it, because the ticket already left stock when it was held.

**`avail`, the "has stock" bitmap.**

- Kept in sync with the count inside the same Lua script: count drops to 0 → `SETBIT avail n 0`; count goes from 0 to 1 → `SETBIT avail n 1`.
- Checking one number is `GETBIT`, which is instant.
- Works for every pattern shape (`1234**`, `****23`, `1****5`). This is why it replaces range-bucket lists such as `avail:100000_199999`: those only help when the first digit is fixed, need a full list scan to filter, and are slow to remove items from.

**`hold:{holdId}` + `holds:expiry`, the reservations.**

- One hold = one ticket, for one user, for 5 minutes.
- Expiry is tracked in the sorted set, not with a key TTL alone: Redis does not reliably notify you when a TTL deletes a key, so the ticket would never return to stock.
- The hold hash also gets a TTL slightly longer than the hold (e.g. 6 minutes) as a cleanup safety net.

**`user:{userId}:holds` and `rate:{userId}`, the limits.** Cap holds per user (e.g. 20) and searches per second (e.g. 5), so one user can't lock up a pattern.

**Redis Cluster note.** A Lua script can only touch keys in the same hash slot. Prefix every key with a hash tag, e.g. `{lotto}:ticket:123456`, `{lotto}:avail`. This data easily fits one Redis primary.

**If copies of a number are different tickets.** If the 8 copies of 123456 each have their own ticket ID or series, replace the count with a set of IDs: `ticket:123456 → {id1 … id8}`. Use `SPOP` to take one and `SADD` to put it back; the count is the set's size.

## How a search works (search and hold)

One Lua script, `SEARCH_AND_HOLD(pattern, user, n, now, seed)`, finds matching numbers with stock and reserves one ticket from each, all in a single atomic step.

1. **Check limits.** Validate the pattern (6 characters, digits or `*`), increase `rate:{user}`, and check that `user:{user}:holds` is under the cap.
2. **Turn the stars into candidate numbers.** Each star is a free digit, so a pattern with _k_ stars has 10^k candidates (`1234**` → 123400 to 123499).
   - 1–3 stars (at most 1,000 candidates): list them all, shuffle them using `seed`, and walk through them in that order.
   - 4 or more stars: try random candidates, up to about 2,000 tries per call, so the script never blocks Redis for long.
3. **For each candidate with `GETBIT avail n` = 1:**
   - `DECR ticket:n`. If the result is below 0, `INCR` it back and skip (safety check).
   - If the result is 0, `SETBIT avail n 0`.
   - Create `hold:{id}`, `ZADD holds:expiry`, `SADD user:{user}:holds`.
4. **Stop** after `n` holds. Return the held tickets with their expiry time, or "no results" if none were found.

`seed` is a random number passed in by the app, so the script's random choices are explicit and repeatable.

**Example: a user searches `1234**`.**

1. The 100 candidates (123400–123499) are shuffled, e.g. 123471, 123402, 123456, …
2. 123471 has bit 0 (sold out), so it's skipped. 123402 has bit 1; `DECR ticket:123402` takes it from 3 to 2, and a hold is created.
3. This continues until the user has `n` tickets. A second user searching `1234**` at the same moment runs after this script finishes, sees the updated counts, and gets different tickets.

## Buying, releasing and expiry

A held ticket ends in one of two ways: the user buys it, or it goes back into stock.

| Action              | When                              | What happens                                                                                                                                                                                                                                                           | Stock count                         |
| ------------------- | --------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------- |
| `BUY(holdId, user)` | User pays                         | Check the hold exists, belongs to the user and hasn't expired. The app saves the sale in PostgreSQL (using `holdId` as an idempotency key, so retries don't double-sell). A script then deletes `hold:{id}` and removes it from `holds:expiry` and `user:{user}:holds` | Unchanged (already taken at search) |
| `RELEASE(holdId)`   | User cancels, or the hold expires | `INCR ticket:n`; if the result is 1, `SETBIT avail n 1`. Delete the hold and remove it from both sets                                                                                                                                                                  | + 1                                 |
| Sweeper             | Every second                      | `ZRANGEBYSCORE holds:expiry 0 now LIMIT 0 100`, then `RELEASE` each expired hold                                                                                                                                                                                       | + 1 per expired hold                |

**Buying safely in three steps.** Money is involved, so the sweeper must never release a ticket that is being paid for.

1. A script marks the hold `status = paying` and removes it from `holds:expiry`, so the sweeper ignores it.
2. The app saves the sale in PostgreSQL.
3. A script deletes the hold.

If the app crashes between steps, a hold stays in `paying`. A check every few minutes looks it up in PostgreSQL: if the sale exists, it deletes the hold; if not, it releases the ticket.

## How two users never get the same ticket

A ticket is only shown to a user after it has been taken out of stock for them, and taking it out happens inside a script that runs one at a time.

- **Redis runs one script at a time.** If two users search `1234**` in the same millisecond, one script finishes completely before the other starts, so the second sees the lower counts.
- **Search reserves, not just shows.** The count drops at search time, so the last copy of a number can be held by only one user. Without this, two users could both see it and one would fail at checkout.
- **The count can't go below zero.** `DECR` returns the new value; if it is below 0, the script puts it back and skips that number.
- **Count and bitmap change together.** Both are updated in the same script, so they never disagree.

**Edge cases.**

| Situation                                               | What happens                                                                               |
| ------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| Last copy of a number, two users searching              | The first script takes it; the second skips it (bit is 0) and picks another number         |
| Whole pattern sold out or fully held                    | "No results"; held tickets come back when their holds expire                               |
| Same user searches again                                | New holds, up to the per-user cap                                                          |
| Different patterns that overlap (`****23` and `1****3`) | Also safe: the hold is on the ticket, so no other search can take it, whatever its pattern |

## Which databases and why

Recommendation: **Redis for search, stock and holds; PostgreSQL for permanent records.**

| Part                              | Job                                                                   | Why this choice                                                                                                                                                                              |
| --------------------------------- | --------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Redis (primary + replica, AOF on) | Stock counts, "has stock" bitmap, holds, limits; runs the Lua scripts | Very fast (100,000+ simple operations per second), built-in bitmaps and counters, and scripts run one at a time so reservations are race-free. Available as a managed service on every cloud |
| PostgreSQL                        | Ticket records, sales, audit trail                                    | Reliable, transactional storage for money-related data; source for rebuilding Redis                                                                                                          |
| App servers (stateless)           | Validate requests, call the scripts, write sales, run the sweeper     | Hold no shared data, so more servers can be added freely                                                                                                                                     |

**Why not other options.**

- _PostgreSQL alone:_ normal indexes can't search patterns with stars in the middle (`1****5`), and every search would be a disk-backed write. Fine for low traffic, but slower.
- _Elasticsearch:_ good at text search, but wildcard queries are costly and it has no atomic way to reserve a ticket, so it fails the no-duplicates rule on its own.
- _Range-bucket lists in Redis_ (`avail:100000_199999`): only help when the first digit is fixed, need a full list scan to filter, and are slow to remove items from. The bitmap does the same job for every pattern in 125 KB.

## Speed and tradeoffs

A search takes about 0.3–1.5 ms, mostly network time to Redis. The work depends on how many tickets are asked for and how sold-out the pattern is, not on the 10 million tickets. These are estimates to confirm with a load test.

| Case                                            | Work inside Redis                 | Estimated time |
| ----------------------------------------------- | --------------------------------- | -------------- |
| Pattern with plenty of stock, 10 tickets        | \~10–20 bit checks + 10 holds     | 30–80 µs       |
| 1–3 stars, mostly sold out                      | up to 1,000 bit checks            | up to \~200 µs |
| 4+ stars, mostly sold out                       | up to 2,000 random tries (capped) | up to \~500 µs |
| Network round trip                              | —                                 | 0.2–1 ms       |
| For comparison: scanning 10M rows in a database | —                                 | 0.5–2 s        |

**Throughput.** Redis runs one script at a time, so at \~50 µs per typical search one Redis primary handles roughly 10,000–20,000 searches per second. Buys and releases are cheaper. App servers scale out freely.

**Memory.** About 80 MB in Redis: 60–80 MB of stock counts, 125 KB bitmap, plus holds (\~200 bytes each).

**Tradeoffs.**

- _Reserving at search time_ guarantees no duplicates, but tickets are locked for up to 5 minutes even if the user never buys. Short holds and per-user caps keep this small; the 5 minutes can be tuned.
- _Capped random tries_ keep Redis responsive, but a wide pattern (4+ stars) that is almost sold out may return "no results" while a few tickets remain. See the next section for the fix.
- _Simple counts_ are enough when copies of a number are interchangeable; if each copy is a distinct ticket, use a set of IDs per number (slightly more memory).

## Startup, failures and known limits

**Startup / rebuild from PostgreSQL.**

1. Count available tickets per number (not sold, not held) and `SET ticket:n` for each.
2. Set `avail` bits from those counts.
3. Re-create holds that haven't expired yet.

Turn on Redis persistence (AOF, `appendfsync everysec`) and a replica, so a full rebuild is rarely needed.

| Problem                                    | Effect                             | Fix                                                                                                 |
| ------------------------------------------ | ---------------------------------- | --------------------------------------------------------------------------------------------------- |
| App server crashes after a search          | User never sees their held tickets | Holds expire after 5 minutes and the sweeper returns them                                           |
| App crashes during a purchase              | Hold stuck in `paying`             | Check against PostgreSQL every few minutes: delete the hold if sold, release it if not              |
| Redis fails over and loses the last second | Counts may be slightly off         | Rebuild counts for affected numbers from PostgreSQL; a nightly check compares Redis with PostgreSQL |
| Redis down                                 | Search and reserve unavailable     | Show a maintenance message; do not fall back to unsafe search                                       |
| One user spams searches                    | Locks up stock                     | Per-user hold cap and rate limit                                                                    |

**Known limits.**

- _Wide patterns that are almost sold out:_ with 4+ stars, the \~2,000 random tries can miss the last few tickets. If an exact "sold out" answer matters, keep a stock counter per popular pattern, or run a slower full scan in small batches when random tries fail.
- _Interchangeable vs distinct tickets:_ counts assume the copies of a number are interchangeable. If not, store a set of ticket IDs per number (`SPOP` / `SADD`).
- _One Redis primary:_ enough for this data size and tens of thousands of searches per second. Beyond that, split numbers across shards and use hash tags so each script stays on one shard.
