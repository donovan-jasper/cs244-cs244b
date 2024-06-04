import numpy as np
import matplotlib.pyplot as plt

def generate_cdf(data):
    sorted_data = np.sort(data)
    cdf = np.arange(1, len(sorted_data) + 1) / len(sorted_data)
    return sorted_data, cdf

def plot_cdf(file_name, label=None, color='blue', linestyle='-'):
    with open(file_name, 'r') as file:
        data = [float(line.strip()) for line in file if line.strip()]  # Skip empty lines

    sorted_data, cdf = generate_cdf(data)

    plt.plot(sorted_data, cdf, label=label, color=color, linestyle=linestyle)

file_names = ["file1.txt", "file2.txt", "file3.txt", "file4.txt", "file5.txt", "file6.txt"]
colors = ['orange', 'purple', 'red', 'green', 'red', 'blue']
linestyles = [
    (0, (1, 1)),          # Orange dotted
    (0, (3, 1, 1, 1)),     # Purple denselydashdotted
    (0, (1, 1, 1, 3)),         # Red loosely two dotted
    (0, (1, 3)),         # Green loosely dotted
    '-',                  # Red solid line
    '--'                  # Blue dashed
]

for i, (file_name, color, linestyle) in enumerate(zip(file_names, colors, linestyles)):
    plot_cdf(file_name, label=f"File {i+1}", color=color, linestyle=linestyle)

plt.xlabel("Duration")
plt.ylabel("CDF")
plt.legend()

# Show plot
plt.grid(True)
plt.show()

