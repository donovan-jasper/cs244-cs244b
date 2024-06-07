import subprocess

LOCALHOST = "127.0.0.1"
# should restore?
# command_prefix = "go run ../go/src/goraft/goraft.go"

# leader_string = "We are leader, so add client command to log"
leader_string = "Election won"
leader_string = "leader is"


def sh(command: str, bg=False, shell=False, ignore=False, **kwargs):
    """Execute a local command."""  # taken from logcabin

    # kwargs["shell"] = True
    kwargs["shell"] = shell
    if bg:
        return subprocess.Popen(command, **kwargs)
    elif ignore:
        try:
            subprocess.check_call(command, **kwargs)
        except subprocess.CalledProcessError:
            pass
    else:
        subprocess.check_call(command, **kwargs)


def setup_remote(servers, seperate_backup=None, private=None):
    kill_cmd = "\"kill \$(ps aux | grep '[g]oraft' | awk '{print \$2}')\""
    sh(
        "env GOOS=linux GOARCH=arm64 go build -o goraft_linux ../go/src/goraft/goraft.go",
        shell=True,
    )
    for server in servers:
        host, port = server.split(":")
        if host == LOCALHOST:
            continue
        sh(f"ssh ubuntu@{host} {kill_cmd}", shell=True, ignore=True)
        kill_server(host, port, private=private)
        continue  # FOR NOW
        ssh_prefix = ""
        if private is not None:
            ssh_prefix += f"-i {private}"
        computer = f"ubuntu@{host}"
        # get goraft binary
        sh(f"scp {ssh_prefix} goraft_linux {computer}:.", shell=True)
        if seperate_backup is not None:
            sh(f"scp {ssh_prefix} -r {seperate_backup} {computer}:.", shell=True)


def kill_server(host, port, private=None):
    if host == LOCALHOST:
        sh(f"lsof -t -i:{port} | xargs -r kil", shell=True)
    else:
        ssh_prefix = ""
        if private is not None:
            ssh_prefix += f"-i {private}"
        sh(
            f"ssh {ssh_prefix} ubuntu@{host} lsof -t -i:{port} | xargs -r kill",
            shell=True,
            ignore=True,
        )


def run_server(
    host,
    idx,
    server_list,
    restore=False,
    interval=None,
    timeout=None,
    backups=None,
    seperate_backup=None,
    private=None,
) -> subprocess.Popen:
    # assumes goraft has already been built
    if host == LOCALHOST:
        command_prefix = "./goraft"
    else:
        command_prefix = "./goraft_linux"
    if interval is not None:
        command_prefix += f" -interval={interval}"
    if timeout is not None:
        # TODO: will change later when timeout is two seperate values
        command_prefix += (
            f" -electionTimeoutMax={timeout} -heartbeatTimeoutMax={timeout}"
        )
    if backups is not None:
        command_prefix += f" -backup={backups}"
    if seperate_backup is not None:
        command_prefix += f" -seperate-backup={seperate_backup}"
    if host == LOCALHOST:
        command = f"{command_prefix} --restore={restore} {idx} {server_list}"
    else:
        # TODO: confirm ssh works, add user, key, etc.
        # rsh(host .... TODO)
        ssh_prefix = ""
        if private is not None:
            ssh_prefix += f"-i {private}"
        command = f"ssh {ssh_prefix} ubuntu@{host} {command_prefix} --restore={restore} {idx} {server_list}"
    # command = "ls -al ../go/src/goraft/goraft.go"
    # print(command_prefix)
    # print(command)
    process = sh(
        command,
        bg=True,
        shell=True,
        stdout=open(f"debug/{idx}.log", "w"),
        stderr=open(f"debug/{idx}-err.log", "w"),
    )
    # process = sh(command, bg=True, shell=True, stdout=open(f"debug/{idx}.log", "w"), stderr=subprocess.STDOUT)
    return process
