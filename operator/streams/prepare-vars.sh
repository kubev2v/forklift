#!/bin/bash

source operator/streams/${STREAM}/operator.conf 

cat operator/streams/${STREAM}/${STREAM}_manifests | envsubst '
${CSV_NAME}
${CSV_DISPLAYNAME}
${NAMESPACE}
${CSV_CERTIFIED}
${CSV_SUPPORT}
${MAINTAINER_NAME}
${MAINTAINER_EMAIL}
${PROVIDER}
${DOCS_LINK_NAME}
${DOCS_LINK_URL}
' > operator/.${STREAM}_manifests
