import numpy as np
import matplotlib.pyplot as plt
import matplotlib.ticker as mtick
import argparse
import pandas as pd
import glob
import os
import time


def generate_cdf(data):
    sorted_data = np.sort(data)
    cdf = np.arange(1, len(sorted_data) + 1) / len(sorted_data)
    return sorted_data, cdf


def plot_cdf(file_name, label=None, color="blue", linestyle="-"):
    df = pd.read_csv(file_name, header=None, sep=" ", names=["elections", "time"])
    # print(df)
    times = np.sort(df["time"].values)
    elections = np.sort(df["elections"].values)
    # print(times)
    # print(elections)

    # sorted_data, cdf = generate_cdf(data)

    # plt.plot(sorted_data, cdf, label=label, color=color, linestyle=linestyle)


# labels = ["150-150ms", "150-151ms", "150-155ms", "150-175ms", "150-200ms", "150-300ms"]


def plot_cdfs(file_names, labels, colors, linestyles, plot_elections=False):
    f1 = plt.figure(1)
    plt.title("Time without leader")

    f2 = plt.figure(2)
    plt.title("Elections")
    for i, (file_name, label, color, linestyle) in enumerate(
        zip(file_names, labels, colors, linestyles)
    ):
        df = pd.read_csv(file_name, header=None, sep=" ", names=["elections", "time"])
        times = np.sort(df["time"].values)
        elections = np.sort(df["elections"].values)
        cdf = np.arange(1, len(times) + 1) / len(times)
        # print(times)
        # print(elections)

        plt.figure(1)
        plt.plot(times, cdf, label=label, color=color, linestyle=linestyle)
        plt.figure(2)
        plt.plot(elections, cdf, label=label, color=color, linestyle=linestyle)
    plt.figure(1)
    plt.xlabel("time without leader (ms)")
    plt.ylabel("cumulative percent")

    axes = plt.gca()
    axes.yaxis.set_major_formatter(mtick.PercentFormatter(1.0))
    plt.xscale("log")
    plt.legend()
    plt.savefig("time_plot.png")

    plt.figure(2)
    plt.xlabel("elections")
    plt.ylabel("cumulative percent")
    axes = plt.gca()
    axes.yaxis.set_major_formatter(mtick.PercentFormatter(1.0))
    plt.legend()
    # plt.show()
    plt.savefig("election_plot.png")


parser = argparse.ArgumentParser(description="Plot CDFs")
parser.add_argument(
    "--folder",
    help="Folder of files to plot CFS for. assumes in .txt format",
    default="/output",
)
args = parser.parse_args()
file_names = glob.glob(args.folder + "/*.txt")
labels = sorted([os.path.basename(file.split(".txt")[0]) for file in file_names])
print(labels)
colors = ["orange", "purple", "red", "green", "red", "blue"]
linestyles = [
    (0, (1, 1)),  # Orange dotted
    (0, (3, 1, 1, 1)),  # Purple denselydashdotted
    (0, (1, 1, 1, 3)),  # Red loosely two dotted
    (0, (1, 3)),  # Green loosely dotted
    "-",  # Red solid line
    "--",  # Blue dashed
]

plot_cdfs(file_names, labels, colors, linestyles)

# for i, (file_name, color, linestyle) in enumerate(zip(file_names, colors, linestyles)):
#     # plot_cdf(file_name, label=f"File {i+1}", color=color, linestyle=linestyle)
#     plot_cdf(file_name, label=labels[i], color=color, linestyle=linestyle)

# plt.xlabel("time without leader (ms)")
# plt.ylabel("cumulative percent")
# axes = plt.gca()
# axes.yaxis.set_major_formatter(mtick.PercentFormatter(1.0))
# plt.xscale("log")
# plt.legend()

# # Show plot
# plt.grid(True)
# plt.show()
