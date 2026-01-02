#!/bin/bash
go-new() {
  local src="github.com/KarlGW/go-template/templates/"
  local type=$1
  local dst=$2

  if [[ "$type" != "base" ]] && [[ "$type" != "http-server" ]] && [[ "$type" != "server" ]] && [[ "$type" != "service" ]]; then
    echo "Type $type is not supported."
    exit 1
  fi

  if [ -z "$dst" ]; then
    echo "Destination cannot be empty."
    exit 1
  fi

  gonew "$src" "$dst"
}
