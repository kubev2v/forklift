#!/bin/sh

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

usage() {
    reply 1 "Usage: $0 -s <source-vmdk> -t <target-lun> [-v <true|false>]"
}

source_vmdk=""
target_lun=""
do_md5_check="false"
verify_md5_value=""

while getopts "s:t:v:" opt; do
    case "$opt" in
        s) source_vmdk="$OPTARG" ;;
        t) target_lun="$OPTARG" ;;
        v) verify_md5_value="$OPTARG" ;;
        *) usage ;;
    esac
done



if [ -z "$source_vmdk" ] || [ -z "$target_lun" ]; then
    usage
fi


# Construct output file path
prefix=$(dirname "$source_vmdk")
suffix="-rdmdisk-$$"
rdm_disk="${source_vmdk}${suffix}"
resulting_rdm_file="$prefix/$(basename "$rdm_disk" .vmdk${suffix})-rdm.vmdk${suffix}"

source_vmdk_flat="${source_vmdk%.vmdk}-flat.vmdk"

# Run vmkfstools
output=$(/bin/vmkfstools -i "$source_vmdk" -d rdm:"$target_lun" "$rdm_disk" 2>&1)
exit_code=$?
output=$(echo "$output" | sed -e ':a;N;$!ba;s/\n/ /g')


md5_compare_result=""
hash_mismatch=false

# MD5 comparison block
if [ "$verify_md5_value" = "true" ]; then
    CHUNK=$((1024 * 1024 * 1024))  # 1 GiB

    # Validate files
    for file in "$source_vmdk" "$target_lun"; do
        if [ ! -e "$file" ]; then
            md5_compare_result="$md5_compare_result | MD5 check: $file not found"
            hash_mismatch=true
        fi
    done

    if [ "$hash_mismatch" = false ]; then
        HEAD_MD5_SRC=$(head -c $CHUNK "$source_vmdk_flat" | md5sum | cut -d' ' -f1)
        TAIL_MD5_SRC=$(tail -c $CHUNK "$source_vmdk_flat" | md5sum | cut -d' ' -f1)
        HEAD_MD5_TGT=$(head -c $CHUNK "$target_lun"   | md5sum | cut -d' ' -f1)
        TAIL_MD5_TGT=$(tail -c $CHUNK "$target_lun"   | md5sum | cut -d' ' -f1)
        if [ "$HEAD_MD5_SRC" = "$HEAD_MD5_TGT" ]; then
            md5_compare_result="$md5_compare_result | MD5 check: HEAD match"
        else
            md5_compare_result="$md5_compare_result | MD5 check: HEAD differ"
            hash_mismatch=true
        fi

        if [ "$TAIL_MD5_SRC" = "$TAIL_MD5_TGT" ]; then
            md5_compare_result="$md5_compare_result | MD5 check: TAIL match"
        else
            md5_compare_result="$md5_compare_result | MD5 check: TAIL differ"
            hash_mismatch=true
        fi
    fi
fi

# Cleanup
echo "cleaning the resulting rdm file $resulting_rdm_file" >> /var/log/vmkfstools-wrapper.log 2>&1
rm -f "$resulting_rdm_file" >> /var/log/vmkfstools-wrapper.log 2>&1
echo "check the file $resulting_rdm_file doesn't exist" >> /var/log/vmkfstools-wrapper.log 2>&1
ls -la "$resulting_rdm_file" >> /var/log/vmkfstools-wrapper.log 2>&1

if [ "$verify_md5_value" = "true" ] && [ "$hash_mismatch" = true ]; then
    exit_code=2
fi

if [ "$verify_md5_value" = "true" ]; then
    output="$output $md5_compare_result"
fi

reply $exit_code "$output"