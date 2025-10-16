#!/bin/python

import argparse
import json
import logging
import os
import subprocess
import uuid
import re

VERSION = "__VERSION__"
TMP_PREFIX = "/tmp/vmkfstools-wrapper-{}"

XML = """<?xml version="1.0" ?>
<output xmlns="http://www.vmware.com/Products/ESX/5.0/esxcli/">
    <structure typeName="result">
        <field name="status"><string>{}</string></field>
        <field name="message"><string>{}</string></field>
    </structure>
</output>
"""


def clone(args):
    source = args.source_vmdk
    target = args.target_lun
    task_id = uuid.uuid4()
    tmp_dir = TMP_PREFIX.format(task_id)
    os.makedirs(tmp_dir, mode=0o750, exist_ok=True)
    rdmfile = f"{source}-rdmdisk-{os.getpid()}"

    stdout_file = open(os.path.join(tmp_dir, "out"), "w")
    stderr_file = open(os.path.join(tmp_dir, "err"), "w")
    vmkfstools_cmdline = f"trap 'echo -n $? > {tmp_dir}/exitcode' EXIT;" \
        f"/bin/vmkfstools " \
        f"-i {source} " \
        f"-d rdm:{target} {rdmfile}"
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

        result = {"taskId": str(task_id),  "pid": int(task.pid)}
        print(XML.format("0", json.dumps(result)))

    except Exception as e:
        print(XML.format("1", f"Error running subprocess: {e}"))
        raise

    finally:
        stdout_file.close()
        stderr_file.close()

def version():
        result = {"version": VERSION}
        print(XML.format("0", json.dumps(result)))

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

    result = {"taskId": args.task_id[0], "pid": int(pid),
              "exitCode": exitcode, "lastLine": line.rstrip(), "stdErr": ste}
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
    parser.add_argument("--version", action="store_true",
                        help="get the version")
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
