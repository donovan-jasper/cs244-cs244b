#!/usr/bin/env python3
import logging
import time
import util
import os
import datetime
from tqdm import tqdm
from argparse import ArgumentParser


def _safe_open(path, mode):
    """Open "path" for writing, creating any parent directories as needed."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    return open(path, mode)


def safe_open_a(path):
    """Open "path" for appending, creating any parent directories as needed."""
    return _safe_open(path, "a")


def safe_open_w(path):
    """Open "path" for writing, creating any parent directories as needed."""
    return _safe_open(path, "w")


def main(
    filename: str,
    timeout: int,
    num_servers: int,
    verbose: bool,
    interval: int,
    restore: bool = False,
    output_file: str = None,
    trials: int = 1,
):
    if output_file is None:
        output_file = f"output/150-{timeout}ms.txt"
    print(
        f"filename={filename}, timeout={timeout}, num_servers={num_servers}, verbose={verbose} restore={restore}, interval={interval}, output_file={output_file}"
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

    def run_trial(first_run=False):
        processes = []
        for idx, server_port in enumerate(servers):
            server = server_port.split(":")[0]
            logging.info("running leader test on %s", server)
            processes.append(
                util.run_server(server, idx, server_list, restore, interval, timeout)
            )
        if first_run:
            f = safe_open_w(output_file)
        else:
            f = safe_open_a(output_file)

        def look_for_leader(ignore_idx=None):
            leader_idx = None
            found_leader = False
            while True:
                for idx, process in enumerate(processes):
                    if idx == ignore_idx:  # for second time around
                        continue
                    # print(process)
                    if process.poll() is not None:
                        logging.error("server %d died", idx)
                        # kill process
                        exit(1)
                    for line in open("debug/%d-err.log" % idx):
                        if util.leader_string in line:
                            leader_time = time.time()
                            # line looks like 2024/06/05 23:43:43 term 1 leader is 4
                            content = line.split(" ")
                            # print(content[1])
                            # leader_time = time.strptime(
                            #     f"{content[0]} {content[1]}", "%Y/%m/%d %H:%M:%S.%f"
                            # )
                            leader_time = time.time()
                            leader_term = int(content[3])
                            logging.info("server %d is leader", idx)
                            found_leader = True
                            leader_idx = idx
                            break
                if found_leader:
                    break
            # print(leader_idx, leader_term, leader_time)
            return leader_idx, leader_term, leader_time

        leader_idx, leader_term, leader_time = look_for_leader()

        processes[leader_idx].kill()
        start_time = time.time()
        processes[leader_idx].wait()
        # print(start_time)
        logging.info("killed leader %d ?", leader_idx)
        leader_idx, new_term, leader_time = look_for_leader(leader_idx)
        # duration = time.mktime(leader_time) - start_time
        duration = (leader_time - start_time) * 0.001  # in milliseconds
        logging.info("duration: %f", duration)
        terms_elapsed = new_term - leader_term
        logging.info("terms elapsed: %d", terms_elapsed)
        f.write(f"{terms_elapsed} {duration}\n")
        f.close()
        # clean up other processes
        for process in processes:
            process.kill()
        for process in processes:
            process.wait()

    run_trial(first_run=True)
    for _ in range(trials - 1):
        run_trial()


def parse_args():
    argparse = ArgumentParser()
    argparse.add_argument(
        "filename", help="The name of the file to read for cluster servers"
    )
    argparse.add_argument(
        "--num_servers", help="Number of servers to use", type=int, default=5
    )
    argparse.add_argument(
        "--timeout",
        help="timeout max (ms)",
        type=int,
        default=300,
    )
    argparse.add_argument(
        "--restore", help="Restore from previous state", action="store_true"
    )
    argparse.add_argument(
        "--interval",
        help="Interval between heartbeats (ms)",
        type=int,
        default=75,
    )
    argparse.add_argument(
        "--verbose", help="Enable verbose output", action="store_true"
    )
    argparse.add_argument("--output_file", help="Output file to write to", default=None)
    argparse.add_argument(
        "--trials", help="Number of trials to run", type=int, default=1
    )
    return argparse.parse_args()


if __name__ == "__main__":
    args = parse_args()
    main(**vars(args))
