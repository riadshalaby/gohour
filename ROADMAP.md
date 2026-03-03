# ROADMAP for v0.3.1

## Goal
Stabilize and harden the v0.3.0 web redesign, then prepare focused incremental improvements.

## Priorities
1. Reliability hardening
- Verify submit/import/refresh flows end-to-end in browser scenarios.
- Close any remaining UX edge cases in dialogs and async actions.

2. Test strategy expansion
- Add browser-level smoke coverage for core month/day workflows.
- Keep server-handler tests aligned with HTMX partial behavior.

3. Usability polish
- Tighten action feedback consistency (loading/error/success states).
- Refine responsive behavior and accessibility details.

## Acceptance Criteria
- Core web flows are stable under normal and degraded remote-auth conditions.
- Automated coverage protects critical submit/import regressions.
- Documentation stays aligned with any behavior changes.
