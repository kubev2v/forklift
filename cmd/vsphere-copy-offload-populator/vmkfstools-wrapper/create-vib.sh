#!/bin/bash

set -euo pipefail

SCRIPT_DIR=$(dirname "$0")
pushd "$SCRIPT_DIR"

CUSTOM_VIB_TEMP_DIR=/tmp/vib-temp-rgo
CUSTOM_VIB_NAME=vmkfstools-wrapper
CUSTOM_VIB_VERSION="${VIB_VERSION}"
CUSTOM_VIB_VENDOR="REDHAT"
CUSTOM_VIB_VENDOR_URL="https://redhat.com"
CUSTOM_VIB_SUMMARY="Custom VIB to wrap vmkfstools as esxcli plugin"
CUSTOM_VIB_DESCRIPTION="Custom VIB to wrap vmkfstools as esxcli plugin"
CUSTOM_VIB_BUILD_DATE=$(date --utc '+%Y-%m-%dT%H:%I:%S')

# clean up any prior builds
CUSTOM_VIB_FILE_NAME=${CUSTOM_VIB_NAME}.vib
rm -f ${CUSTOM_VIB_FILE_NAME}

# Setting up VIB spec confs
VIB_DESC_FILE=${CUSTOM_VIB_TEMP_DIR}/descriptor.xml
VIB_PAYLOAD_DIR=${CUSTOM_VIB_TEMP_DIR}/payloads/payload1

# Create VIB temp & spec payload directory
mkdir -p ${CUSTOM_VIB_TEMP_DIR}
mkdir -p ${VIB_PAYLOAD_DIR}

# Create ESXi folder structure for file(s) placement
CUSTOM_VIB_BIN_DIR=${VIB_PAYLOAD_DIR}/opt/redhat
ESXCLI_PLUGINS_DIR=${VIB_PAYLOAD_DIR}/usr/lib/vmware/esxcli/ext/
mkdir -p ${CUSTOM_VIB_BIN_DIR}
mkdir -p ${ESXCLI_PLUGINS_DIR}

# Copy file(s) to destination folder
# the wrapper is not needed at this point as all the command wrapping happend
# in esxcli-vmkfstools.xml, but this is left just to prove we can use it.
# Should be removed if we find it useless.
cp -v esxcli-vmkfstools.xml ${ESXCLI_PLUGINS_DIR}
cp -v vmkfstools_wrapper.sh ${CUSTOM_VIB_BIN_DIR}/vmkfstools-wrapper
chmod +x ${CUSTOM_VIB_BIN_DIR}/vmkfstools-wrapper

# Create tgz with payload
tar czvf ${CUSTOM_VIB_TEMP_DIR}/payload1 -C ${VIB_PAYLOAD_DIR} opt usr

# Calculate payload size/hash
PAYLOAD_FILES=$(tar tf ${CUSTOM_VIB_TEMP_DIR}/payload1 | grep -v -E '/$' | sed -e 's/^/    <file>/' -e 's/$/<\/file>/')
PAYLOAD_SIZE=$(stat -c %s ${CUSTOM_VIB_TEMP_DIR}/payload1)
PAYLOAD_SHA256=$(sha256sum ${CUSTOM_VIB_TEMP_DIR}/payload1 | awk '{print $1}')
PAYLOAD_SHA256_ZCAT=$(zcat ${CUSTOM_VIB_TEMP_DIR}/payload1 | sha256sum | awk '{print $1}')
PAYLOAD_SHA1_ZCAT=$(zcat ${CUSTOM_VIB_TEMP_DIR}/payload1 | sha1sum | awk '{print $1}')

# Create descriptor.xml
cat > ${VIB_DESC_FILE} << __VIB_DESC__
<vib version="5.0">
  <type>bootbank</type>
  <name>${CUSTOM_VIB_NAME}</name>
  <version>${CUSTOM_VIB_VERSION}</version>
  <vendor>${CUSTOM_VIB_VENDOR}</vendor>
  <summary>${CUSTOM_VIB_SUMMARY}</summary>
  <description>${CUSTOM_VIB_DESCRIPTION}</description>
  <release-date>${CUSTOM_VIB_BUILD_DATE}</release-date>
  <urls>
    <url key="website">${CUSTOM_VIB_VENDOR_URL}</url>
  </urls>
  <relationships>
    <depends>
    </depends>
    <conflicts/>
    <replaces/>
    <provides/>
    <compatibleWith/>
  </relationships>
  <software-tags>
  </software-tags>
  <system-requires>
    <maintenance-mode>false</maintenance-mode>
  </system-requires>
  <file-list>
${PAYLOAD_FILES}
  </file-list>
  <acceptance-level>community</acceptance-level>
  <live-install-allowed>true</live-install-allowed>
  <live-remove-allowed>true</live-remove-allowed>
  <cimom-restart>false</cimom-restart>
  <stateless-ready>true</stateless-ready>
  <overlay>false</overlay>
  <payloads>
    <payload name="payload1" type="tgz" size="${PAYLOAD_SIZE}">
        <checksum checksum-type="sha-256">${PAYLOAD_SHA256}</checksum>
        <checksum checksum-type="sha-256" verify-process="gunzip">${PAYLOAD_SHA256_ZCAT}</checksum>
        <checksum checksum-type="sha-1" verify-process="gunzip">${PAYLOAD_SHA1_ZCAT}</checksum>
    </payload>
  </payloads>
</vib>
__VIB_DESC__

# Create VIB using ar utility
touch ${CUSTOM_VIB_TEMP_DIR}/sig.pkcs7
ar r ${CUSTOM_VIB_FILE_NAME} ${VIB_DESC_FILE} ${CUSTOM_VIB_TEMP_DIR}/sig.pkcs7 ${CUSTOM_VIB_TEMP_DIR}/payload1
popd