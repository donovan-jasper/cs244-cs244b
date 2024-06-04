
import subprocess

LOCALHOST = "127.0.0.1"
# should restore?
command_prefix = "go run ../go/src/goraft/goraft.go"
# leader_string = "We are leader, so add client command to log"
leader_string = "Election won"
def sh(command, bg=False, **kwargs):
    """Execute a local command.""" # taken from logcabin

    kwargs['shell'] = True
    if bg:
        return subprocess.Popen(command, **kwargs)
    else:
        subprocess.check_call(command, **kwargs)

def run_server(host, idx, server_list, restore=False):
    if host == LOCALHOST:
        command = f"{command_prefix} {restore} {idx} {server_list}"
    else:
        # TODO: confirm ssh works, add user, key, etc.
        # rsh(host .... TODO)
        command = f"ssh {host} {command_prefix} {restore} {idx} {server_list}"
    # command = "ls -al ../go/src/goraft/goraft.go"
    process = sh(command, bg=True, shell=True, stdout=open(f"debug/{idx}", "w"), stderr=subprocess.STDOUT)
    return process
