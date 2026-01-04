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

sedi "s/{{bin}}/$app_name/g" Dockerfile && rm ./scripts/init.sh
