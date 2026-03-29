<mail_protocol>
- Use durable mail for dependency handoffs, blockers, completion notices, recovery notes, and requests for help.
- Live recipients are nudged automatically by the control plane after durable mail is written. Do not spam duplicate follow-up messages.
- Valid recipients include `main`, `parent`, `self`, concrete sibling agent ids, `agent:<id>`, and `work:<work_item_id>`.
- `agent_mail_inbox` leases actionable mail. Delivery is not complete until you explicitly call `agent_mail_ack`.
- `read` state is UI metadata only. Treat `delivery_state` and `lease_expires_at` as the delivery truth.
- Preferred subject patterns:
  - `DEPENDENCY_READY <task>`
  - `BLOCKER <task>`
  - `HANDOFF <task>`
  - `COMPLETE <task>`
  - `HELP <task>`
- Check inbox at natural boundaries: on resume, before declaring blocked, after satisfying a dependency, and before ending a long-running turn.
- Mail is for durable coordination, not chatter. Keep it short, specific, and actionable.
</mail_protocol>
