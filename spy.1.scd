spy(1)

# NAME

spy - Collect information on active processes.

# SYNOPSIS

_spy_ [-vsdm] [-f datafile] [-i interval] [-p procDir]


# OPTIONS

*-v*
	Show the spy version.

*-s*
	show the collected data

*-d*
	Enable debug logging.

*-m*
	collect mapped files. Look for "map_files" in *proc*(5)

*-f* <datafile>
	Specify a data file to store the collected data to.

*-i* <interval>
	the seconds to sleep between each data collection

*-p* <procDir>
	the path to the proc filesystem. Look at *proc*(5)

# ENVIRONMENT VARIABLES

_PATH_
	Searches for executables in case a process is executed without it's full
	path. Look for "/proc/pid/cmdline" in *proc*(5)

# CONFIG FILE

$XDG_CONFIG_HOME/spy/update

These functions are defined but could be overridden by custom ones defined by the
user in $XDG_CONFIG_HOME/spy/update because the helper scripts are just shell
scripts and read this config file.

*list_package_files()*
	This function must list out the files belonging to the package argument
	passed to it.

*list_packages()*
	This function must list out all the packages that are installed by the user.
	You could filter it however you want using the shell.


Copy the example file in EXAMPLE_CONFIG_PATH to $XDG_CONFIG_HOME/spy/. The file
in EXAMPLE_CONFIG_PATH are not read by the program and serves only as an example
to the user.

# HELPER SCRIPTS

*spy_list_packages*
	This script lists out the installed packages.

*spy_list_package_files*
	This scripts lists the files belonging to a package.

# OUTPUT FORMAT

The output format for both the data file and the _-s_ flag is a tab separated
format containing 3 fields. The first field is the usage frequency, the second
field is the last used timestamp and the third field is either the filepaths in
the data file or package names in _-s_ output.

summary:

data file = frequency \\t timestamp \\t filepath

\-s flag   = frequency \\t timestamp \\t package

# SIGNALS
*SIGTERM*, *SIGINT*
	Quit

# SEE ALSO
*proc*(5)
