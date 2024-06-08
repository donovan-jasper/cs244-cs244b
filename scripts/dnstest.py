#!/usr/bin/env python3
import json
import logging
import time
import util
import os
import datetime
import sys
from tqdm import tqdm
from argparse import ArgumentParser

LOCALHOST = "127.0.0.1"


# https://stackoverflow.com/questions/3160699/python-progress-bar
# no external dependencies
def progressbar(it, prefix="", size=60, out=sys.stdout):  # Python3.6+
    count = len(it)
    start = time.time()  # time estimate start

    def show(j):
        x = int(size * j / count)
        # time estimate calculation and string
        remaining = ((time.time() - start) / j) * (count - j)
        mins, sec = divmod(remaining, 60)  # limited to minutes
        time_str = f"{int(mins):02}:{sec:03.1f}"
        print(
            f"{prefix}[{u'█'*x}{('.'*(size-x))}] {j}/{count} Est wait {time_str}",
            end="\r",
            file=out,
            flush=True,
        )

    show(0.1)  # avoid div/0
    for i, item in enumerate(it):
        yield item
        show(i + 1)
    print("\n", flush=True, file=out)


def _safe_open(path, mode):
    """Open "path" for writing, creating any parent directories as needed."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    return open(path, mode, buffering=1)


def safe_open_a(path):
    """Open "path" for appending, creating any parent directories as needed."""
    return _safe_open(path, "a")


def safe_open_w(path):
    """Open "path" for writing, creating any parent directories as needed."""
    return _safe_open(path, "w")


def main(
    filename: str,
    domains: str,
    timeout: int,
    num_servers: int,
    verbose: bool,
    interval: int,
    restore: bool = False,
    output_folder: str = None,
    trials: int = 1,
    backup: str = "./backups",
    seperate_backup: str = None,
    private: str = None,
    remote: str = None,
    num_domains: int = None,
):
    def setup(dns_server, servers, server_list):
        util.setup_remote(
            servers,
            seperate_backup=None,
            private=private,
            dns_server=dns_server["public"],
        )
        processes = []
        for idx, server_port in enumerate(servers):
            server, port = server_port.split(":")
            logging.info("creating raft cluster on %s", server)
            processes.append(
                util.run_server(
                    server,
                    idx,
                    server_list,
                    restore,
                    interval,
                    timeout,
                    private=private,
                )
            )

        # setup dns server
        # print(server_list)
        if dns_server["public"] == LOCALHOST:
            dns_process = util.sh(
                f"./raftdns 127.0.0.1 {server_list}",
                bg=True,
                shell=True,
                stdout=open("debug/dnsserver.log", "w"),
                stderr=open("debug/dnsserver-err.log", "w"),
            )
        else:
            dns_public = dns_server["public"]
            dns_local = dns_server["private"]
            dns_process = util.sh(
                f"ssh -T ubuntu@{dns_public} ./raftdns {dns_local} {server_list}",
                bg=True,
                shell=True,
                stdout=open("debug/dnsserver.log", "w"),
                stderr=open("debug/dnsserver-err.log", "w"),
            )
        return dns_process, processes

    def run_trial(dns_server, filename, output_folder, num_domains=None):
        dig_log = open("debug/dig.log", "w")
        time.sleep(20)  # wait for servers to start
        # util.wait_for_port(15353, dns_server, timeout=10)
        domains_list = open(filename, "r").read().splitlines()
        try1 = safe_open_a(output_folder + "/try1.txt")
        try2 = safe_open_a(output_folder + "/try2.txt")
        try3 = safe_open_a(output_folder + "/try3.txt")
        files = [try1, try2, try3]
        if num_domains is not None:
            domains_list = domains_list[:num_domains]
        for domain in tqdm(domains_list):
            # print(domain)
            # time.sleep(100)
            for i in range(3):
                start = time.time()
                util.sh(
                    f"dig +tries=1 {domain} @{dns_server} -p 15353",
                    shell=True,
                    ignore=False,
                    stdout=dig_log,
                )
                end = time.time()
                files[i].write(f"{end - start}\n")
                time.sleep(0.1)

    if output_folder is None:
        output_folder = "dns_output/"

    if not restore:
        util.sh("rm -f debug/*", shell=True)
        util.sh("mkdir -p debug", shell=True)
    util.sh("go build ../go/src/goraft/goraft.go", shell=True)
    util.sh("go build -o raftdns ../go/src/dnsserver/main.go", shell=True)

    if verbose:
        logging.basicConfig(level=logging.DEBUG)
    # read in servers

    servers, server_list, dns_server_data = util.read_servers(
        filename, num_servers, seperate_dns=True
    )
    dns_server = dns_server_data["public"]
    print("servers:", servers)
    print("dns_server:", dns_server)
    print(server_list)
    dns_process, processes = setup(dns_server_data, servers, server_list)
    domains_file = domains
    run_trial(
        dns_server,
        filename=domains_file,
        output_folder=output_folder,
        num_domains=num_domains,
    )
    # cleanup
    dns_process.terminate()
    dns_process.wait()
    for p in processes:
        p.terminate()
        p.wait()


def parse_args():
    argparse = ArgumentParser()
    argparse.add_argument(
        "filename",
        help=(
            "The name of the file to read for cluster servers. "
            "Should be in json format, describing public and"
            "private ip addresses as well as ports"
        ),
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
    argparse.add_argument(
        "--output_folder", help="Output folder to write to", default=None
    )
    argparse.add_argument("--backup", help="directory for logs", default="./backups")
    argparse.add_argument(
        "--seperate-backup",
        help="sepearte directory for loading backups",
        default=None,
    )
    argparse.add_argument("--num_domains", help="Number of domains to query", type=int)
    argparse.add_argument(
        "--domains", help="file with domains to query", default="alexa-top-1000.txt"
    )
    argparse.add_argument("-i", "--private", help="Private key file", default=None)
    argparse
    return argparse.parse_args()


if __name__ == "__main__":
    ## FOR NOW, RUN THE DNS SERVER LOCALLY
    args = parse_args()
    print(args)
    main(**vars(args))
