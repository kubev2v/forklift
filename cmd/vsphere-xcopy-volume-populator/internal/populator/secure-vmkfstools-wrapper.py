#!/usr/bin/python

import json
import logging
import os
import subprocess
import uuid
import re
import sys

TMP_PREFIX = "/tmp/vmkfstools-wrapper-{}"

# Version information for debugging
SCRIPT_VERSION = "0.0.1"

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler('/var/log/secure-vmkfstools-wrapper.log'),
        logging.StreamHandler()
    ]
)

# XML template for ESXi CLI compatible output
XML_TEMPLATE = """
<?xml version="1.0"?>
<o>
    <structure typeName="result">
        <field name="status"><string>{status}</string></field>
        <field name="message"><string>{message}</string></field>
    </structure>
</o>
"""

def validate_path(path):
    """Validate that path is safe and within expected directories"""
    # Log which version is running for debugging
    logging.info(f"secure-vmkfstools-wrapper version {SCRIPT_VERSION} validating path: {path}")

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

def clone_operation(source_vmdk, target_lun):
    """Execute vmkfstools clone operation"""
    try:
        logging.info(f"Clone operation starting: source={source_vmdk}, target={target_lun}")
        
        # Validate inputs
        logging.info("Validating source VMDK path...")
        source_vmdk = validate_path(source_vmdk)
        logging.info("Source VMDK path validation passed")
        
        logging.info("Validating target LUN path...")
        target_lun = validate_path(target_lun)
        logging.info("Target LUN path validation passed")

        # TODO: Think if there is a chance for a race here
        task_id = str(uuid.uuid4())
        tmp_dir = TMP_PREFIX.format(task_id)
        os.makedirs(tmp_dir, mode=0o750, exist_ok=True)
        rdmfile = f"{source_vmdk}-rdmdisk-{os.getpid()}"

        import shlex

        with open(os.path.join(tmp_dir, "out"), "w") as stdout_file, \
            open(os.path.join(tmp_dir, "err"), "w") as stderr_file:

            # Properly escape shell arguments
            escaped_source = shlex.quote(source_vmdk)
            escaped_target = shlex.quote(target_lun)
            escaped_rdmfile = shlex.quote(rdmfile)
            escaped_exitcode_path = shlex.quote(f"{tmp_dir}/exitcode")

            # Create the vmkfstools command with exit code capture
            vmkfstools_cmdline = [
                "/bin/sh", "-c",
                f"trap 'echo -n $? > {escaped_exitcode_path}' EXIT; "
                f"/bin/vmkfstools -i {escaped_source} -d rdm:{escaped_target} {escaped_rdmfile}"
            ]

            logging.info(f"Starting clone operation: source={source_vmdk}, target={target_lun}, task_id={task_id}")

            # Start the process
            task = subprocess.Popen(
                vmkfstools_cmdline,
                stdout=stdout_file,
                stderr=stderr_file,
                text=True,
                preexec_fn=os.setsid,
                close_fds=True,
            )

        # Save task metadata
        with open(os.path.join(tmp_dir, "pid"), "w") as pid_file:
            pid_file.write(str(task.pid))

        with open(os.path.join(tmp_dir, "rdmfile"), "w") as rdmfile_file:
            rdmfile_file.write(f"{rdmfile}\n")

        with open(os.path.join(tmp_dir, "targetLun"), "w") as target_lun_file:
            target_lun_file.write(f"{os.path.basename(target_lun)}")

        result = {"taskId": task_id, "pid": task.pid, "operation": "clone"}
        
        stdout_file.close()
        stderr_file.close()
        
        logging.info(f"Clone operation started successfully: {result}")
        print(XML_TEMPLATE.format(status="success", message=json.dumps(result)))
        
    except Exception as e:
        logging.error(f"Clone operation failed: {e}")
        print(XML_TEMPLATE.format(status="error", message=f"Error starting clone: {e}"))

def status_operation(task_id):
    """Get status of running task"""
    try:
        tmp_dir = TMP_PREFIX.format(task_id)
        
        if not os.path.exists(tmp_dir):
            raise ValueError(f"Task {task_id} not found")
        
        # Read PID
        with open(os.path.join(tmp_dir, "pid"), "r") as f:
            pid = int(f.read().strip())
        
        # Read last line of output
        last_line = ""
        try:
            with open(os.path.join(tmp_dir, "out"), "r") as f:
                for line in f:
                    last_line = line.strip()
        except FileNotFoundError:
            pass
        
        # Read stderr
        stderr_content = ""
        try:
            with open(os.path.join(tmp_dir, "err"), "r") as f:
                stderr_content = f.read()
        except FileNotFoundError:
            pass
        
        # Read exit code if available
        exit_code = ""
        try:
            with open(os.path.join(tmp_dir, "exitcode"), "r") as f:
                exit_code = f.read().strip()
        except FileNotFoundError:
            pass

        try:
            with open(os.path.join(tmp_dir, "targetLun"), "r") as target_lun_file:
                target_lun = target_lun_file.read()
            xcopy_used, xclone_writes = was_xcopy_used(target_lun)
        except FileNotFoundError:
            pass
        
        result = {
            "taskId": task_id,
            "pid": pid,
            "exitCode": exit_code,
            "lastLine": last_line,
            "stdErr": stderr_content,
            "operation": "status",
            "xcopyUsed": xcopy_used,
            "xcloneWrites": xclone_writes
        }
        
        logging.info(f"Status check for task {task_id}: {result}")
        print(XML_TEMPLATE.format(status="success", message=json.dumps(result)))
        
    except Exception as e:
        logging.error(f"Status operation failed: {e}")
        print(XML_TEMPLATE.format(status="error", message=f"Error getting status: {e}"))

def cleanup_operation(task_id):
    """Clean up task artifacts"""
    try:
        tmp_dir = TMP_PREFIX.format(task_id)
        
        if not os.path.exists(tmp_dir):
            logging.warning(f"Task directory {tmp_dir} not found for cleanup")
            print(XML_TEMPLATE.format(status="error", message="Task directory not found"))
            return
        
        # Read and remove RDM files
        try:
            with open(os.path.join(tmp_dir, "rdmfile"), "r") as rdmfile_file:
                rdmfile = rdmfile_file.read().strip()
                if rdmfile:
                    rdmdisk_file = extract_rdmdisk_file(rdmfile)
                    if rdmdisk_file:
                        try:
                            os.remove(rdmdisk_file)
                            logging.info(f"Removed RDM disk file: {rdmdisk_file}")
                        except OSError as e:
                            logging.warning(f"Failed to remove RDM disk file {rdmdisk_file}: {e}")
                    
                    try:
                        os.remove(rdmfile)
                        logging.info(f"Removed RDM file: {rdmfile}")
                    except OSError as e:
                        logging.warning(f"Failed to remove RDM file {rdmfile}: {e}")
        except FileNotFoundError:
            pass
        
        # Remove the entire task directory
        import shutil
        shutil.rmtree(tmp_dir, ignore_errors=True)
        
        logging.info(f"Cleaned up task {task_id}")
        print(XML_TEMPLATE.format(status="success", message="Cleanup completed"))
        
    except Exception as e:
        logging.error(f"Cleanup operation failed: {e}")
        print(XML_TEMPLATE.format(status="error", message=f"Error during cleanup: {e}"))

def extract_rdmdisk_file(rdm_file):
    """Extract RDM disk file path from RDM descriptor file"""
    regex_pattern = re.compile(r'.* VMFSRDM "([^"]*)"\s*$')
    try:
        with open(rdm_file, 'r') as f:
            for line in f:
                match = regex_pattern.search(line)
                if match:
                    return os.path.join(os.path.dirname(rdm_file), match.group(1))
    except Exception as e:
        logging.error(f"Error extracting RDM disk file: {e}")
    return ""

def parse_ssh_original_command():
    """Parse the SSH_ORIGINAL_COMMAND environment variable to extract operation and arguments"""
    ssh_command = os.environ.get('SSH_ORIGINAL_COMMAND', '').strip()
    
    if not ssh_command:
        raise ValueError("No SSH_ORIGINAL_COMMAND provided")
    
    logging.info(f"Received SSH command: {ssh_command}")
    
    # Parse the command - expected formats:
    # "clone <source_vmdk> <target_lun>"
    # "status <task_id>"
    # "cleanup <task_id>"
    
    parts = ssh_command.split()
    if len(parts) < 2:
        raise ValueError(f"Invalid command format: {ssh_command}")
    
    operation = parts[0].lower()
    
    if operation == "clone":
        if len(parts) != 3:
            raise ValueError(f"Clone operation requires exactly 2 arguments: {ssh_command}")
        return operation, parts[1], parts[2]
    
    elif operation in ["status", "cleanup"]:
        if len(parts) != 2:
            raise ValueError(f"{operation.capitalize()} operation requires exactly 1 argument: {ssh_command}")
        return operation, parts[1], None
    
    else:
        raise ValueError(f"Unauthorized operation: {operation}")

def was_xcopy_used(target_lun):
    stats_path = f"/storage/scsifw/devices/{target_lun.strip()}/stats"

    try:
        target_lun_stats = subprocess.run(
            ["vsish", "-r", "-e", "cat", stats_path],
            capture_output=True,
            text=True,
            check=True
        )
    except subprocess.CalledProcessError as e:
        logging.error(f"Error: Unable to read stats for device {target_lun}")
        raise e

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
    """Main entry point that handles SSH_ORIGINAL_COMMAND or command line arguments"""
    # Check for version argument first
    if len(sys.argv) > 1 and sys.argv[1] == "--version":
        print(f"secure-vmkfstools-wrapper version {SCRIPT_VERSION}")
        sys.exit(0)

    # Log script startup
    logging.info(f"secure-vmkfstools-wrapper version {SCRIPT_VERSION} starting")

    try:
        # Parse the SSH command
        operation, arg1, arg2 = parse_ssh_original_command()
        
        logging.info(f"Executing operation: {operation} with args: {arg1}, {arg2}")

        if operation == "clone":
            clone_operation(arg1, arg2)
        elif operation == "status":
            status_operation(arg1)
        elif operation == "cleanup":
            cleanup_operation(arg1)
        else:
            raise ValueError(f"Invalid operation: {operation}")
            
    except Exception as e:
        logging.error(f"Command execution failed: {e}")
        print(XML_TEMPLATE.format(status="error", message=f"Unauthorized or invalid command: {e}"))
        sys.exit(1)

if __name__ == "__main__":
    main() 