"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { newRestartPolicy } = require("./restart-policy");

test("a server that dies after running stably is always restarted", () => {
  const p = newRestartPolicy({ maxQuickFailures: 3, stableMs: 10_000 });
  let now = 0;
  for (let i = 0; i < 10; i++) {
    p.started();
    p.ready(now);
    now += 60_000; // ready-uptime 60s ≥ stableMs
    assert.equal(p.shouldRestart(now), true, `stable death #${i + 1} should restart`);
  }
});

test("a crash loop is cut off after maxQuickFailures quick deaths", () => {
  const p = newRestartPolicy({ maxQuickFailures: 3, stableMs: 10_000 });
  let now = 0;
  for (let i = 0; i < 3; i++) {
    p.started();
    p.ready(now);
    now += 100; // dies almost immediately after becoming ready
    assert.equal(p.shouldRestart(now), true, `quick death #${i + 1} still restarts`);
  }
  p.started();
  p.ready(now);
  now += 100;
  assert.equal(p.shouldRestart(now), false, "4th quick death gives up");
});

test("a stable run resets the quick-failure counter", () => {
  const p = newRestartPolicy({ maxQuickFailures: 3, stableMs: 10_000 });
  let now = 0;
  // Two quick deaths...
  for (let i = 0; i < 2; i++) {
    p.started();
    p.ready(now);
    now += 100;
    assert.equal(p.shouldRestart(now), true);
  }
  // ...then a stable run: the counter resets.
  p.started();
  p.ready(now);
  now += 10_000; // exactly stableMs of ready-uptime counts as stable
  assert.equal(p.shouldRestart(now), true);
  // The crash-loop budget is available in full again.
  for (let i = 0; i < 2; i++) {
    p.started();
    p.ready(now);
    now += 100;
    assert.equal(p.shouldRestart(now), true, `post-reset quick death #${i + 1}`);
  }
});

test("a spawn attempt that never became ready counts as a quick failure", () => {
  // started() is called per attempt; if startServer rejects, the next
  // shouldRestart sees a never-ready run. Without the readiness reset, a
  // previously long uptime would reset the counter every loop iteration and
  // a permanently-broken server would respawn forever.
  const p = newRestartPolicy({ maxQuickFailures: 3, stableMs: 10_000 });
  let now = 0;
  p.started();
  p.ready(now);
  now += 60_000; // long stable run, then death
  assert.equal(p.shouldRestart(now), true);
  for (let i = 0; i < 2; i++) {
    p.started(); // respawn attempt fails instantly, never ready
    assert.equal(p.shouldRestart(now), true, `failed spawn #${i + 1} retries`);
  }
  p.started();
  assert.equal(p.shouldRestart(now), false, "budget exhausted despite earlier stability");
});

test("slow startup failures never reset the counter, no matter how long they take", () => {
  // Regression guard: stability is measured from ready(), not from spawn. A
  // server that takes longer than stableMs to *fail* at startup (e.g. a hung
  // filesystem blocking its repo check before it exits) must still be a
  // quick failure — measuring from spawn time would reset the counter every
  // attempt and the restart loop would spin forever.
  const p = newRestartPolicy({ maxQuickFailures: 3, stableMs: 10_000 });
  let now = 0;
  for (let i = 0; i < 3; i++) {
    p.started();
    now += 11_000; // dies after 11s without ever printing its port
    assert.equal(p.shouldRestart(now), true, `slow failure #${i + 1} still restarts`);
  }
  p.started();
  now += 11_000;
  assert.equal(p.shouldRestart(now), false, "4th slow never-ready failure gives up");
});
