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

download-versioned-src() {
	if [ -z "${1}" ]; then
	    echo "Error: No repo specified!"
		return -1
	fi
    REPO="${1}"
	API_PATH="${2}"
	API_VERSION="${3}"
	OPT_INCLUSIONS="${4}"
	OPT_EXCLUSIONS=""
	if [ ! -z "${5}" ]; then
		OPT_EXCLUSIONS="-x ${5}"
	fi
	echo "Downloading proto files for ${REPO}/${API_PATH} at version: ${API_VERSION}"
	VENDOR_REPO_PATH="${VENDOR_PATH}/${REPO}"
	mkdir -p ${VENDOR_REPO_PATH}
	rm -rf ${VENDOR_REPO_PATH}/*
	curl -s -Lo "${VENDOR_REPO_PATH}/${API_VERSION}.zip" https://${REPO}/${API_PATH}/archive/${API_VERSION}.zip
	unzip -qq -o -d ${VENDOR_REPO_PATH}/ ${VENDOR_REPO_PATH}/${API_VERSION}.zip ${OPT_INCLUSIONS} ${OPT_EXCLUSIONS}
	mv ${VENDOR_REPO_PATH}/${API_PATH}-${API_VERSION} ${VENDOR_REPO_PATH}/${API_PATH}
	rm ${VENDOR_REPO_PATH}/${API_VERSION}.zip
}

echo ""
echo "Fetching Envoy API proto sources"

# Fetch Envoy API
VERSION_ENVOY_API="7d740da8fc19dac2132403b7456bdca477a1b70a"
download-versioned-src github.com/envoyproxy data-plane-api ${VERSION_ENVOY_API} "*.proto" "*/metrics_service.proto"

# Fetch Googleapis
VERSION_GOOGLE_APIS="c8c975543a134177cc41b64cbbf10b88fe66aa1d"
download-versioned-src github.com/googleapis googleapis ${VERSION_GOOGLE_APIS} "*/google/rpc/*.proto */google/api/*.proto"

# Fetch Lyft protogen validate
VERSION_LYFT_APIS="8e6aaf55f4954f1ef9d3ee2e8f5a50e79cc04f8f"
download-versioned-src github.com/lyft protoc-gen-validate ${VERSION_LYFT_APIS} "*/validate/validate.proto"

echo "Generating Envoy API gogo files"
OUTDIR="vendor/github.com/envoyproxy/data-plane-api"

imports=(
 "${OUTDIR}"
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
  "google/protobuf/duration.proto=github.com/gogo/protobuf/types"
  "google/rpc/status.proto=istio.io/gogo-genproto/googleapis/google/rpc"
  "google/rpc/code.proto=istio.io/gogo-genproto/googleapis/google/rpc"
  "google/rpc/error_details.proto=istio.io/gogo-genproto/googleapis/google/rpc"
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
PLUGIN="--plugin=${GOGOFAST_PROTOC_GEN} --gogofast-${GOGO_VERSION}_out=plugins=grpc,$MAPPINGS:"
PLUGIN+="${OUTDIR}"

runprotoc() {
	err=`${protoc} ${IMPORTS} ${PLUGIN} ${1}`
	if [ ! -z "$err" ]; then 
	  echo "Error in building Envoy API gogo files "
	  echo "${err}"
	fi
}

API_PROTOS=`find ${OUTDIR}/api -maxdepth 1 -name "*.proto"`
API_AUTH_PROTOS=`find ${OUTDIR}/api/auth -maxdepth 1 -name "*.proto"`
API_FILTER_PROTOS=`find ${OUTDIR}/api/filter -maxdepth 1 -name "*.proto"`
API_FILTER_ACCESSLOG_PROTOS=`find ${OUTDIR}/api/filter/accesslog -maxdepth 1 -name "*.proto"`
API_FILTER_HTTP_PROTOS=`find ${OUTDIR}/api/filter/http -maxdepth 1 -name "*.proto"`
API_FILTER_HTTP_NETWORK_PROTOS=`find ${OUTDIR}/api/filter/network -maxdepth 1 -name "*.proto"`

runprotoc "${API_PROTOS}"
runprotoc "${API_AUTH_PROTOS}"
runprotoc "${API_FILTER_PROTOS}"
runprotoc "${API_FILTER_ACCESSLOG_PROTOS}"
runprotoc "${API_FILTER_HTTP_PROTOS}"
runprotoc "${API_FILTER_HTTP_NETWORK_PROTOS}"

echo "Done building Envoy API proto gogo files!"
echo ""
