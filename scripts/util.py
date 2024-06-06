import subprocess

LOCALHOST = "127.0.0.1"
# should restore?
command_prefix = "go run ../go/src/goraft/goraft.go"
command_prefix = "./goraft"
# leader_string = "We are leader, so add client command to log"
leader_string = "Election won"
leader_string = "leader is"


def sh(command: str, bg=False, shell=False, **kwargs):
    """Execute a local command."""  # taken from logcabin

    # kwargs["shell"] = True
    kwargs["shell"] = shell
    if bg:
        return subprocess.Popen(command, **kwargs)
    else:
        subprocess.check_call(command, **kwargs)


def run_server(host, idx, server_list, restore=False) -> subprocess.Popen:
    # assumes goraft has already been built
    if host == LOCALHOST:
        command = f"{command_prefix} --restore={restore} {idx} {server_list}"
    else:
        # TODO: confirm ssh works, add user, key, etc.
        # rsh(host .... TODO)
        command = f"ssh {host} {command_prefix} --restore={restore} {idx} {server_list}"
    # command = "ls -al ../go/src/goraft/goraft.go"
    process = sh(
        command,
        bg=True,
        shell=True,
        stdout=open(f"debug/{idx}.log", "w"),
        stderr=open(f"debug/{idx}-err.log", "w"),
    )
    # process = sh(command, bg=True, shell=True, stdout=open(f"debug/{idx}.log", "w"), stderr=subprocess.STDOUT)
    return process
