spy(1)

# NAME

spy - Collect information on active processes.

# SYNOPSIS

_spy_ [-vsdm] [-f datafile] [-i interval] [-p procDir]


# OPTIONS

*-v*
	Show the spy version.

*-s*
	Show the collected data. It might be better to use the *spy_show*
	program.

*-d*
	Enable debug logging.

*-m*
	Collect mapped files. Look for "map_files" in *proc*(5).

*-f* <datafile>
	Specify a data file to store the collected data to. Defaults to
	_/var/log/spy.log_.

*-i* <interval>
	The seconds to sleep between each data collection.

*-p* <procDir>
	The path to the proc filesystem. Look at *proc*(5)

# ENVIRONMENT VARIABLES

_PATH_
	Searches for executables in case a process is executed without it's full
	path. Look for "/proc/pid/cmdline" in *proc*(5)

# CONFIG FILE

_$XDG_CONFIG_HOME/spy/update_

These functions are defined but could be overridden by custom ones defined by the
user in $XDG_CONFIG_HOME/spy/update because the helper scripts are just shell
scripts that source this config file.

*list_package_files()*
	This function must list out the files belonging to the package argument
	passed to it.

*list_packages()*
	This function must list out all the packages that are installed by the user.
	You could filter it however you want using the shell.


Copy the example file in EXAMPLE_CONFIG_PATH to _$XDG_CONFIG_HOME/spy/_. The file
in EXAMPLE_CONFIG_PATH is not read by the program and serves only as an example
to the user.

# HELPER PROGRAMS

*spy_list_packages*
	This script lists out the installed packages.

*spy_list_package_files*
	This scripts lists the files belonging to a package.

*spy_show*
	Displays the data table formatted in html which can viewed by a browser.

	usage: spy_show [datafile]

# OUTPUT FORMAT

The output format for both the data file and the _-s_ flag is a tab separated
format containing 3 fields. The first field is the usage frequency, the second
field is the last used timestamp and the third field is either the filepaths in
the data file or package names in _-s_ output.

summary of formats:

data file format:

	frequency \\t timestamp \\t filepath

\-s flag format:

	frequency \\t timestamp \\t package

# SIGNALS
*SIGTERM*, *SIGINT*
	Quit

# SEE ALSO
*proc*(5)
