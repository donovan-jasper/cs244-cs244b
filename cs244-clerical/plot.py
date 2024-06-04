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

file_names = ["file1.txt", "file2.txt", "file3.txt", "file4.txt", "file5.txt"]
colors = ['orange', 'purple', 'red', 'green', 'red']
linestyles = ['--', '--', '--', '--', '-']

for i, (file_name, color, linestyle) in enumerate(zip(file_names, colors, linestyles)):
    if i < 4:
        plot_cdf(file_name, label=f"File {i+1}", color=color, linestyle=linestyle)
    else:
        plot_cdf(file_name, label=f"File {i+1}", color=color)

plt.xlabel("Duration")
plt.ylabel("CDF")
plt.legend()

# Show plot
plt.grid(True)
plt.show()

