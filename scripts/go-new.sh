#!/bin/bash
go-new() {
  local src="github.com/KarlGW/go-template/templates/"
  local type=$1
  local dst=$2

  case $type in
  "base")
    src+="$type"
    ;;
  "http-server")
    src+="$type"
    ;;
  "server")
    src+="$type"
    ;;
  "service")
    src+="$type"
    ;;
  *)
    echo "Type $type is not supported."
    exit 1
    ;;
  esac

  if [ -z "$dst" ]; then
    echo "Destination cannot be empty."
    exit 1
  fi

  gonew "$src" "$dst"
}
