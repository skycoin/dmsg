#!/usr/bin/env bash

function print_usage() {
  echo "Use: $0 [-t <docker_image_tag_name>] [-p | -b]"
  echo "use -p for push (it builds and push the image)"
  echo "use -b for build image locally"
}

function docker_build() {
  docker image build \
    --tag=skycoinpro/dmsg-server:"$tag" \
    -f ./docker/images/dmsg-server/Dockerfile .

  docker image build \
    --tag=skycoinpro/dmsg-discovery:"$tag" \
    -f ./docker/images/dmsg-discovery/Dockerfile .
}

function docker_push() {
  docker login -u "$DOCKER_USERNAME" -p "$DOCKER_PASSWORD"
  docker tag skycoinpro/dmsg-server:"$tag" skycoinpro/dmsg-server:"$tag"
  docker tag skycoinpro/dmsg-discovery:"$tag" skycoinpro/dmsg-discovery:"$tag"
  docker image push skycoinpro/dmsg-server:"$tag"
  docker image push skycoinpro/dmsg-discovery:"$tag"
}

while getopts ":t:pb" o; do
  case "${o}" in
  t)
    tag="$(echo "${OPTARG}" | tr -d '[:space:]')"
    if [[ $tag == "develop" ]]; then
      tag="test"
    elif [[ $tag == "master" ]]; then
      tag="latest"
    fi
    ;;
  p)
    docker_build
    docker_push
    ;;
  b)
    docker_build
    ;;
  *)
    print_usage
    ;;
  esac
done
