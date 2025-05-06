#!/bin/sh

trap 'log' INT TERM HUP QUIT

# This tool must return the output in xml format that complies with the
# definitions set in esxcli-vmkfstools.xml
reply() {
    exit_code=$1
    output=$2
    cat << EOF
<?xml version="1.0" ?>
<output xmlns="http://www.vmware.com/Products/ESX/5.0/esxcli/">
    <structure typeName="result">
        <field name="status"><string>$exit_code</string></field>
        <field name="message"><string>$output</string></field>
    </structure>
</output>
EOF
    exit 0
}

# Function to display usage instructions
usage() {
    reply 1 "Usage: $0 -s <source-vmdk> -t <target-lun>"
}

log() {
    # log statements in 2 forms:
    # 1. log this is my log message
    # 2. echo this is my log message | log
    # if the argument list is empty try to use stdin
    file="/var/log/vmkfstools-wrapper.log"
    message="${*:-$(cat <&0)}"
    # format is [year-month-day:time] [shell ID] MESSAGE
    printf "%s [%s] INFO: %s\n" "$(date +%Y-%m-%dT%H:%M:%S%z)" "${$}" "${message}" >> $file 2>&1
}

# Initialize variables for the flags
source_vmdk=""
target_lun=""

# Parse flags
while getopts "s:t:" opt; do
    case "$opt" in
        s) source_vmdk="$OPTARG" ;;
        t) target_lun="$OPTARG" ;;
        *) usage ;;
    esac
done

# Ensure that both flags are provided
if [ -z "$source_vmdk" ] || [ -z "$target_lun" ]; then
    usage
fi

prefix=$(dirname "$source_vmdk")
suffix="-rdmdisk-$$"
rdm_disk="$source_vmdk$suffix"
resulting_rdm_file="$prefix/$(basename "$rdm_disk" .vmdk$suffix)-rdm.vmdk$suffix"

# First catch the output of the invocation
output=$(/bin/vmkfstools -i "$source_vmdk" -d rdm:"$target_lun" "$rdm_disk" 2>&1)
# Now catch the exit code
exit_code=$?
# Squeeze all to a single line, otherwise the output parsing in the xml will fail
output=$(/bin/echo $output | /bin/sed -e ':a;N;$!ba;s/\n/ /g')

log "cleaning the resulting rdm file $resulting_rdm_file"
# cleanup the resulting rdm file, it is not needed
rm -f "$resulting_rdm_file" 2>&1 | log

log "check the file $resulting_rdm_file doesn't exists"
find "$resulting_rdm_file" 2>&1 | log

reply $exit_code "$output"

