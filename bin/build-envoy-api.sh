#!/bin/bash

# Copyright 2017 Istio Authors. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# This script fetches envoy API protos at a locked version and generates go grpc files

VENDOR_PATH="${GOPATH}/src/istio.io/istio/vendor"
OLD_IFS=$IFS

get_toml_meta() {
	local __version_regex="s/.*${1}\\s*=\\s*\"\\(.*\\)\".*/\\1/g"
	TOML_META=`cat Gopkg.toml | grep ${1} | sed -e ${__version_regex}`
}

download-versioned-src() {
	if [ -z "${1}" ]; then
	    echo "Error: No repo specified for download-versioned-src!"
		exit -1
	fi
    REPO="${1}"
	API_PATH="${2}"
	API_VERSION="${3}"
	OPT_INCLUSIONS="${4}"
	OPT_EXCLUSIONS=""
	if [ ! -z "${5}" ]; then
		OPT_EXCLUSIONS="-x ${5}"
	fi
	echo "    ${REPO}/${API_PATH} at version: ${API_VERSION}"
	VENDOR_REPO_PATH="${VENDOR_PATH}/${REPO}"
	mkdir -p ${VENDOR_REPO_PATH}
	rm -rf ${VENDOR_REPO_PATH}/*
	curl -s -Lo "${VENDOR_REPO_PATH}/${API_VERSION}.zip" https://${REPO}/${API_PATH}/archive/${API_VERSION}.zip
	unzip -qq -o -d ${VENDOR_REPO_PATH}/ ${VENDOR_REPO_PATH}/${API_VERSION}.zip ${OPT_INCLUSIONS} ${OPT_EXCLUSIONS}
	mv ${VENDOR_REPO_PATH}/${API_PATH}-${API_VERSION} ${VENDOR_REPO_PATH}/${API_PATH}
	rm ${VENDOR_REPO_PATH}/${API_VERSION}.zip
}

GO_PACKAGE_DIRS=()

extract_packages() {
    GO_PACKAGE_PREFIX="vendor/${1}/${2}"
	IFS=' ' read -ra PROTO_PATHS <<< "${3}"
	for PROTO_PATH in "${PROTO_PATHS[@]}"
	do
	  # sed regex: 's/\*\(\/\(.*\/\)?\).*/\1/g'
	  local __version_regex="s/\\*\\(\\/\\(.*\\/\\)\?\\).*/\\1/g"
	  PROTO_PATH_BASE=`echo "${PROTO_PATH}" | sed -e ${__version_regex}`
	  PROTO_PATH_SUBDIRS=`find ${GO_PACKAGE_PREFIX}${PROTO_PATH_BASE} -type d`
	  for PROTO_PATH_SUBDIR in "${PROTO_PATH_SUBDIRS[@]}"
	  do
	  	GO_PACKAGE_DIRS+=(${PROTO_PATH_SUBDIR})
	  done
	done
}

echo -e "\nFetching Envoy API proto sources"

get_toml_meta "GOGO_PROTO_APIS"
IFS=' ' read -ra APIS <<< "${TOML_META}"
for API in "${APIS[@]}"
do
    get_toml_meta "${API}_REPO"
    REPO="${TOML_META}"
    get_toml_meta "${API}_PROJECT"
    PROJECT="${TOML_META}"
    get_toml_meta "${API}_PROTOS"
    PROTOS="${TOML_META}"
    get_toml_meta "${API}_EXCLUSIONS"
    EXCLUSIONS="${TOML_META}"
    get_toml_meta "${API}_REVISION"
    REVISION="${TOML_META}"
    download-versioned-src "${REPO}" "${PROJECT}" "${REVISION}" "${PROTOS}" "${EXCLUSIONS}" 
    extract_packages "${REPO}" "${PROJECT}" "${PROTOS}" 
done

imports=(
 "vendor/github.com/envoyproxy/data-plane-api"
 "vendor/github.com/gogo/protobuf"
 "vendor/github.com/gogo/protobuf/protobuf"
 "vendor/github.com/googleapis/googleapis"
 "vendor/github.com/lyft/protoc-gen-validate"
)
for i in "${imports[@]}"
do
  IMPORTS+="--proto_path=$i "
done

mappings=(
  "gogoproto/gogo.proto=github.com/gogo/protobuf/gogoproto"
  "google/protobuf/any.proto=github.com/gogo/protobuf/types"
  "google/protobuf/api.proto=github.com/gogo/protobuf/types"
  "google/protobuf/descriptor.proto=github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
  "google/protobuf/duration.proto=github.com/gogo/protobuf/types"
  "google/protobuf/struct.proto=github.com/gogo/protobuf/types"
  "google/protobuf/timestamp.proto=github.com/gogo/protobuf/types"
  "google/protobuf/type.proto=github.com/gogo/protobuf/types"
  "google/protobuf/wrappers.proto=github.com/gogo/protobuf/types"
  "google/rpc/code.proto=istio.io/gogo-genproto/googleapis/google/rpc"
  "google/rpc/error_details.proto=istio.io/gogo-genproto/googleapis/google/rpc"
  "google/rpc/status.proto=istio.io/gogo-genproto/googleapis/google/rpc"
  "validate/validate.proto=github.com/lyft/protoc-gen-validate/validate"
)
MAPPINGS=""
for i in "${mappings[@]}"
do
  MAPPINGS+="M$i,"
done

GOGO_VERSION=$(sed -n '/gogo\/protobuf/,/\[\[projects/p' Gopkg.lock | grep version | sed -e 's/^[^\"]*\"//g' -e 's/\"//g')
GOGO_PROTOC="${GOPATH}/bin/protoc-min-version-${GOGO_VERSION}"
if [ ! -f ${GOGO_PROTOC} ]; then
  echo "Building gogo protoc for version ${GOGO_VERSION}"
  GOBIN=${GOPATH}/bin go install vendor/github.com/gogo/protobuf/protoc-min-version/minversion.go
  mv -u ${GOPATH}/bin/minversion ${GOGO_PROTOC}
fi
GOGOFAST_PROTOC_GEN="${GOPATH}/bin/protoc-gen-gogofast-${GOGO_VERSION}"
if [ ! -f ${GOGOFAST_PROTOC_GEN} ]; then
  echo "Building gogofast protoc gen for version ${GOGO_VERSION}"
  GOBIN=${GOPATH}/bin go install vendor/github.com/gogo/protobuf/protoc-gen-gogofast/main.go
  mv -u ${GOPATH}/bin/main ${GOGOFAST_PROTOC_GEN}
fi

protoc="${GOGO_PROTOC} -version=3.5.0"
PLUGIN="--plugin=${GOGOFAST_PROTOC_GEN} --gogofast-${GOGO_VERSION}_out=plugins=grpc,$MAPPINGS"

update_go_package() {
  local __pkg_regex="s/option\\s*go_package\\s*=\\s*\"\\(.*\\)\".*/\\1/g"
  local __go_pkg=`cat ${1} | grep "option go_package" | sed -e ${__pkg_regex}`
  local __re=".*/.*"
  if [[ ! "${__go_pkg}" =~ ${__re} ]]
  then
    local __go_pkg_root=`echo "${1}" | cut -d "/" -f1,2,3,4`
    OUTDIR=":${__go_pkg_root}"
  fi
}

runprotoc() {
    OUTDIR=":vendor"
    update_go_package "${1}"
	# echo -e "Running: ${protoc} ${IMPORTS} ${PLUGIN} $@\n"
	err=`${protoc} ${IMPORTS} ${PLUGIN}${OUTDIR} $@`
	if [ ! -z "$err" ]; then 
	  echo "Error in building Envoy API gogo files:"
	  echo "${err}"
	  exit -1
	fi
}

echo -e "\nGenerating Envoy API gogo files"
for PACKAGE_DIR in "${GO_PACKAGE_DIRS[@]}"
do
	echo "    proto-source: ${PACKAGE_DIR}"
	GOGO_PROTOS=`find ${PACKAGE_DIR} -maxdepth 1 -name "*.proto"`
	runprotoc ${GOGO_PROTOS}
done
echo -e "\nDone building Envoy API proto gogo files!\n"
