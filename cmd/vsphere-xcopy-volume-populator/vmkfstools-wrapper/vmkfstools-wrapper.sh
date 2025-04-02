#!/bin/sh

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

echo "cleaning the resulting rdm file $resulting_rdm_file" >> /var/log/vmkfstools-wrapper.log 2>&1
# cleanup the resulting rdm file, it is not needed
rm -f "$resulting_rdm_file" >> /var/log/vmkfstools-wrapper.log 2>&1

echo "check the file $resulting_rdm_file doesn't exists" >> /var/log/vmkfstools-wrapper.log 2>&1
ls -la "$resulting_rdm_file" >> /var/log/vmkfstools-wrapper.log 2>&1

reply $exit_code "$output"

