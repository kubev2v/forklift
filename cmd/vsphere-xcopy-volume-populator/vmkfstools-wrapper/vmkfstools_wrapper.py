#!/bin/python

import argparse
import json
import logging
import os
import subprocess
import uuid
import re
import sys
import shlex

TMP_PREFIX = "/tmp/vmkfstools-wrapper-{}"

# Version information for debugging
SCRIPT_VERSION = "0.0.2"

XML = """<?xml version="1.0" ?>
<output xmlns="http://www.vmware.com/Products/ESX/5.0/esxcli/">
    <structure typeName="result">
        <field name="status"><string>{}</string></field>
        <field name="message"><string>{}</string></field>
    </structure>
</output>
"""


def validate_path(path):
    """Validate that path is safe and within expected directories"""
    # Log which version is running for debugging
    logging.info(f"vmkfstools-wrapper version {SCRIPT_VERSION} validating path: {path}")

    # Normalize the path to prevent bypasses
    path = os.path.normpath(path)

    # Only allow paths in specific safe directories for ESXi operations
    allowed_prefixes = [
        '/vmfs/volumes/',      # Datastore volumes (source VMDK files)
        '/vmfs/devices/disks/', # ESXi disk devices (target devices for cloning)
        '/tmp/'
    ]
    if not any(path.startswith(prefix) for prefix in allowed_prefixes):
        raise ValueError(f"Path not in allowed directories: {path}")

    # Prevent path traversal attacks
    if '..' in path or '//' in path:
        raise ValueError(f"Invalid path detected: {path}")

    logging.info(f"Path validation passed for: {path}")
    return path


def clone(args):
    # Validate inputs for security
    try:
        logging.info("Validating source VMDK path...")
        source = shlex.quote(validate_path(args.source_vmdk))
        logging.info("Source VMDK path validation passed")

        logging.info("Validating target LUN path...")
        target = shlex.quote(validate_path(args.target_lun))
        logging.info("Target LUN path validation passed")
    except ValueError as ve:
        # Path validation errors
        logging.error(f"Path validation failed: {ve}")
        print(XML.format("1", f"Path validation error: {ve}"))
        raise

    task_id = uuid.uuid4()
    tmp_dir = TMP_PREFIX.format(task_id)
    os.makedirs(tmp_dir, mode=0o750, exist_ok=True)
    rdmfile = f"{args.source_vmdk}-rdmdisk-{os.getpid()}"

    stdout_file = open(os.path.join(tmp_dir, "out"), "w")
    stderr_file = open(os.path.join(tmp_dir, "err"), "w")
    vmkfstools_cmdline = f"trap 'echo -n $? > {shlex.quote(f'{tmp_dir}/exitcode')}' EXIT;" \
        f"/bin/vmkfstools " \
        f"-i {source} " \
        f"-d rdm:{target} {shlex.quote(rdmfile)}"
    logging.info(f"about to run {vmkfstools_cmdline}")
    try:
        task = subprocess.Popen(
            ["/bin/sh", "-c", vmkfstools_cmdline],
            stdout=stdout_file,
            stderr=stderr_file,
            text=True,
            preexec_fn=os.setsid,
            close_fds=True,
        )

        with open(os.path.join(tmp_dir, "pid"), "w") as pid_file:
            pid_file.write(str(task.pid))

        with open(os.path.join(tmp_dir, "rdmfile"), "w") as rdmfile_file:
            rdmfile_file.write(f"{rdmfile}\n")

        with open(os.path.join(tmp_dir, "targetLun"), "w") as target_lun_file:
            target_lun_file.write(f"{os.path.basename(target)}")

        result = {"taskId": str(task_id),  "pid": int(task.pid)}
        print(XML.format("0", json.dumps(result)))

    except Exception as e:
        print(XML.format("1", f"Error running subprocess: {e}"))
        raise

    finally:
        stdout_file.close()
        stderr_file.close()


def taskGet(args):
    tmp_dir = TMP_PREFIX.format(args.task_id[0])
    with open(os.path.join(tmp_dir, "pid"), "r") as f:
        pid = f.read()
    with open(os.path.join(tmp_dir, "out"), "r") as f:
        line = ""
        for line in f:
            pass
    with open(os.path.join(tmp_dir, "err"), "r") as f:
        ste = f.read()
    # it could be that the task is still running, hense no exit code
    try:
        with open(os.path.join(tmp_dir, "exitcode"), "r") as f:
            exitcode = f.read()
    except FileNotFoundError:
        exitcode = ""
    except Exception as e:
        result = {"taskId": args.task_id[0], "pid": int(pid),
                  "exitCode": "1", "lastLine": line.rstrip(), "stdErr": e}
        print(XML.format("1", json.dumps(result)))
        return
    try:
        with open(os.path.join(tmp_dir, "targetLun"), "r") as target_lun_file:
            target_lun = target_lun_file.read()
        xcopy_used, xclone_writes = was_xcopy_used(target_lun)
        # Log xclone_writes for debugging, but don't expose in result
        logging.info(f"XCOPY used: {xcopy_used}, clone write ops: {xclone_writes}")
    except Exception as e:
        result = {"taskId": args.task_id[0], "pid": int(pid),
                  "exitCode": "1", "lastLine": line.rstrip(), "stdErr": e}
        print(XML.format("1", json.dumps(result)))

    result = {"taskId": args.task_id[0], "pid": int(pid),
            "exitCode": exitcode, "lastLine": line.rstrip(),
            "xcopyUsed": xcopy_used, "stdErr": ste}
    print(XML.format("0", json.dumps(result)))

def taskClean(args):
    tmp_dir = TMP_PREFIX.format(args.task_id[0])
    with open(os.path.join(tmp_dir, "rdmfile"), "r") as rdmfile_file:
        rdmfile = rdmfile_file.read()
        rdmfile = rdmfile.rstrip()
        if rdmfile != "":
            rdmdisk_file = extract_rdmdisk_file(rdmfile)
            logging.info(f"removing {rdmfile} and {rdmdisk_file}")
            try:
                os.remove(rdmdisk_file)
                logging.info(f"removed {rdmdisk_file}")
                os.remove(rdmfile)
                logging.info(f"removed {rdmfile}")
            except Exception as e:
                logging.info(f"failed to remove files {e}")
                print(XML.format("1", f"failed to remove files {e}"))
                return
    print(XML.format("0", ""))


def extract_rdmdisk_file(rdm_file):
    regex_pattern = re.compile(r'.* VMFSRDM "([^"]*)"\s*$')
    try:
        with open(rdm_file, 'r') as f:
            for _, line in enumerate(f, 1):
                match = regex_pattern.search(line)
                if match:
                    return os.path.join(os.path.dirname(rdm_file), match.group(1))
    except Exception as e:
        logging.error(e)
    return ""

def was_xcopy_used(target_lun):
    stats_path = f"/storage/scsifw/devices/{target_lun.strip()}/stats"

    try:
        target_lun_stats = subprocess.run(
            ["vsish", "-r", "-e", "cat", stats_path],
            capture_output=True,
            text=True,
            check=True
        )

        logging.info(f"vshish stats {target_lun_stats}")
    except subprocess.CalledProcessError as e:
        # don't panic, just warn we can't extract xcopy info
        logging.error(f"Error: Unable to read XCOPY stats for device {target_lun} with path {stats_path}")
        return False, "0"
        

    write_ops = 0
    for statistic in target_lun_stats.stdout.splitlines():
        if "clonewriteops" in statistic.lower():
            try:
                write_ops = int(statistic.split(':')[1], 16)
            except ValueError:
                logging.error(f"Error: Unable to parse statistic: {statistic.split(':')[1]}")
                write_ops = 0
            break

    return write_ops > 0, str(write_ops)


def main():
    parser = argparse.ArgumentParser(description="vmkfstools-wrapper")

    parser.add_argument("--clone", action="store_true",
                        help="source VMDK to clone")

    parser.add_argument("-s", "--source-vmdk", type=str,
                        metavar="source_vmdk", default=None,
                        help="source VMDK to clone")

    parser.add_argument("-t", "--target-lun", type=str,
                        metavar="target_lun", default=None,
                        help="destination target LUN")

    parser.add_argument("--task-get", action="store_true",
                        help="get task status")

    parser.add_argument("--task-clean", action="store_true",
                        help="clean task")

    parser.add_argument("-i", "--task-id", type=str, nargs=1,
                        metavar="task_id", default=None,
                        help="id of task to get")
    args = parser.parse_args()
    logging.basicConfig(filename='/var/log/vmkfstools-wrapper.log',
                        level=logging.INFO,
                        format='%(asctime)s - %(levelname)s - %(message)s')

    if args.clone and args.source_vmdk is not None and args.target_lun is not None:
        clone(args)
    elif args.task_get:
        taskGet(args)
    elif args.task_clean:
        taskClean(args)


if __name__ == "__main__":
    main()
