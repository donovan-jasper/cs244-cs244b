import subprocess
import json
import logging
import time
import socket

LOCALHOST = "127.0.0.1"
# should restore?
# command_prefix = "go run ../go/src/goraft/goraft.go"

# leader_string = "We are leader, so add client command to log"
leader_string = "Election won"
leader_string = "leader is"


# https://gist.github.com/butla/2d9a4c0f35ea47b7452156c96a4e7b12
def wait_for_port(port: int, host: str = "localhost", timeout: float = 5.0):
    """Wait until a port starts accepting TCP connections.
    Args:
        port: Port number.
        host: Host address on which the port should exist.
        timeout: In seconds. How long to wait before raising errors.
    Raises:
        TimeoutError: The port isn't accepting connection after time specified in `timeout`.
    """
    start_time = time.perf_counter()
    while True:
        try:
            with socket.create_connection((host, port), timeout=timeout):
                break
        except OSError as ex:
            time.sleep(0.01)
            if time.perf_counter() - start_time >= timeout:
                raise TimeoutError(
                    "Waited too long for the port {} on host {} to start accepting "
                    "connections.".format(port, host)
                ) from ex


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


def setup_remote(servers, seperate_backup=None, private=None, dns_server=None):
    kill_cmd = "\"kill \\$(ps aux | grep '[g]oraft' | awk '{print \\$2}')\""
    sh(
        "env GOOS=linux GOARCH=arm64 go build -o goraft_linux ../go/src/goraft/goraft.go",
        shell=True,
    )
    for server in servers:
        host, port = server.split(":")
        if host == LOCALHOST:
            continue
        sh(f"ssh -T ubuntu@{host} {kill_cmd}", shell=True, ignore=True)
        kill_server(host, port, private=private)
        # continue  # FOR NOW TOTO UPDATE
        ssh_prefix = ""
        if private is not None:
            ssh_prefix += f"-i {private}"
        computer = f"ubuntu@{host}"
        # get goraft binary
        sh(f"scp {ssh_prefix} goraft_linux {computer}:.", shell=True)
        if seperate_backup is not None:
            sh(f"scp {ssh_prefix} -r {seperate_backup} {computer}:.", shell=True)
    if dns_server is not None:
        DNS_PORT = 15353
        if dns_server == LOCALHOST:
            sh("kill $(ps aux | grep '[r]aftdns' | awk '{{print $2}}')", shell=True)
        else:
            sh(
                "env GOOS=linux GOARCH=arm64 go build -o raftdns_linux ../go/src/dnsserver/main.go",
                shell=True,
            )
            ssh_prefix = ""
            if private is not None:
                ssh_prefix += f"-i {private}"
            sh(
                f"ssh -T {ssh_prefix} ubuntu@{dns_server} \"kill \\$(ps aux | grep '[r]aftdns' | awk '{{print \\$2}}')\"",
                shell=True,
                ignore=True,
            )
            kill_server(host, DNS_PORT, private=private)
            sh(
                f"scp {ssh_prefix} raftdns_linux ubuntu@{dns_server}:raftdns",
                shell=True,
                ignore=True,
            )


def kill_server(host, port, private=None):
    if host == LOCALHOST:
        sh(f"lsof -t -i:{port} | xargs -r kil", shell=True)
    else:
        ssh_prefix = ""
        if private is not None:
            ssh_prefix += f"-i {private}"
        sh(
            f'ssh -T {ssh_prefix} ubuntu@{host} "lsof -t -i:{port} | xargs -r kill -9"',
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
        command = f"ssh -T {ssh_prefix} ubuntu@{host} {command_prefix} --restore={restore} {idx} {server_list}"
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


def read_servers(filename: str, num_servers: int, seperate_dns=False, format="json"):
    # TODO: add support for other formats
    with open(filename, "r") as f:
        data = json.load(f)
    if "servers" not in data:
        logging.error("no server key in remote file")
    server_data = data["servers"]
    if len(server_data) < num_servers:
        logging.error(
            "not enough servers in remote file, need at least %d, only had %d",
            num_servers,
            len(server_data),
        )
        raise ValueError("not enough servers")
    server_list = " ".join(
        [d["private"] + ":" + str(d["port"]) for d in server_data[:num_servers]]
    )
    servers = [d["public"] + ":" + str(d["port"]) for d in server_data[:num_servers]]
    logging.info("server_list: %s", server_list)
    if seperate_dns:  # get extra IP for DNS server
        return servers, server_list, server_data[-1]
    return servers, server_list
