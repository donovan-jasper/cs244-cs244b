
import subprocess

LOCALHOST = "127.0.0.1"
command_prefix = "go run goraft/goraft.go"
leader_string = "We are leader, so add client command to log"

def sh(command, bg=False, **kwargs):
    """Execute a local command.""" # taken from logcabin

    kwargs['shell'] = True
    if bg:
        return subprocess.Popen(command, **kwargs)
    else:
        subprocess.check_call(command, **kwargs)

def run_server(host, idx, server_list):
    if host == LOCALHOST:
        command = f"{command_prefix} {idx} {server_list}"
    else:
        # TODO: confirm ssh works, add user, key, etc.
        command = f"ssh {host} {command_prefix} {idx} {server_list}"
    command = "ls -al ../go/src/goraft/goraft.go"
    process = subprocess.run(command, shell=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    return process
