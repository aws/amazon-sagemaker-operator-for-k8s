#!/bin/bash

BUNDLED_RELEASE_PATH="release/"
TMP_RELEASE_PATH="/tmp/release"

mkdir -p "${TMP_RELEASE_PATH}"

# Create installers from CRDs into temporary directory
INSTALLER_PATH="${TMP_RELEASE_PATH}" make create-installers

# Diff the two folders, excluding hidden files (such as .gitkeep)
diff -r --exclude=".*" "${TMP_RELEASE_PATH}" "${BUNDLED_RELEASE_PATH}"

if [ $? -ne 0 ]; then
  echo "Release files did not match. See diff above for more details"
  exit 1
fi