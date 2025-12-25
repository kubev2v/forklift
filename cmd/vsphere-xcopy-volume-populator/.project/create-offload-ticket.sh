#!/bin/bash

# A script to create a Jira ticket for the copy-offload team.

# --- Configuration ---
# You can pre-configure the version here to avoid being prompted every time.
# Example: TARGET_VERSION="MTV 2.6"
TARGET_VERSION=""
PROJECT=MTV
DEFAULT_LABELS="mtv-copy-offload"
DEFAULT_COMPONENT="Storage Offload"
ISSUE_TYPES=("Story" "Bug" "Task")

# --- Functions ---

function check_deps() {
  if ! command -v jira &> /dev/null; then
    echo "Error: 'jira' CLI is not installed or not in your PATH."
    echo "Please install it from: https://github.com/ankitpokhrel/jira-cli"
    exit 1
  fi
  if ! command -v fzf &> /dev/null; then
    echo "Error: 'fzf' is not installed or not in your PATH."
    echo "fzf is used for interactive selection. Please install it."
    exit 1
  fi
}

function get_target_version() {
  if [[ -z "$TARGET_VERSION" ]]; then
    read -p "Enter the target release version (e.g., 2.10.x, 2.11.0): " TARGET_VERSION
  fi
  
  if [[ -z "$TARGET_VERSION" ]]; then
    echo "No version selected. Exiting."
    exit 1
  fi
  
  echo "Using target version: $TARGET_VERSION"
}


function select_epic() {
  echo "Fetching active epics for version '$TARGET_VERSION'..."
  
  # JQL to find Epics in the target version that are not "Done"
  JQL="project = '$PROJECT' AND type = Epic AND fixVersion = '$TARGET_VERSION' AND status != Done AND labels in ($DEFAULT_LABELS)"
  
  EPICS=$(jira epic list -q "$JQL" --plain --columns "KEY,SUMMARY" --no-headers)
  
  if [[ -z "$EPICS" ]]; then
    echo "No active epics found for version '$TARGET_VERSION' in project '$PROJECT'."
    echo "Please create an Epic for this version first."
    exit 1
  fi
  
  echo "Please select the parent Epic:"
  SELECTED_EPIC=$(echo "$EPICS" | fzf --prompt="Select Epic > ")
  
  if [[ -z "$SELECTED_EPIC" ]]; then
    echo "No epic selected. Exiting."
    exit 1
  fi
  
  EPIC_KEY=$(echo "$SELECTED_EPIC" | awk '{print $1}')
  echo "Selected Epic: $EPIC_KEY"
}

function get_issue_details() {
  echo "Please select the issue type:"
  ISSUE_TYPE=$(printf "%s\n" "${ISSUE_TYPES[@]}" | fzf)
   if [[ -z "$ISSUE_TYPE" ]]; then
    echo "No issue type selected. Exiting."
    exit 1
  fi
  echo "Selected issue type: $ISSUE_TYPE"

  read -p "Enter the issue title (summary): " ISSUE_TITLE
  if [[ -z "$ISSUE_TITLE" ]]; then
    echo "Title cannot be empty. Exiting."
    exit 1
  fi
}

function create_ticket() {
  echo
  echo "--- Creating Jira Issue ---"
  echo "Project:      $PROJECT"
  echo "Version:      $TARGET_VERSION"
  echo "Epic:         $EPIC_KEY"
  echo "Type:         $ISSUE_TYPE"
  echo "Title:        $ISSUE_TITLE"
  echo "Labels:       $DEFAULT_LABELS"
  echo "Component:    $DEFAULT_COMPONENT"
  echo "---------------------------"
  
  read -p "Proceed to create this issue? (y/N) " confirm
  if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
    echo "Aborted."
    exit 0
  fi
  
  jira issue create \
    -p"$PROJECT" \
    --type="$ISSUE_TYPE" \
    --summary="$ISSUE_TITLE" \
    --parent="$EPIC_KEY" \
    --fix-version="$TARGET_VERSION" \
    --label="$DEFAULT_LABELS" \
    --component="$DEFAULT_COMPONENT" \
    --web
}

# --- Main ---

check_deps
get_target_version
select_epic
get_issue_details
create_ticket

echo "Done."

