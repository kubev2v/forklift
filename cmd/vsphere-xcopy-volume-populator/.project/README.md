# Copy-Offload Project Management Scripts

Helper scripts for managing copy-offload Jira tickets and backlog.

## Prerequisites

- [jira-cli](https://github.com/ankitpokhrel/jira-cli) - configured and authenticated
- [fzf](https://github.com/junegunn/fzf) - for interactive selection

## Scripts

### create-offload-ticket.sh

Creates a new Jira ticket for the copy-offload team.

**Usage:**
```bash
./.project/create-offload-ticket.sh
```

**Workflow:**
1. Prompts for target release version
2. Fetches active Epics for that version
3. Select Epic using fzf
4. Choose issue type (Story/Bug/Task)
5. Enter issue title
6. Creates ticket with default labels (`mtv-copy-offload`, `mtv-storage-offload`) and component (`Storage Offload`)

### triage-offload-backlog.sh

Triages untriaged copy-offload issues by assigning them to Epics.

**Usage:**
```bash
./.project/triage-offload-backlog.sh
```

**Workflow:**
1. Finds all untriaged issues (with copy-offload labels but no Epic)
2. Optionally create new Epics
3. Interactively assign issues to Epics using fzf

**Configuration:**
- Projects: `MTV`, `ECOPROJECT`
- Labels: `mtv-copy-offload`, `mtv-storage-offload`, `storage-offloading`
- Relevant versions: `2.11.0`, `2.10.3`, `2.10.0`, `2.9.0`

Edit the script to customize these values.
