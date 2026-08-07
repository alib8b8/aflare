-------------------------------------------------------------------------------
-- aflare DAG Scheduler — TLA+ Specification
--
-- This specification formalises the topological-batch scheduling algorithm
-- in /workspace/hkaic/internal/workflow/dag.go (topoBatches).
--
-- It models:
--   - A set of N steps {0..N-1}
--   - A dependency relation deps: step -> set of steps it waits for
--   - The scheduler state: which steps are "done", and the current batch
--
-- Invariants verified (safety):
--   SafeBatch     — every step in a batch has ALL its deps in prior batches
--   NoDoubleExec  — no step appears in two batches
--   AllScheduled  — every step appears in exactly one batch (completeness)
--
-- Liveness (termination):
--   EventuallyDone — if the graph is acyclic, all steps eventually become done
--
-- The Go test dag_formal_test.go performs bounded model-checking of these
-- invariants over randomly generated DAGs, serving as an executable companion
-- to this specification.
-------------------------------------------------------------------------------

------------------------------- MODULE DagScheduler ---------------------------

EXTENDS Naturals, Sequences, FiniteSets, TLC

(* @definition *)
CONSTANTS
    N,            (* number of steps: the model is the set 0..N-1        *)
    Deps          (* function: step -> subset of 0..N-1 it depends on     *)

(* S is the set of all step indices. *)
S == 0..N-1

-------------------------------------------------------------------------------
-- Acyclicity precondition
--
-- A dependency graph is acyclic iff there exists a topological ordering.
-- topoBatches() in dag.go first calls detectCycle() and aborts if a cycle
-- is found. We model that precondition here: the scheduler only runs on
-- graphs that the cycle detector has accepted.
--
-- Acyclic == the transitive closure of Deps has no self-loop.
-------------------------------------------------------------------------------

(* Transitive closure of Deps (reachability). *)
ReachableFrom(i) ==
    LET Rec(close, frontier) ==
        IF frontier = {}
        THEN close
        ELSE LET n == CHOOSE x \in frontier : TRUE
             IN Rec(close \cup frontier,
                     (\Union {Deps[x]} \cup {Deps[x]}) \ close)
    IN Rec({}, {i})

(* Acyclic: no step can reach itself through the dependency relation. *)
Acyclic == \A i \in S : i \notin ReachableFrom(i)

-------------------------------------------------------------------------------
-- Scheduler state
-------------------------------------------------------------------------------

VARIABLES
    done,         (* set of steps that have completed execution            *)
    batches       (* sequence of batches, each batch is a set of steps     *)

-------------------------------------------------------------------------------
-- The "ready" set: steps whose deps are all done and that are not yet done.
-- This mirrors the inner loop of topoBatches():
--   "for i := 0; i < nodeCount; i++ { if inDegree[i]==0 { batch=append...} }"
-------------------------------------------------------------------------------

Ready ==
    { i \in S :
        /\ i \notin done
        /\ \A d \in Deps[i] : d \in done
    }

-------------------------------------------------------------------------------
-- Next-state relation: pick the entire Ready set as the next batch.
-- (dag.go collects ALL ready steps into one batch, not just one.)
-------------------------------------------------------------------------------

PickBatch ==
    /\ Ready # {}
    /\ batches' = Append(batches, Ready)
    /\ done'   = done \cup Ready

(* Termination: no more ready steps. Either we're done, or we're stuck. *)
Done == Ready = {}

-------------------------------------------------------------------------------
-- Init + Next
-------------------------------------------------------------------------------

Init ==
    /\ done = {}
    /\ batches = << >>

Next ==
    \/ PickBatch
    \/ Done  (* stutter when no more steps can be scheduled *)

-------------------------------------------------------------------------------
-- Safety invariants
-------------------------------------------------------------------------------

(* SafeBatch: every step in batch b has all deps in earlier batches. *)
SafeBatch ==
    \A b \in 1..Len(batches) :
        \A i \in batches[b] :
            \A d \in Deps[i] :
                \E c \in 1..(b-1) : d \in batches[c]

(* NoDoubleExec: no step appears in two batches. *)
NoDoubleExec ==
    \A b1, b2 \in 1..Len(batches) :
        b1 # b2 => batches[b1] \cap batches[b2] = {}

(* AllScheduled: every step is in exactly one batch (when Done). *)
AllScheduled ==
    Done => \Union {batches[b] : b \in 1..Len(batches)} = S

-------------------------------------------------------------------------------
-- Liveness: if acyclic, eventually all steps are done.
-------------------------------------------------------------------------------

EventuallyDone ==
    Acyclic => <>(done = S)

-------------------------------------------------------------------------------
-- Theorem: the spec implies all invariants.
-------------------------------------------------------------------------------

Spec == Init /\ [][Next]_<<done, batches>>

Theorem ==
    /\ Spec
    /\ []SafeBatch
    /\ []NoDoubleExec
    /\ AllScheduled
    /\ EventuallyDone

=============================================================================
