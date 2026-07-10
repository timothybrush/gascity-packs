Atomically claim one routed workflow bead for current live session.

Usage:
  gc <binding> claim

The command calls:

```bash
gc hook --claim --drain-ack --json
```

For `action=work`, it re-reads claimed bead and verifies its id, open or
in-progress status, assignee, and route before returning one normalized JSON
object. Result includes `bead_id`, `root_bead_id`, `continuation_group`, and
full `bead` record. `action=drain` means no routed work remained and drain
acknowledgement completed.
