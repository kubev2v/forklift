#!/bin/sh

FILE="$2"

reply() {
    exit_code=$1
    output=$2
    head_md5=$3
    tail_md5=$4
    cat << EOF
<?xml version="1.0" ?>
<output xmlns="http://www.vmware.com/Products/ESX/5.0/esxcli/">
    <structure typeName="result">
        <field name="head_md5"><string>$head_md5</string></field>
        <field name="tail_md5"><string>$tail_md5</string></field>
        <field name="status"><string>$exit_code</string></field>
        <field name="message"><string>$output</string></field>
    </structure>
</output>
EOF
    exit 0
}

if [ ! -e "$FILE" ]; then
    reply 1 "File not found: $FILE" "" ""
fi

# change CHUNK to 1073741824 (1*1024*1024*1024) for production
CHUNK=$((1024 * 1024))   # 1 MiB for testing
SIZE=$(stat -c %s "$FILE" 2>/dev/null || echo 0)

# if file is smaller than CHUNK, hash entire file for both head & tail
if [ "$SIZE" -le "$CHUNK" ]; then
    H=$(md5sum "$FILE" 2>/dev/null | cut -d' ' -f1)
    reply 0 "File ≤ ${CHUNK} bytes; using single‐block hash" "$H" "$H"
fi

# otherwise stream head/tail directly into md5sum
HEAD_MD5=$(head -c $CHUNK "$FILE" 2>/dev/null | md5sum | cut -d' ' -f1)
TAIL_MD5=$(tail -c $CHUNK "$FILE" 2>/dev/null | md5sum | cut -d' ' -f1)

reply 0 "Hashes calculated successfully" "$HEAD_MD5" "$TAIL_MD5"
