// Crash-loop guard for the Go sidecar's auto-restart (see main.js).
//
// The server is respawned whenever it dies while the app is running — e.g. an
// external `pkill editor-server` (another project's app build ships a sidecar
// with the same binary name) or a crash. Unlimited respawning would spin
// forever on a server that keeps dying, so restarts are allowed only while
// failures are not consecutive "quick" ones: a run that stayed up at least
// stableMs *after becoming ready* resets the counter, and more than
// maxQuickFailures consecutive quick deaths means giving up.
//
// Stability is measured from readiness (the server printed its port), not
// from spawn: a run that never became ready is always a quick failure, no
// matter how long the process lingered before dying — otherwise a server
// that takes >stableMs to fail at startup (e.g. a hung filesystem blocking
// its repo check) would reset the counter every attempt and respawn forever.
//
// Pure and clock-injected (timestamps are passed in) so main.js stays a thin
// shell and this decision logic is unit-testable without Electron.

"use strict";

function newRestartPolicy({ maxQuickFailures = 3, stableMs = 10_000 } = {}) {
  let readyAt = null;
  let quickFailures = 0;
  return {
    // Record a spawn attempt. Call right before every spawn (including the
    // first); the run counts as a quick failure until ready() is called.
    started() {
      readyAt = null;
    },
    // Record that the current run became ready (the server printed its port).
    ready(nowMs) {
      readyAt = nowMs;
    },
    // Record that the server died (or failed to start) and answer whether to
    // try again.
    shouldRestart(nowMs) {
      const readyUptime = readyAt == null ? 0 : nowMs - readyAt;
      if (readyUptime >= stableMs) quickFailures = 0;
      quickFailures += 1;
      return quickFailures <= maxQuickFailures;
    },
  };
}

module.exports = { newRestartPolicy };
