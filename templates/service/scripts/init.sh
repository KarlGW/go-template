#!/bin/bash
sedi() {
  local pattern=$1
  local file=$2
  if [[ "$OSTYPE" =~ "darwin" ]]; then
    sed -i '' "$pattern" "$file"
  else
    sed -i "$pattern" "$file"
  fi
}

app_name=$1
if [[ -z "$app_name" ]]; then
  echo "Provide a name for the application."
  exit 1
fi

sedi "s/{{bin}}/$app_name/g" Dockerfile && rm ./scripts/init.sh
