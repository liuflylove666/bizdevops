#!/bin/sh
set -eu

git clone "${GITOPS_REPO_URL}" gitops
cd gitops

python3 - <<'PY'
import os
import yaml

values_file = os.environ["GITOPS_VALUES_FILE"]
with open(values_file, "r", encoding="utf-8") as fh:
    values = yaml.safe_load(fh) or {}

values.setdefault("image", {})
values["image"]["repository"] = os.environ["IMAGE_REPOSITORY"]
values["image"]["tag"] = os.environ["IMAGE_TAG"]

with open(values_file, "w", encoding="utf-8") as fh:
    yaml.safe_dump(values, fh, sort_keys=False)
PY

git status --short
git diff -- "${GITOPS_VALUES_FILE}"
git add "${GITOPS_VALUES_FILE}"
if git diff --cached --quiet; then
  echo "values file is already up to date"
  exit 0
fi

git commit -m "chore(gitops): deploy ${CI_PROJECT_NAME} ${IMAGE_TAG}"
git push origin HEAD:main
