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
port=$2
if [[ -z "$app_name" ]]; then
  echo "Provide a name for the application."
  exit 1
fi

if [[ -z $port ]]; then
  port=8080
fi

sedi "s/{{bin}}/$app_name/g" Dockerfile && sedi "s/{{port}}/$port/g" Dockerfile && rm ./scripts/init.sh
