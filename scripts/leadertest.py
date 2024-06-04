#!/usr/bin/env python3
import logging
import util
from argparse import ArgumentParser

def main(filename, timeout, num_servers, verbose):
    print(f"filename={filename}, timeout={timeout}, num_servers={num_servers}, verbose={verbose}")
    util.sh('rm -f debug/*')
    util.sh('mkdir -p debug')
    if verbose:
        logging.basicConfig(level=logging.DEBUG)
    # read file, assume it is in format of "server:port"
    with open(filename, "r") as f:
        servers = f.read().splitlines()
    logging.info("servers: %s", servers)
    if len(servers) < num_servers:
        logging.error("not enough servers, need at least %d, only had %d", num_servers, len(servers))
        return

    server_list = " ".join(servers[:num_servers])
    logging.info("server_list: %s", server_list)
    # run the leader test
    for idx, server_port in enumerate(servers):
        server = server_port.split(":")[0]
        logging.info("running leader test on %s", server)
        util.run_server(server, idx, server_list)

if __name__ == "__main__":
    argparse = ArgumentParser()
    argparse.add_argument("filename", help="The name of the file to read")
    argparse.add_argument("--num_servers", help="Number servers", type=int, default=5)
    argparse.add_argument("--timeout", help="The timeout value", type=int, default=10)
    argparse.add_argument("--verbose", help="Enable verbose output", action="store_true")
    args = argparse.parse_args()
    main(**vars(args))