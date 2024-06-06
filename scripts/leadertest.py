#!/usr/bin/env python3
import logging
import time
import util
from argparse import ArgumentParser


def main(
    filename: str,
    timeout: int,
    num_servers: int,
    verbose: bool,
    restore=False,
    interval: int = 10,
):
    print(
        f"filename={filename}, timeout={timeout}, num_servers={num_servers}, verbose={verbose}"
    )
    if not restore:
        util.sh("rm -f debug/*", shell=True)
        util.sh("mkdir -p debug", shell=True)
    util.sh("go build ../go/src/goraft/goraft.go", shell=True)

    if verbose:
        logging.basicConfig(level=logging.DEBUG)
    # read file, assume it is in format of "server:port"
    with open(filename, "r") as f:
        servers = f.read().splitlines()
    logging.info("servers: %s", servers)
    if len(servers) < num_servers:
        logging.error(
            "not enough servers, need at least %d, only had %d",
            num_servers,
            len(servers),
        )
        return

    server_list = " ".join(servers[:num_servers])
    logging.info("server_list: %s", server_list)
    # run the leader test
    processes = []
    for idx, server_port in enumerate(servers):
        server = server_port.split(":")[0]
        logging.info("running leader test on %s", server)
        processes.append(util.run_server(server, idx, server_list))

    found_leader = False
    leader_idx = None
    current_term = 0
    leader_term = 0
    while True:
        for idx, process in enumerate(processes):
            # print(process)
            if process.poll() is not None:
                logging.error("server %d died", idx)
                return
            for line in open("debug/%d.log" % idx):
                if util.leader_string in line:
                    logging.info("server %d is leader", idx)
                    found_leader = True
                    leader_idx = idx
                    break
        if found_leader:
            break
    # TODO: once you figure out which one is leader, kill it
    processes[leader_idx].kill()
    processes[leader_idx].wait()
    start_time = time.time()
    logging.info("killed leader %d ?", leader_idx)
    for i in range(len(processes)):
        if i == leader_idx:
            continue
        processes[i].terminate()
        processes[i].wait()
        print("killed server", i)
    print("killed all servers except leader")
    print(processes[leader_idx])


if __name__ == "__main__":
    argparse = ArgumentParser()
    argparse.add_argument(
        "filename", help="The name of the file to read for cluster servers"
    )
    argparse.add_argument(
        "--num_servers", help="Number of servers to use", type=int, default=5
    )
    argparse.add_argument(
        "--timeout",
        help="timeout hige range (ms). NOT IMPLEMENTED",
        type=int,
        default=150,
    )
    argparse.add_argument(
        "--restore", help="Restore from previous state", action="store_true"
    )
    argparse.add_argument(
        "--interval",
        help="Interval between heartbeats (ms). NOT IMPLEMENTED",
        type=int,
        default=10,
    )
    argparse.add_argument(
        "--verbose", help="Enable verbose output", action="store_true"
    )
    args = argparse.parse_args()
    main(**vars(args))
