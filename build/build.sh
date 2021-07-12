#!/usr/bin/env bash

set -ex

KV_VERSION=${KV_VERSION:-"v0.43.0"}
mkdir -p files
cd files
RES=$(curl -H "Accept: application/vnd.github.v3+json" "https://api.github.com/repos/kubevirt/kubevirt/releases/tags/${KV_VERSION}")
JSON=$(echo ${RES}| jq -Mc '.assets[] | select(.name | startswith("virtctl-")) | {name: .name, url: .browser_download_url, mime: .content_type, size: .size, os: .name | match("^virtctl-v0.43.0-(.*)-amd64").captures[].string}')
#for j in ${JSON}; do
#  URL=$(echo ${j} | jq -r .url)
#  NAME=$(echo ${j} | jq -r .name)
#  MIME=$(echo $j | jq -r .mime)
#  curl -s -L -H "Accept:${MIME}" -o "${NAME}" "${URL}"
#done


for f in *; do
  gzip ${f}
done

cd -
mkdir -p metadata
echo "${JSON}" | jq -s 'del(.[].url)' > metadata/files.json

docker build -t "clidl:${KV_VERSION}" .
