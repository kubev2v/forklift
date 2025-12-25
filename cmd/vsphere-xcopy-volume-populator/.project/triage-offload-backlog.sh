#!/bin/bash

# A script to help triage the copy-offload backlog by categorizing
# untriaged issues into new or existing Epics in an interactive way.

# --- Configuration ---
PROJECTS="MTV,ECOPROJECT" # Comma-separated list of projects to search
TRIAGE_LABELS="mtv-copy-offload,mtv-storage-offload" # Comma-separated list of labels
OUTPUT_FILE="untriaged_tickets.txt"
# You can add more versions here as they become relevant
RELEVANT_VERSIONS="2.11.0 2.10.3 2.10.0 2.9.0" 

# --- Functions ---

function check_deps() {
  if ! command -v jira &> /dev/null; then
    echo "Error: 'jira' CLI is not installed or not in your PATH."
    exit 1
  fi
  if ! command -v fzf &> /dev/null; then
    echo "Error: 'fzf' is not installed or not in your PATH."
    exit 1
  fi
}

# --- Phase 1: Find Untriaged Issues ---
function find_untriaged_issues() {
  echo "Phase 1: Finding issues with label(s) \"$TRIAGE_LABELS\" in project(s) \"$PROJECTS\" that are not in an Epic..."

  JQL="project IN ($PROJECTS) AND labels IN ($TRIAGE_LABELS) AND issueType != Epic AND \"Epic Link\" is EMPTY"

  if ! jira issue list -q "$JQL" --order-by created --reverse --plain --no-headers --columns "KEY,SUMMARY" > "$OUTPUT_FILE"; then
    echo "Error: Failed to fetch issues from Jira. Please check your JQL query and connection."
    rm -f "$OUTPUT_FILE"
    exit 1
  fi

  if [[ ! -s "$OUTPUT_FILE" ]]; then
    echo "Congratulations! No untriaged issues found. Everything is categorized."
    exit 0
  fi

  echo "Found the following untriaged issues:"
  echo "--------------------------------------------------"
  cat "$OUTPUT_FILE"
  echo "--------------------------------------------------"
  echo "This list has been saved to '$OUTPUT_FILE'."
  echo
}

# --- Phase 2: Create New Epics (Optional) ---
function create_new_epics() {
  echo "Phase 2: Create New Epics (Optional)."
  echo "--------------------------------------------------"

  while true; do
    read -p "Do you want to create a new Epic before assigning tickets? (y/N) " create_epic_choice
    if [[ "$create_epic_choice" != "y" && "$create_epic_choice" != "Y" ]]; then
      break
    fi

    local project_for_epics=$(echo $PROJECTS | tr ',' '\n' | fzf --prompt="Select a project for the new Epic > ")
    if [[ -z "$project_for_epics" ]]; then
        echo "No project selected. Skipping Epic creation."
        continue
    fi

    read -p "Enter the name for the new Epic (e.g., 'My New Feature'): " EPIC_NAME
    if [[ -z "$EPIC_NAME" ]]; then
      echo "Epic name cannot be empty. Please try again."
      continue
    fi
    local EPIC_SUMMARY="[Copy-Offload] $EPIC_NAME" # Standard prefix

    echo "Select the target version for this Epic (optional, press Esc to skip):"
    local SELECTED_VERSION=$(echo "$RELEVANT_VERSIONS" | tr ' ' '\n' | fzf --prompt="Select Version > ")

    local create_cmd=(jira epic create -p "$project_for_epics" -n "$EPIC_SUMMARY" -s "$EPIC_SUMMARY" -l "$TRIAGE_LABELS")
    if [[ -n "$SELECTED_VERSION" ]]; then
        create_cmd+=(--fix-version="$SELECTED_VERSION")
    fi

    echo "About to run: ${create_cmd[*]}"
    read -p "Proceed? (Y/n) " confirm_create
    if [[ "$confirm_create" == "n" || "$confirm_create" == "N" ]]; then
        echo "Cancelled."
        continue
    fi

    if ! "${create_cmd[@]}" ; then
      echo "Error: Failed to create Epic. Please check your input and Jira permissions."
    else
      echo "Epic '$EPIC_SUMMARY' created successfully."
    fi
    echo
  done
  echo "Finished creating Epics."
  echo
}

# --- Phase 3: Assign Issues to Epics ---
function assign_issues_to_epics() {
  echo "Phase 3: Assigning untriaged issues to Epics."
  echo "--------------------------------------------------"

  local untriaged_issues_count=$(wc -l < "$OUTPUT_FILE")
  if [[ "$untriaged_issues_count" -eq 0 ]]; then
    echo "No untriaged issues to assign."
    return
  fi

  echo "Fetching available Epics with label 'mtv-copy-offload' for assignment..."
  JQL_EPICS="project IN ($PROJECTS) AND type = Epic AND status != Done AND labels = mtv-copy-offload"
  AVAILABLE_EPICS=$(jira epic list -q "$JQL_EPICS" --plain --no-headers --order-by updated --reverse --columns "KEY,SUMMARY")

  if [[ -z "$AVAILABLE_EPICS" ]]; then
    echo "No Epics found to assign issues to. Please create some Epics first."
    return
  fi

  echo "You have $untriaged_issues_count untriaged issues to assign."
  echo "For each issue, you will be prompted to select an Epic."

  mapfile -t ISSUES_ARRAY < "$OUTPUT_FILE"

  for ISSUE_LINE in "${ISSUES_ARRAY[@]}"; do
    ISSUE_KEY=$(echo "$ISSUE_LINE" | awk '{print $1}')
    ISSUE_SUMMARY=$(echo "$ISSUE_LINE" | cut -d' ' -f2-)

    echo "--------------------------------------------------"
    echo "Assigning Issue: $ISSUE_KEY - $ISSUE_SUMMARY"
    
    SELECTED_EPIC=$(echo "$AVAILABLE_EPICS" | fzf --prompt="Assign \'$ISSUE_KEY | $ISSUE_SUMMARY\' to Epic > " --header="Select an Epic or press Esc to skip")

    if [[ -z "$SELECTED_EPIC" ]]; then
      echo "Skipping $ISSUE_KEY. It will remain untriaged."
      continue
    fi

    EPIC_TO_ASSIGN_KEY=$(echo "$SELECTED_EPIC" | awk '{print $1}')

    echo "Assigning \'$ISSUE_KEY | $ISSUE_SUMMARY\' to Epic $EPIC_TO_ASSIGN_KEY..."
    if ! jira epic add "$EPIC_TO_ASSIGN_KEY" "$ISSUE_KEY"; then
      echo "Error: Failed to assign $ISSUE_KEY to $EPIC_TO_ASSIGN_KEY. This may be because it is a Sub-task or an Epic itself."
    else
      echo "Successfully assigned $ISSUE_KEY to $EPIC_TO_ASSIGN_KEY."
    fi
  done
  echo "--------------------------------------------------"
  echo "Assignment phase complete."
  echo
}

# --- Main ---
check_deps
find_untriaged_issues

if [[ -s "$OUTPUT_FILE" ]]; then
  read -p "Press [Enter] to proceed to the interactive assignment phase..."

  create_new_epics
  assign_issues_to_epics
fi

echo "Triage script finished."
rm -f "$OUTPUT_FILE"
