#!/bin/python

import argparse
import errno
import json
import logging
import os
import subprocess
import uuid
import re
import sys
import shlex
import shutil

TMP_PREFIX = "/tmp/vmkfstools-wrapper-{}"

# Version information for debugging
SCRIPT_VERSION = "0.0.5"

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
        '/vmfs/devices/disks/' # ESXi disk devices (target devices for cloning)
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

        result = {"taskId": str(task_id), "pid": int(task.pid)}
        print(XML.format("0", json.dumps(result)))


    except Exception as e:
        err_code = getattr(e, 'errno', None)
        if err_code == errno.ENOSPC:
            print(XML.format("28", f"Error running subprocess: {e} free space is low: check /var/log/vmkernel.log for more details"))
        else:
            print(XML.format("1", f"Error running subprocess: {e}"))
        raise

    finally:
        stdout_file.close()
        stderr_file.close()


def get_last_line(f):
    with open(f.name, "rb") as file_handle:
        # Seek to end of file
        file_handle.seek(0, 2)
        file_size = file_handle.tell()

        if file_size > 0:
            chunk_size = min(4096, file_size)
            file_handle.seek(-chunk_size, 2)
            content = file_handle.read()

            last_cr = content.rfind(b'\r')
            if last_cr != -1:
                line = content[last_cr + 1:].decode('utf-8', errors='replace')
            else:
                last_nl = content.rfind(b'\n')
                if last_nl != -1:
                    line = content[last_nl + 1:].decode('utf-8', errors='replace')
                else:
                    line = content.decode('utf-8', errors='replace')
            return line

    return ""


def taskGet(args):
    tmp_dir = TMP_PREFIX.format(args.task_id[0])

    if not os.path.isdir(tmp_dir):
        result = {"taskId": args.task_id[0], "error": f"Task directory {tmp_dir} not found"}
        print(XML.format("1", json.dumps(result)))
        return

    with open(os.path.join(tmp_dir, "pid"), "r") as f:
        pid = f.read()
    with open(os.path.join(tmp_dir, "out"), "r") as f:
        line = get_last_line(f)
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
                  "exitCode": "1", "lastLine": line.rstrip(), "stdErr": str(e)}
        print(XML.format("1", json.dumps(result)))
        return

    # Default to None if we can't determine xcopy usage
    xcopy_used = None
    try:
        with open(os.path.join(tmp_dir, "targetLun"), "r") as target_lun_file:
            target_lun = target_lun_file.read()
        xcopy_used, xclone_writes = was_xcopy_used(target_lun)
        # Log xclone_writes for debugging, but don't expose in result
        logging.info(f"XCOPY used: {xcopy_used}, clone write ops: {xclone_writes}")
    except Exception as e:
        logging.warning(f"Failed to determine xcopy usage: {e}, defaulting to False")

    result = {"taskId": args.task_id[0], "pid": int(pid),
            "exitCode": exitcode, "lastLine": line.rstrip(),
            "xcopyUsed": xcopy_used, "stdErr": ste}
    print(XML.format("0", json.dumps(result)))

def taskClean(args):
    tmp_dir = TMP_PREFIX.format(args.task_id[0])

    if not os.path.isdir(tmp_dir):
        result = {"taskId": args.task_id[0], "error": f"Task directory {tmp_dir} not found"}
        print(XML.format("1", json.dumps(result)))
        return

    rdmfile_path = os.path.join(tmp_dir, "rdmfile")
    # Attempt to remove rdmdisk_file
    if os.path.exists(rdmfile_path):
        try:
            with open(rdmfile_path, "r") as rdmfile_file:
                rdmfile = rdmfile_file.read().rstrip()
                if rdmfile != "" and os.path.exists(rdmfile):
                    rdmdisk_file = extract_rdmdisk_file(rdmfile)
                    if rdmdisk_file and os.path.exists(rdmdisk_file):
                        logging.info(f"removing rdmdisk file {rdmdisk_file}")
                        try:
                            os.remove(rdmdisk_file)
                            logging.info(f"removed rdmdisk file {rdmdisk_file}")
                        except Exception as e:
                            logging.warning(f"failed to remove {rdmdisk_file}: {e}")
                    try:
                        logging.info(f"removing rdmfile {rdmfile}")
                        os.remove(rdmfile)
                        logging.info(f"removed rdmfile {rdmfile}")
                    except Exception as e:
                        logging.warning(f"failed to remove {rdmfile}: {e}")
        except Exception as e:
            logging.warning(f"failed to process rdmfile: {e}")
    try:
        validate_path(tmp_dir)
        shutil.rmtree(tmp_dir)
        logging.info(f"removed task directory {tmp_dir}")
    except Exception as e:
        logging.error(f"failed to remove task directory {tmp_dir}: {e}")
        print(XML.format("1", f"failed to remove task directory {e}"))
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

def version():
    print(XML.format("0", json.dumps({"version":SCRIPT_VERSION})))
    return

def main():
    # Setup logging first, before any logging calls
    logging.basicConfig(
            level=logging.INFO,
            handlers=[
                logging.FileHandler('/var/log/vmkfstools-wrapper.log'),
                logging.StreamHandler()
            ],
            format='%(asctime)s - %(levelname)s - %(message)s')

    # For SSH restricted command execution, parse SSH_ORIGINAL_COMMAND
    ssh_command = os.environ.get('SSH_ORIGINAL_COMMAND', '').strip()
    if ssh_command:
        logging.info(f"Received SSH_ORIGINAL_COMMAND: {ssh_command}")
        # Convert SSH command to sys.argv format for argparse
        sys.argv = ['vmkfstools_wrapper.py'] + ssh_command.split()

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

    parser.add_argument("-v", "--version", action="store_true",
                        help="version")

    args = parser.parse_args()

    if args.version:
        version()
    if args.clone and args.source_vmdk is not None and args.target_lun is not None:
        clone(args)
    elif args.task_get:
        taskGet(args)
    elif args.task_clean:
        taskClean(args)


if __name__ == "__main__":
    main()
