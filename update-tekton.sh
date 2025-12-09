#!/bin/bash

# Use this script to update the Tekton Task Bundle references used in a Pipeline or a PipelineRun.
# update-tekton-task-bundles.sh .tekton/*.yaml

set +e -uo pipefail

FILES=$@

# Find existing image references
OLD_REFS="$(
  yq '... | select(has("resolver")) | .params // [] | .[] | select(.name == "bundle") | .value' $FILES |
    grep -v -- '---' |
    sort -u
)"

# Find updates for image references
for old_ref in ${OLD_REFS}; do
  repo_tag="${old_ref%@*}"
  task_ver=${repo_tag#*:}
  task_x=${task_ver%.*}
  task_y=$((${task_ver#*.} + 1))
  task_name=${repo_tag%:*}
  new_digest=$(skopeo inspect --no-tags docker://${task_name}:${task_x}.${task_y} 2>/dev/null)
  if [[ $? -eq 0 ]]; then
    new_digest=$(echo $new_digest | yq '.Digest')
    new_ref="${task_name}:${task_x}.${task_y}@${new_digest}"
    echo "New digest and version found! $new_ref"
    if [[ $SKIP_UPDATE == "true" ]]; then continue; fi
    for file in $FILES; do
      sed -i -e "s!${old_ref}!${new_ref}!g" $file
    done
    continue
  else
    new_digest="$(skopeo inspect --no-tags docker://${repo_tag} | yq '.Digest')"
    new_ref="${repo_tag}@${new_digest}"
    [[ $new_ref == $old_ref ]] && continue
    echo "New digest found! $new_ref"
    if [[ $SKIP_UPDATE == "true" ]]; then continue; fi
    for file in $FILES; do
      sed -i -e "s!${old_ref}!${new_ref}!g" $file
    done
  fi
done
