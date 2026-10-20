# Evaluation plan

Automated: prove hard constraints, stable ranking, zero match, original availability handling, idempotent decisions, concurrent stock changes, session isolation, auth, consent and rendering budget. Use real PostgreSQL for persistence/transaction checks. Provider HTTP fixtures validate contracts only.

Browser: shopper shortlist, compare, choose, reject, restart; merchant auth, stock edits, summary; change stock between shortlist and choice; mobile and desktop; loading/error/empty states. Record actual runs separately from planned checks.

Formative study, not yet conducted: approximately five shoppers and one or two sellers. Compare equivalent tasks using a plain catalog and shortlist+preview with counterbalanced order. Record completion, task time, constraint violations, confidence and chosen SKU. Use consent and a separate real-study dataset. Small samples give early signals, not measured conversion uplift or recovered revenue.
