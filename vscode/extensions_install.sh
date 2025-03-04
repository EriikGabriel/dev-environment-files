#!/bin/bash

# Path to JSON file with extensions
JSON_FILE="extensions.json"

# Extract extensions from JSON file
EXTENSIONS=$(jq -r '.[].identifier.id' "$JSON_FILE")

# Install all extensions
for EXT in $EXTENSIONS
do
  echo "-> Instalando a extensão: $EXT"
  code --install-extension "$EXT"
done